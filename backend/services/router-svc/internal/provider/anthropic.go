package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/bidwriter/services/router-svc/internal/model"
)

// AnthropicProvider implements Provider against Anthropic's /v1/messages API.
// It is intentionally hand-rolled (no SDK dependency) to keep the binary slim
// and the routing logic fully transparent.
type AnthropicProvider struct {
	apiKey  string
	baseURL string
	client  *http.Client
	pricing Pricing
}

// NewAnthropicProvider builds a provider. baseURL defaults to the public API
// when empty.
func NewAnthropicProvider(apiKey, baseURL string, p Pricing) *AnthropicProvider {
	if baseURL == "" {
		baseURL = "https://api.anthropic.com"
	}
	return &AnthropicProvider{
		apiKey:  apiKey,
		baseURL: strings.TrimRight(baseURL, "/"),
		client: &http.Client{
			Timeout: 180 * time.Second,
		},
		pricing: p,
	}
}

// Name implements Provider.
func (p *AnthropicProvider) Name() string { return "anthropic" }

// anthropicRequest is the wire shape for /v1/messages.
type anthropicRequest struct {
	Model       string             `json:"model"`
	Messages    []anthropicMessage `json:"messages"`
	MaxTokens   int                `json:"max_tokens"`
	Temperature float64            `json:"temperature,omitempty"`
	System      string             `json:"system,omitempty"`
}

type anthropicMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type anthropicResponse struct {
	ID      string `json:"id"`
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
	StopReason string `json:"stop_reason"`
	Usage      struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
	Model string `json:"model"`
}

// Chat implements Provider.
func (p *AnthropicProvider) Chat(ctx context.Context, in ChatInput) (*ChatOutput, error) {
	msgs, system := splitSystemMessages(in.Messages)
	req := anthropicRequest{
		Model:       in.Model,
		Messages:    msgs,
		MaxTokens:   orDefaultInt(in.MaxTokens, 4096),
		Temperature: in.Temperature,
		System:      system,
	}

	buf, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("anthropic: marshal: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/v1/messages", bytes.NewReader(buf))
	if err != nil {
		return nil, fmt.Errorf("anthropic: build request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", p.apiKey)
	httpReq.Header.Set("anthropic-version", "2023-06-01")

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("anthropic: http: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("anthropic: read: %w", err)
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("anthropic: HTTP %d: %s", resp.StatusCode, truncate(string(body), 500))
	}

	var parsed anthropicResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("anthropic: decode: %w", err)
	}

	content := ""
	for _, c := range parsed.Content {
		if c.Type == "text" {
			content += c.Text
		}
	}

	out := &ChatOutput{
		Content:          content,
		Model:            orDefault(parsed.Model, in.Model),
		PromptTokens:     parsed.Usage.InputTokens,
		CompletionTokens: parsed.Usage.OutputTokens,
	}
	out.CostUSD = float64(out.PromptTokens)/1_000_000*p.pricing.InputPerMTokensUSD +
		float64(out.CompletionTokens)/1_000_000*p.pricing.OutputPerMTokensUSD
	return out, nil
}

// EstimateCost implements Provider.
func (p *AnthropicProvider) EstimateCost(in ChatInput) float64 {
	promptTok := 0
	for _, m := range in.Messages {
		promptTok += len(m.Content) / 4
	}
	completionTok := orDefaultInt(in.MaxTokens, 4096)
	return float64(promptTok)/1_000_000*p.pricing.InputPerMTokensUSD +
		float64(completionTok)/1_000_000*p.pricing.OutputPerMTokensUSD
}

// HealthCheck implements Provider.
func (p *AnthropicProvider) HealthCheck(ctx context.Context) error {
	// Cheap probe: list models via /v1/models (available on Anthropic since 2024).
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.baseURL+"/v1/models", nil)
	if err != nil {
		return err
	}
	req.Header.Set("x-api-key", p.apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")
	resp, err := p.client.Do(req)
	if err != nil {
		return fmt.Errorf("anthropic: health: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 500 {
		return fmt.Errorf("anthropic: unhealthy HTTP %d", resp.StatusCode)
	}
	return nil
}

// splitSystemMessages pulls system messages out of the conversation and returns
// them concatenated as the system prompt (Anthropic's API requires this).
func splitSystemMessages(msgs []model.Message) ([]anthropicMessage, string) {
	var sysBuf strings.Builder
	conv := make([]anthropicMessage, 0, len(msgs))
	for _, m := range msgs {
		if m.Role == "system" {
			sysBuf.WriteString(m.Content)
			sysBuf.WriteString("\n")
			continue
		}
		conv = append(conv, anthropicMessage{Role: m.Role, Content: m.Content})
	}
	return conv, strings.TrimSpace(sysBuf.String())
}

func orDefaultInt(n, def int) int {
	if n <= 0 {
		return def
	}
	return n
}