// Package router builds the call-chain resolver that selects providers,
// enforces budgets, caches results, and emits structured call logs.
package router

import (
	"fmt"
	"os"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
)

// RoutesConfig is the top-level YAML schema.
type RoutesConfig struct {
	Version int          `yaml:"version"`
	Routes  []RouteEntry `yaml:"routes"`
}

// RouteEntry defines how a single task is routed.
type RouteEntry struct {
	Task     string         `yaml:"task"`
	Primary  TargetConfig   `yaml:"primary"`
	Fallback []TargetConfig `yaml:"fallback"`
	Budget   BudgetConfig   `yaml:"budget"`
	Cache    CacheConfig    `yaml:"cache"`
}

// TargetConfig is provider + model + pricing override.
type TargetConfig struct {
	Provider   string  `yaml:"provider"`
	Model      string  `yaml:"model"`
	MaxCostUSD float64 `yaml:"max_cost_usd,omitempty"`
}

// BudgetConfig caps a single call's spend.
type BudgetConfig struct {
	MaxInputTokens int     `yaml:"max_input_tokens"`
	MaxCostUSD     float64 `yaml:"max_cost_usd"`
	TimeoutSeconds int     `yaml:"timeout_seconds"`
}

// CacheConfig controls prompt caching for this route.
type CacheConfig struct {
	Enabled    bool `yaml:"enabled"`
	TTLSeconds int  `yaml:"ttl_seconds"`
}

// TTL returns the cache TTL as a time.Duration.
func (c CacheConfig) TTL() time.Duration {
	if c.TTLSeconds <= 0 {
		return 0
	}
	return time.Duration(c.TTLSeconds) * time.Second
}

// Timeout returns the call timeout as a time.Duration.
func (b BudgetConfig) Timeout() time.Duration {
	if b.TimeoutSeconds <= 0 {
		return 120 * time.Second
	}
	return time.Duration(b.TimeoutSeconds) * time.Second
}

// LoadRoutesConfig reads a routes YAML from disk.
func LoadRoutesConfig(path string) (*RoutesConfig, error) {
	buf, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read routes config %s: %w", path, err)
	}
	return ParseRoutesConfig(buf)
}

// ParseRoutesConfig decodes a YAML byte slice.
func ParseRoutesConfig(buf []byte) (*RoutesConfig, error) {
	var cfg RoutesConfig
	if err := yaml.Unmarshal(buf, &cfg); err != nil {
		return nil, fmt.Errorf("parse routes config: %w", err)
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return &cfg, nil
}

// DefaultRoutes returns the canonical built-in route table. Used when no YAML
// is present (dev mode, tests).
func DefaultRoutes() *RoutesConfig {
	return &RoutesConfig{
		Version: 1,
		Routes: []RouteEntry{
			{
				Task:    "rfp_parse",
				Primary: TargetConfig{Provider: "mock", Model: "mock-rfp"},
				Fallback: []TargetConfig{
					{Provider: "mock", Model: "mock-fast"},
				},
				Budget: BudgetConfig{MaxInputTokens: 100000, MaxCostUSD: 0.50, TimeoutSeconds: 120},
				Cache:  CacheConfig{Enabled: true, TTLSeconds: 86400},
			},
			{
				Task:    "outline_generate",
				Primary: TargetConfig{Provider: "mock", Model: "mock-outline"},
				Budget:  BudgetConfig{MaxInputTokens: 30000, MaxCostUSD: 0.05, TimeoutSeconds: 60},
				Cache:   CacheConfig{Enabled: true, TTLSeconds: 43200},
			},
			{
				Task:    "content_generate",
				Primary: TargetConfig{Provider: "mock", Model: "mock-write"},
				Budget:  BudgetConfig{MaxInputTokens: 50000, MaxCostUSD: 0.30, TimeoutSeconds: 180},
				Cache:   CacheConfig{Enabled: false},
			},
			{
				Task:     "audit_agent",
				Primary:  TargetConfig{Provider: "mock", Model: "mock-deep"},
				Fallback: []TargetConfig{}, // no fallback by design
				Budget:   BudgetConfig{MaxInputTokens: 500000, MaxCostUSD: 5.00, TimeoutSeconds: 600},
				Cache:    CacheConfig{Enabled: false},
			},
			{
				Task:    "*",
				Primary: TargetConfig{Provider: "mock", Model: "mock-default"},
				Budget:  BudgetConfig{MaxInputTokens: 16000, MaxCostUSD: 0.05, TimeoutSeconds: 60},
				Cache:   CacheConfig{Enabled: false},
			},
		},
	}
}

// Validate ensures the config is internally consistent.
func (c *RoutesConfig) Validate() error {
	if c == nil {
		return fmt.Errorf("routes config is nil")
	}
	seen := map[string]bool{}
	for i, r := range c.Routes {
		if r.Task == "" {
			return fmt.Errorf("routes[%d]: task is required", i)
		}
		if seen[r.Task] {
			return fmt.Errorf("routes[%d]: duplicate task %q", i, r.Task)
		}
		seen[r.Task] = true
		if r.Primary.Provider == "" || r.Primary.Model == "" {
			return fmt.Errorf("routes[%d] (%s): primary provider and model are required", i, r.Task)
		}
		for j, f := range r.Fallback {
			if f.Provider == "" || f.Model == "" {
				return fmt.Errorf("routes[%d] (%s).fallback[%d]: provider and model are required", i, r.Task, j)
			}
		}
	}
	return nil
}

// GetRoute returns the entry for a given task, or the wildcard entry if present.
// Returns nil only when neither an exact nor a "*" entry is configured.
func (c *RoutesConfig) GetRoute(task string) *RouteEntry {
	if c == nil {
		return nil
	}
	var wildcard *RouteEntry
	for i := range c.Routes {
		if c.Routes[i].Task == task {
			return &c.Routes[i]
		}
		if c.Routes[i].Task == "*" {
			w := c.Routes[i]
			wildcard = &w
		}
	}
	return wildcard
}

// Tasks returns the set of configured task names (excludes the wildcard "*").
func (c *RoutesConfig) Tasks() []string {
	if c == nil {
		return nil
	}
	out := make([]string, 0, len(c.Routes))
	for _, r := range c.Routes {
		if r.Task == "*" {
			continue
		}
		out = append(out, r.Task)
	}
	return out
}

// InMemoryRoutesStore provides concurrent-safe reload of routes.
// For v1 we load once at startup; reload API exists for future hot-reload.
type InMemoryRoutesStore struct {
	mu     sync.RWMutex
	config *RoutesConfig
}

// NewInMemoryRoutesStore builds a store seeded with cfg.
func NewInMemoryRoutesStore(cfg *RoutesConfig) *InMemoryRoutesStore {
	return &InMemoryRoutesStore{config: cfg}
}

// Get returns the current config.
func (s *InMemoryRoutesStore) Get() *RoutesConfig {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.config
}

// Replace atomically swaps in a new config.
func (s *InMemoryRoutesStore) Replace(cfg *RoutesConfig) {
	s.mu.Lock()
	s.config = cfg
	s.mu.Unlock()
}