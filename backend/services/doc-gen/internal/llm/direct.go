// 直连 OpenAI 兼容 API 的 LLM 客户端实现。
// 支持 OpenAI / DeepSeek / 其他兼容接口，CLI 模式默认使用。
package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/bidwriter/services/doc-gen/internal/core"
)

// DirectClient 直连 OpenAI 兼容 API。
type DirectClient struct {
	apiKey   string
	apiBase  string
	model    string
	embModel string
	http     *http.Client
}

// NewDirectClient 创建直连客户端。
func NewDirectClient(apiKey, apiBase, model, embModel string) *DirectClient {
	return &DirectClient{
		apiKey:   apiKey,
		apiBase:  apiBase,
		model:    model,
		embModel: embModel,
		http: &http.Client{
			Timeout: 120 * time.Second, // LLM 调用可能很慢
		},
	}
}

// Chat 调用 /chat/completions。
func (c *DirectClient) Chat(ctx context.Context, req *core.LLMRequest) (*core.LLMResponse, error) {
	if c.apiKey == "" {
		return nil, fmt.Errorf("direct LLM client: API key not set")
	}
	model := c.model
	maxTokens := req.MaxTokens
	if maxTokens == 0 {
		maxTokens = 4096
	}
	temp := req.Temperature
	if temp == 0 {
		temp = 0.7
	}

	body := map[string]any{
		"model":       model,
		"messages":    req.Messages,
		"max_tokens":  maxTokens,
		"temperature": temp,
	}
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	url := c.apiBase + "/chat/completions"
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("new request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)

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
		return nil, fmt.Errorf("LLM API error %d: %s", resp.StatusCode, string(raw))
	}

	var result struct {
		Model   string `json:"model"`
		Choices []struct {
			Message struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Usage struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}
	if len(result.Choices) == 0 {
		return nil, fmt.Errorf("LLM returned no choices")
	}

	return &core.LLMResponse{
		Content:          result.Choices[0].Message.Content,
		Model:            result.Model,
		Provider:         "direct",
		PromptTokens:     result.Usage.PromptTokens,
		CompletionTokens: result.Usage.CompletionTokens,
	}, nil
}

// Embed 调用 /embeddings。
func (c *DirectClient) Embed(ctx context.Context, text string) ([]float32, error) {
	vecs, err := c.EmbedBatch(ctx, []string{text})
	if err != nil {
		return nil, err
	}
	if len(vecs) == 0 {
		return nil, fmt.Errorf("empty embedding result")
	}
	return vecs[0], nil
}

// EmbedBatch 批量调用 /embeddings。
func (c *DirectClient) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	if c.apiKey == "" {
		return nil, fmt.Errorf("direct LLM client: API key not set")
	}
	if len(texts) == 0 {
		return nil, nil
	}

	body := map[string]any{
		"model": c.embModel,
		"input": texts,
	}
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal embed request: %w", err)
	}

	url := c.apiBase + "/embeddings"
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("new embed request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.http.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("http embed call: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read embed body: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("embed API error %d: %s", resp.StatusCode, string(raw))
	}

	var result struct {
		Data []struct {
			Embedding []float32 `json:"embedding"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, fmt.Errorf("unmarshal embed response: %w", err)
	}

	vecs := make([][]float32, len(result.Data))
	for i, d := range result.Data {
		vecs[i] = d.Embedding
	}
	return vecs, nil
}
