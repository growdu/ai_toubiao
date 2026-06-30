package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/google/uuid"
)

// RouterClient calls router-svc for LLM operations.
type RouterClient struct {
	baseURL string
	client  *http.Client
}

// NewRouterClient builds a client pointing at the given router-svc base URL.
func NewRouterClient(routerURL string) *RouterClient {
	return &RouterClient{
		baseURL: routerURL,
		client:  &http.Client{Timeout: 120 * 1e9},
	}
}

// ChatRequest mirrors router-svc internal model.ChatRequest.
type ChatRequest struct {
	TenantID    uuid.UUID `json:"tenant_id"`
	Task        string    `json:"task"`
	Messages    []Message `json:"messages"`
	MaxTokens   int       `json:"max_tokens,omitempty"`
	Temperature float64   `json:"temperature,omitempty"`
}

// Message is a chat message turn.
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ChatResponse mirrors router-svc response.
type ChatResponse struct {
	Content  string `json:"content"`
	Model    string `json:"model"`
	Provider string `json:"provider"`
}

// Chat calls router-svc /api/v1/router/chat.
func (c *RouterClient) Chat(ctx context.Context, tenantID uuid.UUID, task string, messages []Message, maxTokens int) (*ChatResponse, error) {
	reqBody := ChatRequest{
		TenantID:  tenantID,
		Task:      task,
		Messages:  messages,
		MaxTokens: maxTokens,
	}
	buf, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal chat request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/v1/router/chat", bytes.NewReader(buf))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("router-svc call: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		var errBody map[string]any
		json.NewDecoder(resp.Body).Decode(&errBody)
		return nil, fmt.Errorf("router-svc error: %d %v", resp.StatusCode, errBody)
	}

	var wrapper struct {
		Data ChatResponse `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&wrapper); err != nil {
		return nil, fmt.Errorf("decode chat response: %w", err)
	}
	return &wrapper.Data, nil
}