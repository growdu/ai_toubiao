// Package llm 定义 LLM 客户端抽象。
// CLI 模式可直连 Provider 或走 router-svc；服务模式走 router-svc。
// 内核通过此接口访问 LLM，不绑定具体实现。
package llm

import (
	"context"
	"fmt"

	"github.com/bidwriter/services/doc-gen/internal/core"
)

// Client 是 LLM 调用的统一接口。
type Client interface {
	// Chat 发送对话请求。
	Chat(ctx context.Context, req *core.LLMRequest) (*core.LLMResponse, error)
	// Embed 生成文本的向量表示。
	Embed(ctx context.Context, text string) ([]float32, error)
	// EmbedBatch 批量生成向量。
	EmbedBatch(ctx context.Context, texts []string) ([][]float32, error)
}

// NoopClient 是无 LLM 时的占位实现（调试用）。
type NoopClient struct{}

func (NoopClient) Chat(ctx context.Context, req *core.LLMRequest) (*core.LLMResponse, error) {
	return nil, fmt.Errorf("noop LLM client: no provider configured")
}
func (NoopClient) Embed(ctx context.Context, text string) ([]float32, error) {
	return nil, fmt.Errorf("noop LLM client: no provider configured")
}
func (NoopClient) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	return nil, fmt.Errorf("noop LLM client: no provider configured")
}
