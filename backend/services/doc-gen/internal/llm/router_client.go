// 通过 router-svc HTTP API 调用 LLM 的客户端实现。
// 服务化模式（Phase2）使用；CLI 模式可选（指向本地 router-svc）。
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
	"github.com/google/uuid"
)

// RouterClient 通过 router-svc 调用 LLM。
type RouterClient struct {
	baseURL  string
	tenantID uuid.UUID
	http     *http.Client
}

// NewRouterClient 创建 router-svc 客户端。
func NewRouterClient(baseURL string, tenantID uuid.UUID) *RouterClient {
	return &RouterClient{
		baseURL:  baseURL,
		tenantID: tenantID,
		http: &http.Client{
			Timeout: 120 * time.Second,
		},
	}
}

// Chat 调用 router-svc 的 /router/chat 端点。
func (c *RouterClient) Chat(ctx context.Context, req *core.LLMRequest) (*core.LLMResponse, error) {
	body := map[string]any{
		"tenant_id":   c.tenantID,
		"task":        req.Task,
		"messages":    req.Messages,
		"max_tokens":  req.MaxTokens,
		"temperature": req.Temperature,
	}
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal router request: %w", err)
	}

	url := c.baseURL + "/router/chat"
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("new router request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("router http call: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read router body: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("router API error %d: %s", resp.StatusCode, string(raw))
	}

	var result core.LLMResponse
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, fmt.Errorf("unmarshal router response: %w", err)
	}
	return &result, nil
}

// Embed 调用 router-svc 的 embed 任务。
func (c *RouterClient) Embed(ctx context.Context, text string) ([]float32, error) {
	vecs, err := c.EmbedBatch(ctx, []string{text})
	if err != nil {
		return nil, err
	}
	if len(vecs) == 0 {
		return nil, fmt.Errorf("empty embedding result")
	}
	return vecs[0], nil
}

// EmbedBatch 批量嵌入。
func (c *RouterClient) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, nil
	}
	// router-svc 的 embed 端点（按 task=embed 路由）
	messages := make([]core.Message, len(texts))
	for i, t := range texts {
		messages[i] = core.Message{Role: "user", Content: t}
	}
	body := map[string]any{
		"tenant_id":  c.tenantID,
		"task":       "embed",
		"messages":   messages,
		"max_tokens": 8192,
	}
	jsonBody, _ := json.Marshal(body)

	url := c.baseURL + "/router/chat"
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("router embed error %d: %s", resp.StatusCode, string(raw))
	}

	// router-svc 返回的 content 是 JSON 数组的向量
	var llmResp core.LLMResponse
	if err := json.Unmarshal(raw, &llmResp); err != nil {
		return nil, err
	}
	var vecs [][]float32
	if err := json.Unmarshal([]byte(llmResp.Content), &vecs); err != nil {
		// 单个向量
		var single []float32
		if err2 := json.Unmarshal([]byte(llmResp.Content), &single); err2 != nil {
			return nil, fmt.Errorf("parse embedding: %w", err)
		}
		vecs = [][]float32{single}
	}
	return vecs, nil
}
