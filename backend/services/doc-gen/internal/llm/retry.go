// Package llm 的重试包装器：对 LLM 调用做指数退避重试。
// 解决 API 超时/限流导致的章节生成失败问题。
package llm

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/bidwriter/services/doc-gen/internal/core"
)

// RetryClient 包装一个 Client，添加重试逻辑。
type RetryClient struct {
	inner      Client
	maxRetries int
	baseDelay  time.Duration
	log        *slog.Logger
}

// NewRetryClient 创建带重试的客户端。
// maxRetries=3 表示最多重试 3 次（共 4 次调用）。
func NewRetryClient(inner Client, maxRetries int, log *slog.Logger) *RetryClient {
	if maxRetries <= 0 {
		maxRetries = 3
	}
	return &RetryClient{
		inner:      inner,
		maxRetries: maxRetries,
		baseDelay:  2 * time.Second,
		log:        log,
	}
}

// Chat 带重试的 Chat 调用。指数退避：2s → 4s → 8s。
func (r *RetryClient) Chat(ctx context.Context, req *core.LLMRequest) (*core.LLMResponse, error) {
	var lastErr error
	for attempt := 0; attempt <= r.maxRetries; attempt++ {
		resp, err := r.inner.Chat(ctx, req)
		if err == nil {
			return resp, nil
		}
		lastErr = err
		if attempt < r.maxRetries {
			delay := r.baseDelay * time.Duration(1<<attempt) // 2s, 4s, 8s
			if r.log != nil {
				r.log.Warn("LLM 调用失败，重试",
					"attempt", attempt+1,
					"max", r.maxRetries,
					"delay", delay,
					"err", err)
			}
			select {
			case <-time.After(delay):
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}
	}
	return nil, fmt.Errorf("LLM 调用 %d 次后仍失败: %w", r.maxRetries+1, lastErr)
}

// Embed 带重试的 Embed 调用。
func (r *RetryClient) Embed(ctx context.Context, text string) ([]float32, error) {
	var lastErr error
	for attempt := 0; attempt <= r.maxRetries; attempt++ {
		vec, err := r.inner.Embed(ctx, text)
		if err == nil {
			return vec, nil
		}
		lastErr = err
		if attempt < r.maxRetries {
			delay := r.baseDelay * time.Duration(1<<attempt)
			select {
			case <-time.After(delay):
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}
	}
	return nil, fmt.Errorf("Embed 调用 %d 次后仍失败: %w", r.maxRetries+1, lastErr)
}

// EmbedBatch 带重试的 EmbedBatch 调用。
func (r *RetryClient) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	var lastErr error
	for attempt := 0; attempt <= r.maxRetries; attempt++ {
		vecs, err := r.inner.EmbedBatch(ctx, texts)
		if err == nil {
			return vecs, nil
		}
		lastErr = err
		if attempt < r.maxRetries {
			delay := r.baseDelay * time.Duration(1<<attempt)
			select {
			case <-time.After(delay):
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}
	}
	return nil, fmt.Errorf("EmbedBatch 调用 %d 次后仍失败: %w", r.maxRetries+1, lastErr)
}
