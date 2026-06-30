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

// OpenAICompatible implements Provider against the OpenAI /v1/chat/completions
// protocol. It covers OpenAI, DeepSeek, Ollama, and any other OpenAI-shaped API.
type OpenAICompatible struct {
	name     string
	baseURL  string
	apiKey   string
	client   *http.Client
	pricing  Pricing
}

// Pricing describes per-token USD rates.
type Pricing struct {
	InputPerMTokensUSD  float64
	OutputPerMTokensUSD float64
}

// NewOpenAICompatible builds a provider with the given identity.
func NewOpenAICompatible(name, baseURL, apiKey string, p Pricing) *OpenAICompatible {
	return &OpenAICompatible{
		name:    name,
		baseURL: strings.TrimRight(baseURL, "/"),
		apiKey:  apiKey,
		client: &http.Client{
			Timeout: 120 * time.Second,
		},
		pricing: p,
	}
}

// Name implements Provider.
func (p *OpenAICompatible) Name() string { return p.name }

// Chat implements Provider.
func (p *OpenAICompatible) Chat(ctx context.Context, in ChatInput) (*ChatOutput, error) {
	body := map[string]any{
		"model":    in.Model,
		"messages": toOpenAIMessages(in.Messages),
	}
	if in.MaxTokens > 0 {
		body["max_tokens"] = in.MaxTokens
	}
	if in.Temperature > 0 {
		body["temperature"] = in.Temperature
	}

	buf, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("%s: marshal request: %w", p.name, err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/chat/completions", bytes.NewReader(buf))
	if err != nil {
		return nil, fmt.Errorf("%s: build request: %w", p.name, err)
	}
	req.Header.Set("Content-Type", "application/json")
	if p.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+p.apiKey)
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%s: http: %w", p.name, err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("%s: read body: %w", p.name, err)
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("%s: HTTP %d: %s", p.name, resp.StatusCode, truncate(string(respBody), 500))
	}

	var parsed struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
			FinishReason string `json:"finish_reason"`
		} `json:"choices"`
		Usage struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
			TotalTokens      int `json:"total_tokens"`
		} `json:"usage"`
		Model string `json:"model"`
	}
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return nil, fmt.Errorf("%s: decode response: %w", p.name, err)
	}
	if len(parsed.Choices) == 0 {
		return nil, fmt.Errorf("%s: no choices in response", p.name)
	}

	out := &ChatOutput{
		Content:          parsed.Choices[0].Message.Content,
		Model:            orDefault(parsed.Model, in.Model),
		PromptTokens:     parsed.Usage.PromptTokens,
		CompletionTokens: parsed.Usage.CompletionTokens,
	}
	out.CostUSD = float64(out.PromptTokens)/1_000_000*p.pricing.InputPerMTokensUSD +
		float64(out.CompletionTokens)/1_000_000*p.pricing.OutputPerMTokensUSD
	return out, nil
}

// EstimateCost implements Provider.
func (p *OpenAICompatible) EstimateCost(in ChatInput) float64 {
	promptTok := 0
	for _, m := range in.Messages {
		promptTok += len(m.Content) / 4
	}
	completionTok := in.MaxTokens
	if completionTok == 0 {
		completionTok = 512
	}
	return float64(promptTok)/1_000_000*p.pricing.InputPerMTokensUSD +
		float64(completionTok)/1_000_000*p.pricing.OutputPerMTokensUSD
}

// HealthCheck implements Provider.
func (p *OpenAICompatible) HealthCheck(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.baseURL+"/models", nil)
	if err != nil {
		return err
	}
	if p.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+p.apiKey)
	}
	resp, err := p.client.Do(req)
	if err != nil {
		return fmt.Errorf("%s: health: %w", p.name, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 500 {
		return fmt.Errorf("%s: unhealthy HTTP %d", p.name, resp.StatusCode)
	}
	return nil
}

func toOpenAIMessages(msgs []model.Message) []map[string]string {
	out := make([]map[string]string, len(msgs))
	for i, m := range msgs {
		out[i] = map[string]string{"role": m.Role, "content": m.Content}
	}
	return out
}

func orDefault(s, def string) string {
	if s == "" {
		return def
	}
	return s
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "...(truncated)"
}