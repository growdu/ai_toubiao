package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/google/uuid"
)

// RouterClient calls router-svc for chat and embed operations.
type RouterClient struct {
	baseURL string
	client  *http.Client
}

// NewRouterClient builds a client pointing at the given router-svc base URL.
func NewRouterClient(routerURL string) *RouterClient {
	return &RouterClient{
		baseURL: routerURL,
		client: &http.Client{Timeout: 60 * 1e9},
	}
}

// EmbedRequest is the request for the /embed endpoint.
type EmbedRequest struct {
	TenantID uuid.UUID `json:"tenant_id"`
	Texts    []string  `json:"texts"`
	Model    string    `json:"model,omitempty"`
	Task     string    `json:"task,omitempty"`
}

// EmbedResponse is the response from the /embed endpoint.
type EmbedResponse struct {
	Embeddings [][]float32 `json:"embeddings"`
	Model      string      `json:"model"`
	Provider   string      `json:"provider"`
}

// Embed calls router-svc /api/v1/router/embed.
func (c *RouterClient) Embed(ctx context.Context, tenantID uuid.UUID, texts []string, model string) (*EmbedResponse, error) {
	reqBody := EmbedRequest{
		TenantID: tenantID,
		Texts:    texts,
		Model:    model,
		Task:     "embed",
	}
	buf, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal embed request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/v1/router/embed", bytes.NewReader(buf))
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
		Data EmbedResponse `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&wrapper); err != nil {
		return nil, fmt.Errorf("decode embed response: %w", err)
	}
	return &wrapper.Data, nil
}