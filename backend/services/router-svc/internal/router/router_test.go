package router_test

import (
	"context"
	"sync"
	"testing"

	"github.com/google/uuid"

	"github.com/bidwriter/services/router-svc/internal/model"
	"github.com/bidwriter/services/router-svc/internal/provider"
	"github.com/bidwriter/services/router-svc/internal/router"
)

// flakyProvider fails the first N calls then succeeds. Used to exercise the
// fallback chain.
type flakyProvider struct {
	name    string
	failN   int
	mu      sync.Mutex
	calls   int
	content string
}

func (p *flakyProvider) Name() string { return p.name }

func (p *flakyProvider) Chat(ctx context.Context, in provider.ChatInput) (*provider.ChatOutput, error) {
	p.mu.Lock()
	p.calls++
	n := p.calls
	p.mu.Unlock()
	if n <= p.failN {
		return nil, errSimulated
	}
	return &provider.ChatOutput{
		Content:          p.content,
		Model:            in.Model,
		PromptTokens:     10,
		CompletionTokens: 5,
		CostUSD:          0.0001,
	}, nil
}

func (p *flakyProvider) EstimateCost(in provider.ChatInput) float64 { return 0.0001 }
func (p *flakyProvider) HealthCheck(ctx context.Context) error      { return nil }
func (p *flakyProvider) Embed(ctx context.Context, in provider.EmbeddingInput) (*provider.EmbeddingOutput, error) {
	return &provider.EmbeddingOutput{Embeddings: [][]float32{{0}}, Model: "flaky"}, nil
}

var errSimulated = &simulatedError{}

type simulatedError struct{}

func (*simulatedError) Error() string { return "simulated provider failure" }

func newFlaky(name string, failN int, content string) *flakyProvider {
	return &flakyProvider{name: name, failN: failN, content: content}
}

// flakyErr also counts call-log emissions.
type captureSink struct {
	mu   sync.Mutex
	logs []model.CallLog
}

func (s *captureSink) Add(l model.CallLog) {
	s.mu.Lock()
	s.logs = append(s.logs, l)
	s.mu.Unlock()
}

func (s *captureSink) Len() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.logs)
}

func TestRouter_HappyPath(t *testing.T) {
	primary := provider.NewMockProvider().WithResponse("hello world")
	registry := provider.NewMapRegistry(map[string]provider.Provider{
		"primary": primary,
	})
	routes := router.NewInMemoryRoutesStore(&router.RoutesConfig{
		Version: 1,
		Routes: []router.RouteEntry{{
			Task:    "test_task",
			Primary: router.TargetConfig{Provider: "primary", Model: "mock"},
			Budget:  router.BudgetConfig{MaxCostUSD: 1.0, TimeoutSeconds: 5},
			Cache:   router.CacheConfig{Enabled: false},
		}},
	})
	cache := router.NewLRUCache(10)
	budget := router.NewBudgetChecker()
	sink := &captureSink{}

	r := router.New(registry, routes, cache, budget, sink.Add)
	resp, err := r.Route(context.Background(), &model.ChatRequest{
		TenantID:  uuid.New(),
		Task:      "test_task",
		Messages:  []model.Message{{Role: "user", Content: "hi"}},
		MaxTokens: 100,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Content != "hello world" {
		t.Errorf("expected 'hello world', got %q", resp.Content)
	}
	if primary.Calls() != 1 {
		t.Errorf("expected 1 primary call, got %d", primary.Calls())
	}
	if sink.Len() != 1 {
		t.Errorf("expected 1 call log, got %d", sink.Len())
	}
}

func TestRouter_FallbackOnFailure(t *testing.T) {
	flaky := newFlaky("primary", 1, "primary content")
	backup := provider.NewMockProvider().WithResponse("backup content")
	registry := provider.NewMapRegistry(map[string]provider.Provider{
		"primary": flaky,
		"backup":  backup,
	})
	routes := router.NewInMemoryRoutesStore(&router.RoutesConfig{
		Version: 1,
		Routes: []router.RouteEntry{{
			Task:    "fallback_task",
			Primary: router.TargetConfig{Provider: "primary", Model: "m"},
			Fallback: []router.TargetConfig{
				{Provider: "backup", Model: "m"},
			},
			Budget: router.BudgetConfig{MaxCostUSD: 1.0, TimeoutSeconds: 5},
			Cache:  router.CacheConfig{Enabled: false},
		}},
	})
	r := router.New(registry, routes, router.NewLRUCache(10), router.NewBudgetChecker(), nil)

	// 1st call → primary fails (calls=1, failN=1), fallback succeeds
	resp1, err := r.Route(context.Background(), &model.ChatRequest{
		TenantID: uuid.New(), Task: "fallback_task",
		Messages: []model.Message{{Role: "user", Content: "hi"}}, MaxTokens: 10,
	})
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if resp1.Content != "backup content" || !resp1.FallbackUsed {
		t.Errorf("expected fallback success: %+v", resp1)
	}

	// 2nd call → primary succeeds (calls=2 > failN=1)
	resp2, err := r.Route(context.Background(), &model.ChatRequest{
		TenantID: uuid.New(), Task: "fallback_task",
		Messages: []model.Message{{Role: "user", Content: "hi"}}, MaxTokens: 10,
	})
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if resp2.Content != "primary content" || resp2.FallbackUsed {
		t.Errorf("expected primary success on second call: %+v", resp2)
	}
}

func TestRouter_NoFallback(t *testing.T) {
	flaky := newFlaky("only", 100, "")
	registry := provider.NewMapRegistry(map[string]provider.Provider{"only": flaky})
	routes := router.NewInMemoryRoutesStore(&router.RoutesConfig{
		Version: 1,
		Routes: []router.RouteEntry{{
			Task:     "audit_agent",
			Primary:  router.TargetConfig{Provider: "only", Model: "m"},
			Fallback: []router.TargetConfig{},
			Budget:   router.BudgetConfig{MaxCostUSD: 1.0, TimeoutSeconds: 5},
			Cache:    router.CacheConfig{Enabled: false},
		}},
	})
	r := router.New(registry, routes, router.NewLRUCache(10), router.NewBudgetChecker(), nil)

	_, err := r.Route(context.Background(), &model.ChatRequest{
		TenantID: uuid.New(), Task: "audit_agent", NoFallback: true,
		Messages: []model.Message{{Role: "user", Content: "x"}}, MaxTokens: 10,
	})
	if err == nil {
		t.Fatal("expected error when all providers fail with NoFallback")
	}
}

func TestRouter_UnknownTask(t *testing.T) {
	registry := provider.NewMapRegistry(map[string]provider.Provider{"p": provider.NewMockProvider()})
	routes := router.NewInMemoryRoutesStore(router.DefaultRoutes())
	r := router.New(registry, routes, router.NewLRUCache(10), router.NewBudgetChecker(), nil)

	_, err := r.Route(context.Background(), &model.ChatRequest{
		TenantID: uuid.New(), Task: model.Task("unknown_task"),
		Messages: []model.Message{{Role: "user", Content: "x"}},
	})
	if err == nil {
		t.Fatal("expected error for unknown task")
	}
}

func TestRouter_CacheHit(t *testing.T) {
	mock := provider.NewMockProvider().WithResponse("cached content")
	registry := provider.NewMapRegistry(map[string]provider.Provider{"p": mock})
	routes := router.NewInMemoryRoutesStore(&router.RoutesConfig{
		Version: 1,
		Routes: []router.RouteEntry{{
			Task:    "cached_task",
			Primary: router.TargetConfig{Provider: "p", Model: "m"},
			Budget:  router.BudgetConfig{MaxCostUSD: 1.0, TimeoutSeconds: 5},
			Cache:   router.CacheConfig{Enabled: true, TTLSeconds: 60},
		}},
	})
	cache := router.NewLRUCache(10)
	sink := &captureSink{}
	r := router.New(registry, routes, cache, router.NewBudgetChecker(), sink.Add)

	req := &model.ChatRequest{
		TenantID: uuid.New(), Task: "cached_task",
		Messages: []model.Message{{Role: "user", Content: "same prompt"}}, MaxTokens: 10,
	}

	// 1st call → cache miss → provider call
	r1, err := r.Route(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if r1.CacheHit {
		t.Error("expected cache miss on first call")
	}

	// 2nd call → cache hit
	r2, err := r.Route(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if !r2.CacheHit {
		t.Error("expected cache hit on second identical call")
	}
	if mock.Calls() != 1 {
		t.Errorf("expected exactly 1 provider call, got %d", mock.Calls())
	}

	// Different prompt → cache miss again
	req.Messages = []model.Message{{Role: "user", Content: "different"}}
	_, err = r.Route(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if mock.Calls() != 2 {
		t.Errorf("expected 2 provider calls, got %d", mock.Calls())
	}

	// cache size should reflect 2 entries
	if cache.Len() != 2 {
		t.Errorf("expected 2 cache entries, got %d", cache.Len())
	}
	// sink should have 3 call logs (miss + hit + miss)
	if sink.Len() != 3 {
		t.Errorf("expected 3 call logs, got %d", sink.Len())
	}
}

func TestRouter_CacheBypass(t *testing.T) {
	mock := provider.NewMockProvider().WithResponse("x")
	registry := provider.NewMapRegistry(map[string]provider.Provider{"p": mock})
	routes := router.NewInMemoryRoutesStore(&router.RoutesConfig{
		Version: 1,
		Routes: []router.RouteEntry{{
			Task:    "bypass_task",
			Primary: router.TargetConfig{Provider: "p", Model: "m"},
			Budget:  router.BudgetConfig{MaxCostUSD: 1.0, TimeoutSeconds: 5},
			Cache:   router.CacheConfig{Enabled: true, TTLSeconds: 60},
		}},
	})
	r := router.New(registry, routes, router.NewLRUCache(10), router.NewBudgetChecker(), nil)
	req := &model.ChatRequest{
		TenantID: uuid.New(), Task: "bypass_task",
		Messages: []model.Message{{Role: "user", Content: "same"}}, MaxTokens: 10,
		CacheBypass: true,
	}
	for i := 0; i < 3; i++ {
		resp, err := r.Route(context.Background(), req)
		if err != nil {
			t.Fatal(err)
		}
		if resp.CacheHit {
			t.Fatalf("call %d: expected cache miss with bypass, got hit", i+1)
		}
	}
	if mock.Calls() != 3 {
		t.Errorf("expected 3 provider calls with bypass, got %d", mock.Calls())
	}
}

func TestRouter_BudgetExhaustion(t *testing.T) {
	mock := provider.NewMockProvider().WithCostPerCall(0.5).WithResponse("x")
	registry := provider.NewMapRegistry(map[string]provider.Provider{"p": mock})
	routes := router.NewInMemoryRoutesStore(&router.RoutesConfig{
		Version: 1,
		Routes: []router.RouteEntry{{
			Task:    "budget_task",
			Primary: router.TargetConfig{Provider: "p", Model: "m"},
			Budget:  router.BudgetConfig{MaxCostUSD: 1.0, TimeoutSeconds: 5},
			Cache:   router.CacheConfig{Enabled: false},
		}},
	})
	tenant := uuid.New()
	budget := router.NewBudgetChecker()
	budget.SetCap(tenant, 0.6) // < 2 × $0.5 estimate
	r := router.New(registry, routes, router.NewLRUCache(10), budget, nil)

	// 1st call: pre-check passes (0+0.5 ≤ 0.6); provider call succeeds; authoritative charge → spent=0.5
	_, err := r.Route(context.Background(), &model.ChatRequest{
		TenantID: tenant, Task: "budget_task",
		Messages: []model.Message{{Role: "user", Content: "x"}}, MaxTokens: 10,
	})
	if err != nil {
		t.Fatalf("first call should pass: %v", err)
	}

	// 2nd call: pre-check fails (0.5+0.5 > 0.6) → immediately rejected
	_, err = r.Route(context.Background(), &model.ChatRequest{
		TenantID: tenant, Task: "budget_task",
		Messages: []model.Message{{Role: "user", Content: "y"}}, MaxTokens: 10,
	})
	if err == nil {
		t.Fatal("second call should fail due to budget pre-check")
	}

	// 3rd call: still rejected
	_, err = r.Route(context.Background(), &model.ChatRequest{
		TenantID: tenant, Task: "budget_task",
		Messages: []model.Message{{Role: "user", Content: "z"}}, MaxTokens: 10,
	})
	if err == nil {
		t.Fatal("third call should also fail")
	}

	snap := budget.Snapshot(tenant, "budget_task")
	if snap.RemainingUSD >= 0.2 {
		t.Errorf("expected remaining < 0.2, got %f", snap.RemainingUSD)
	}
}

func TestBudgetChecker_ChargeAndSnapshot(t *testing.T) {
	b := router.NewBudgetChecker()
	tenant := uuid.New()
	b.SetCap(tenant, 1.0)

	// Use values that are exactly representable in IEEE 754 to avoid drift.
	if !b.Charge(tenant, 0.5) {
		t.Error("expected 0.5 charge to succeed")
	}
	if b.Charge(tenant, 0.6) {
		t.Error("expected 0.6 charge to fail (would exceed cap)")
	}
	if !b.Charge(tenant, 0.5) {
		t.Error("expected second 0.5 charge to succeed (spent=1.0)")
	}
	if b.Charge(tenant, 0.01) {
		t.Error("expected 0.01 charge to fail at cap")
	}

	snap := b.Snapshot(tenant, "x")
	if snap.MonthlyCapUSD != 1.0 {
		t.Errorf("cap: %f", snap.MonthlyCapUSD)
	}
	if snap.RemainingUSD != 0.0 {
		t.Errorf("remaining: %f (want 0.0)", snap.RemainingUSD)
	}
	if !snap.Exhausted {
		t.Error("expected exhausted")
	}

	// Refund path.
	b.Refund(tenant, 0.25)
	snap = b.Snapshot(tenant, "x")
	if snap.RemainingUSD != 0.25 {
		t.Errorf("after refund: %f (want 0.25)", snap.RemainingUSD)
	}

	// Charges should succeed again up to the cap.
	if !b.Charge(tenant, 0.25) {
		t.Error("expected post-refund charge to succeed")
	}
	if snap.Exhausted {
		t.Error("should not be exhausted after refund + small charge")
	}
}

func TestCacheKey_Deterministic(t *testing.T) {
	k1 := router.CacheKey("t", "m", "hello", 0.7)
	k2 := router.CacheKey("t", "m", "hello", 0.7)
	if k1 != k2 {
		t.Error("expected stable key for identical inputs")
	}
	k3 := router.CacheKey("t", "m", "hello", 0.8)
	if k1 == k3 {
		t.Error("temperature should affect key")
	}
	k4 := router.CacheKey("t", "other", "hello", 0.7)
	if k1 == k4 {
		t.Error("model should affect key")
	}
}

func TestLRUCache_Eviction(t *testing.T) {
	c := router.NewLRUCache(2)
	resp := &model.ChatResponse{Content: "x", Model: "m", Provider: "p"}
	c.Set("a", resp, 60_000_000_000) // 60s in ns
	c.Set("b", resp, 60_000_000_000)
	if c.Len() != 2 {
		t.Fatalf("expected 2 entries, got %d", c.Len())
	}
	c.Set("c", resp, 60_000_000_000) // should evict "a"
	if c.Len() != 2 {
		t.Fatalf("expected 2 entries after eviction, got %d", c.Len())
	}
	if c.Get("a") != nil {
		t.Error("expected 'a' to be evicted")
	}
	if c.Get("b") == nil || c.Get("c") == nil {
		t.Error("expected 'b' and 'c' to remain")
	}
}

func TestExtractJSON(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want any
		ok   bool
	}{
		{"plain object", `{"a":1}`, map[string]any{"a": float64(1)}, true},
		{"single quotes", "{'a':1}", map[string]any{"a": float64(1)}, true},
		{"trailing comma", `{"a":1,}`, map[string]any{"a": float64(1)}, true},
		{"with line comment", `// hi\n{"a":1}`, map[string]any{"a": float64(1)}, true},
		{"wrapped in prose", `Here is the JSON: {"a":1} -- end`, map[string]any{"a": float64(1)}, true},
		{"broken", `not json at all`, nil, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := provider.ExtractJSON(tc.in)
			if tc.ok {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if fmtSprint(got) != fmtSprint(tc.want) {
					t.Errorf("got %v, want %v", got, tc.want)
				}
			} else {
				if err == nil {
					t.Errorf("expected error for %q", tc.in)
				}
			}
		})
	}
}

// fmtSprint renders any value as a string for comparison without importing fmt
// in many places — keeps the test file compact.
func fmtSprint(v any) string {
	if v == nil {
		return ""
	}
	switch x := v.(type) {
	case string:
		return x
	case map[string]any:
		s := "{"
		first := true
		for k, vv := range x {
			if !first {
				s += ","
			}
			s += k + ":" + fmtSprint(vv)
			first = false
		}
		return s + "}"
	default:
		return ""
	}
}