// Anthropic 兼容 API 的 LLM 客户端实现。
// 支持 Anthropic 官方 API 及兼容接口（如 MiniMax）。
// CLI 模式可通过 ANTHROPIC_AUTH_TOKEN 环境变量自动配置。
package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/bidwriter/services/doc-gen/internal/core"
)

// AnthropicClient 调用 Anthropic 兼容 API。
type AnthropicClient struct {
	apiKey  string
	apiBase string
	model   string
	http    *http.Client
}

// NewAnthropicClient 创建 Anthropic 客户端。
// apiBase 如 "https://api.anthropic.com" 或 "https://api.minimaxi.com/anthropic"
func NewAnthropicClient(apiKey, apiBase, model string) *AnthropicClient {
	// 确保不以 / 结尾
	apiBase = strings.TrimRight(apiBase, "/")
	return &AnthropicClient{
		apiKey:  apiKey,
		apiBase: apiBase,
		model:   model,
		http: &http.Client{
			Timeout: 120 * time.Second,
		},
	}
}

// Chat 调用 /v1/messages 端点。
func (c *AnthropicClient) Chat(ctx context.Context, req *core.LLMRequest) (*core.LLMResponse, error) {
	if c.apiKey == "" {
		return nil, fmt.Errorf("anthropic client: API key not set")
	}

	// 分离 system 消息和对话消息
	var systemPrompt string
	var messages []map[string]string
	for _, m := range req.Messages {
		if m.Role == "system" {
			if systemPrompt != "" {
				systemPrompt += "\n"
			}
			systemPrompt += m.Content
		} else {
			messages = append(messages, map[string]string{
				"role":    m.Role,
				"content": m.Content,
			})
		}
	}

	maxTokens := req.MaxTokens
	if maxTokens == 0 {
		maxTokens = 4096
	}
	temp := req.Temperature
	if temp == 0 {
		temp = 0.7
	}

	body := map[string]any{
		"model":       c.model,
		"max_tokens":  maxTokens,
		"messages":    messages,
		"temperature": temp,
	}
	if systemPrompt != "" {
		body["system"] = systemPrompt
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	url := c.apiBase + "/v1/messages"
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("new request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", c.apiKey)
	httpReq.Header.Set("anthropic-version", "2023-06-01")

	resp, err := c.http.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("http call: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("anthropic API error %d: %s", resp.StatusCode, string(raw))
	}

	var result struct {
		Model    string `json:"model"`
		Content  []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
		Usage struct {
			InputTokens  int `json:"input_tokens"`
			OutputTokens int `json:"output_tokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	// 拼接所有 text 内容块
	var content strings.Builder
	for _, block := range result.Content {
		if block.Type == "text" {
			content.WriteString(block.Text)
		}
	}

	return &core.LLMResponse{
		Content:          content.String(),
		Model:            result.Model,
		Provider:         "anthropic",
		PromptTokens:     result.Usage.InputTokens,
		CompletionTokens: result.Usage.OutputTokens,
	}, nil
}

// Embed 调用 Anthropic 兼容的嵌入接口。
// 注意：Anthropic 官方 API 不提供 embedding 端点，
// 此实现返回空向量，CLI 模式下向量检索退化为关键词匹配。
func (c *AnthropicClient) Embed(ctx context.Context, text string) ([]float32, error) {
	return nil, nil
}

// EmbedBatch 批量嵌入。
func (c *AnthropicClient) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	// Anthropic 不提供 embedding，返回 nil（关键词匹配降级）
	return nil, nil
}
