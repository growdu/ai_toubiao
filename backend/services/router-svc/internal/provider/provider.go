// Package provider defines the AI provider abstraction and ships adapters for
// Anthropic, OpenAI, and OpenAI-compatible APIs (DeepSeek, Ollama).
package provider

import (
	"context"

	"github.com/bidwriter/services/router-svc/internal/model"
)

// ChatInput is the wire-agnostic request passed to every provider.
type ChatInput struct {
	Model       string
	Messages    []model.Message
	MaxTokens   int
	Temperature float64
}

// ChatOutput is the wire-agnostic response from every provider.
type ChatOutput struct {
	Content          string
	Model            string
	PromptTokens     int
	CompletionTokens int
	// CostUSD is computed from the response tokens using the provider's pricing.
	CostUSD float64
}

// Provider is the contract every AI backend must satisfy.
type Provider interface {
	// Name returns a stable identifier (e.g. "anthropic", "openai", "deepseek").
	Name() string
	// Chat sends a single conversation and returns the model's reply.
	Chat(ctx context.Context, in ChatInput) (*ChatOutput, error)
	// Embed returns vector embeddings for the given texts.
	Embed(ctx context.Context, in EmbeddingInput) (*EmbeddingOutput, error)
	// EstimateCost returns an upper-bound USD cost for the call, used for
	// pre-flight budget checks.
	EstimateCost(in ChatInput) float64
	// HealthCheck verifies that the provider's API is reachable.
	HealthCheck(ctx context.Context) error
}

// Registry looks up providers by name. Used by the router to build fallback chains.
type Registry interface {
	Get(name string) (Provider, bool)
	Names() []string
}

// MapRegistry is a simple in-memory provider registry.
type MapRegistry struct {
	providers map[string]Provider
}

// NewMapRegistry builds a registry from a name→provider map.
func NewMapRegistry(ps map[string]Provider) *MapRegistry {
	r := &MapRegistry{providers: map[string]Provider{}}
	for k, v := range ps {
		r.providers[k] = v
	}
	return r
}

// Get returns the provider for name, or false if not registered.
func (r *MapRegistry) Get(name string) (Provider, bool) {
	p, ok := r.providers[name]
	return p, ok
}

// Names returns all registered provider names.
func (r *MapRegistry) Names() []string {
	out := make([]string, 0, len(r.providers))
	for k := range r.providers {
		out = append(out, k)
	}
	return out
}

// EmbeddingInput is the request input for embedding generation.
type EmbeddingInput struct {
	Model  string
	Texts  []string
	Format string // "float" or "base64", default "float"
}

// EmbeddingOutput is the response from an embedding provider.
type EmbeddingOutput struct {
	Embeddings [][]float32
	Model      string
	Usage      EmbeddingUsage
}

// EmbeddingUsage reports token usage for an embedding call.
type EmbeddingUsage struct {
	PromptTokens int `json:"prompt_tokens"`
	TotalTokens  int `json:"total_tokens"`
}