package workers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
)

// RouterClient calls the router-svc AI routing API.
type RouterClient struct {
	baseURL string
	client  *http.Client
}

// NewRouterClient builds a RouterClient.
func NewRouterClient(baseURL string) *RouterClient {
	return &RouterClient{baseURL: baseURL, client: &http.Client{Timeout: 120 * time.Second}}
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatRequest struct {
	TenantID  uuid.UUID     `json:"tenant_id"`
	Task      string        `json:"task"`
	Messages  []chatMessage `json:"messages"`
	MaxTokens int           `json:"max_tokens,omitempty"`
}

type chatResponse struct {
	Content          string  `json:"content"`
	Model            string  `json:"model"`
	Provider         string  `json:"provider"`
	PromptTokens     int     `json:"prompt_tokens"`
	CompletionTokens int     `json:"completion_tokens"`
	TotalTokens      int     `json:"total_tokens"`
	CostUSD          float64 `json:"cost_usd"`
	CacheHit         bool    `json:"cache_hit"`
}

// Chat calls router-svc /api/v1/router/chat.
func (c *RouterClient) Chat(ctx context.Context, tenantID uuid.UUID, task string, messages []chatMessage, maxTokens int) (*chatResponse, error) {
	reqBody := chatRequest{TenantID: tenantID, Task: task, Messages: messages, MaxTokens: maxTokens}
	buf, _ := json.Marshal(reqBody)
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/v1/router/chat", bytes.NewReader(buf))
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("router-svc HTTP %d", resp.StatusCode)
	}
	var wrapper struct {
		Data chatResponse `json:"data"`
	}
	json.NewDecoder(resp.Body).Decode(&wrapper)
	return &wrapper.Data, nil
}

// KnowledgeClient calls the knowledge-svc search API.
type KnowledgeClient struct {
	baseURL string
	client  *http.Client
}

// NewKnowledgeClient builds a KnowledgeClient.
func NewKnowledgeClient(baseURL string) *KnowledgeClient {
	return &KnowledgeClient{baseURL: baseURL, client: &http.Client{Timeout: 30 * time.Second}}
}

type searchRequest struct {
	Query    string `json:"query"`
	TopK     int    `json:"top_k"`
	Category string `json:"category,omitempty"`
}

type searchResult struct {
	ChunkID       uuid.UUID `json:"chunk_id"`
	MaterialID    uuid.UUID `json:"material_id"`
	MaterialTitle string    `json:"material_title"`
	Content       string    `json:"content"`
	Score         float64   `json:"score"`
}

type searchResponse struct {
	Hits  []searchResult `json:"hits"`
	Total int            `json:"total"`
}

// Search queries the knowledge base for relevant evidence.
func (c *KnowledgeClient) Search(ctx context.Context, tenantID uuid.UUID, query string, topK int) ([]searchResult, error) {
	reqBody := searchRequest{Query: query, TopK: topK}
	buf, _ := json.Marshal(reqBody)
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/v1/kb/search", bytes.NewReader(buf))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Tenant-ID", tenantID.String())
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("knowledge-svc HTTP %d", resp.StatusCode)
	}
	var wrapper struct {
		Data searchResponse `json:"data"`
	}
	json.NewDecoder(resp.Body).Decode(&wrapper)
	return wrapper.Data.Hits, nil
}

// DocumentClient calls the document-svc API.
type DocumentClient struct {
	baseURL string
	client  *http.Client
}

// NewDocumentClient builds a DocumentClient.
func NewDocumentClient(baseURL string) *DocumentClient {
	return &DocumentClient{baseURL: baseURL, client: &http.Client{Timeout: 60 * time.Second}}
}

// GetParseResult fetches the RFP parse result for a document.
func (c *DocumentClient) GetParseResult(ctx context.Context, docID uuid.UUID) (map[string]any, error) {
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/api/v1/documents/"+docID.String()+"/parse-result", nil)
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("document-svc HTTP %d", resp.StatusCode)
	}
	var wrapper struct {
		Data map[string]any `json:"data"`
	}
	json.NewDecoder(resp.Body).Decode(&wrapper)
	return wrapper.Data, nil
}

// AuditClient calls the audit-svc API.
type AuditClient struct {
	baseURL string
	client  *http.Client
}

// NewAuditClient builds an AuditClient.
func NewAuditClient(baseURL string) *AuditClient {
	return &AuditClient{baseURL: baseURL, client: &http.Client{Timeout: 120 * time.Second}}
}

// TriggerAudit POSTs to audit-svc to trigger a compliance audit.
func (c *AuditClient) TriggerAudit(ctx context.Context, bidJobID, tenantID uuid.UUID) error {
	reqBody := map[string]any{"bid_job_id": bidJobID.String()}
	buf, _ := json.Marshal(reqBody)
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/v1/audit/bidjobs/"+bidJobID.String()+"/report", bytes.NewReader(buf))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Tenant-ID", tenantID.String())
	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("audit-svc HTTP %d", resp.StatusCode)
	}
	return nil
}

// DocgenPatternsClient queries docgen-svc for historical bid patterns.
type DocgenPatternsClient struct {
	baseURL string
	client  *http.Client
}

func NewDocgenPatternsClient(baseURL string) *DocgenPatternsClient {
	return &DocgenPatternsClient{baseURL: baseURL, client: &http.Client{Timeout: 30 * time.Second}}
}

type docgenPattern struct {
	Industry        string  `json:"industry"`
	RFPType         string  `json:"rfp_type"`
	OutlineTemplate string  `json:"outline_template"`
	QualityScore    float64 `json:"quality_score"`
	Label           string  `json:"label"`
}

// GetPatterns retrieves historical bid patterns from docgen-svc.
func (c *DocgenPatternsClient) GetPatterns(ctx context.Context, industry, rfpType string, topK int) ([]docgenPattern, error) {
	url := fmt.Sprintf("%s/api/v1/docgen/patterns?industry=%s&rfp_type=%s&top_k=%d",
		c.baseURL, industry, rfpType, topK)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("docgen-svc patterns HTTP %d", resp.StatusCode)
	}
	var result struct {
		Patterns []docgenPattern `json:"patterns"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return result.Patterns, nil
}
