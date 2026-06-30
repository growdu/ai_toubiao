package router

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/bidwriter/services/router-svc/internal/model"
	"github.com/bidwriter/services/router-svc/internal/provider"
)

// BudgetChecker answers "does this tenant have remaining budget for this task?".
// In v1 we keep it in-process + thread-safe. A persistent implementation can
// replace it later (see router_tenant_budgets table).
type BudgetChecker struct {
	mu     sync.RWMutex
	spent  map[string]float64                                // tenantID → USD spent this period
	caps   map[string]float64                                // tenantID → cap (USD/month)
	period time.Time                                         // start of current period
}

// NewBudgetChecker builds a checker with default $100/month cap.
func NewBudgetChecker() *BudgetChecker {
	now := time.Now()
	period := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	return &BudgetChecker{
		spent:  map[string]float64{},
		caps:   map[string]float64{},
		period: period,
	}
}

// SetCap sets (or removes with 0) the cap for a tenant.
func (b *BudgetChecker) SetCap(tenantID uuid.UUID, capUSD float64) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if capUSD <= 0 {
		delete(b.caps, tenantID.String())
		return
	}
	b.caps[tenantID.String()] = capUSD
}

// Remaining returns the remaining USD for tenantID/task. task is currently unused
// (we track at tenant granularity) but is part of the signature for future
// per-task budgets.
func (b *BudgetChecker) Remaining(tenantID uuid.UUID, _ model.Task) float64 {
	b.mu.RLock()
	defer b.mu.RUnlock()
	cap, ok := b.caps[tenantID.String()]
	if !ok {
		cap = 100.0 // default
	}
	return cap - b.spent[tenantID.String()]
}

// Charge records a spend and returns whether it was accepted.
// Returns false if the charge would exceed the cap (with a small epsilon to
// guard against floating-point drift on otherwise-equal totals).
func (b *BudgetChecker) Charge(tenantID uuid.UUID, amount float64) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	cap, ok := b.caps[tenantID.String()]
	if !ok {
		cap = 100.0
	}
	if b.spent[tenantID.String()]+amount > cap+1e-6 {
		return false
	}
	b.spent[tenantID.String()] += amount
	return true
}

// Refund returns a previously charged amount. Used to roll back pre-flight
// estimates after a successful call records the authoritative cost.
func (b *BudgetChecker) Refund(tenantID uuid.UUID, amount float64) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.spent[tenantID.String()] -= amount
	if b.spent[tenantID.String()] < 0 {
		b.spent[tenantID.String()] = 0
	}
}

// Snapshot returns a BudgetStatus view.
func (b *BudgetChecker) Snapshot(tenantID uuid.UUID, task model.Task) model.BudgetStatus {
	b.mu.RLock()
	defer b.mu.RUnlock()
	cap, ok := b.caps[tenantID.String()]
	if !ok {
		cap = 100.0
	}
	spent := b.spent[tenantID.String()]
	return model.BudgetStatus{
		TenantID:       tenantID,
		Task:           task,
		MonthlyCapUSD:  cap,
		SpentUSD:       spent,
		RemainingUSD:   cap - spent,
		Exhausted:      cap-spent <= 0,
	}
}

// Router is the top-level call router. It is safe for concurrent use.
type Router struct {
	registry provider.Registry
	routes   *InMemoryRoutesStore
	cache    *LRUCache
	budget   *BudgetChecker

	// emit is called after every Chat attempt (success or failure) so the
	// call-log batcher can write to the DB asynchronously.
	emit func(model.CallLog)
}

// New builds a Router. emit may be nil; in that case call logs are dropped.
func New(reg provider.Registry, routes *InMemoryRoutesStore, cache *LRUCache, budget *BudgetChecker, emit func(model.CallLog)) *Router {
	return &Router{
		registry: reg,
		routes:   routes,
		cache:    cache,
		budget:   budget,
		emit:     emit,
	}
}

// Route is the main entry point. It selects a provider via the route table,
// attempts the call, falls back on failure, caches successful responses, and
// emits a structured call log.
func (r *Router) Route(ctx context.Context, req *model.ChatRequest) (*model.ChatResponse, error) {
	if req.Task == "" {
		return nil, fmt.Errorf("task is required")
	}

	route := r.routes.Get().GetRoute(string(req.Task))
	if route == nil {
		return nil, fmt.Errorf("no route configured for task %s", req.Task)
	}

	// Cache lookup
	key := CacheKey(req.Task, route.Primary.Model, flattenPrompt(req.Messages), req.Temperature)
	if !req.CacheBypass && route.Cache.Enabled {
		if cached := r.cache.Get(key); cached != nil {
			cached.CacheHit = true
			r.emitLog(*req, cached, true, false, 0, "")
			return cached, nil
		}
	}

	// Budget pre-check
	remaining := r.budget.Remaining(req.TenantID, req.Task)
	if remaining <= 0 {
		return nil, fmt.Errorf("budget exhausted for tenant %s (task %s)", req.TenantID, req.Task)
	}

	chain := r.buildChain(route, req.NoFallback)
	var lastErr error
	for i, target := range chain {
		prov, ok := r.registry.Get(target.Provider)
		if !ok {
			lastErr = fmt.Errorf("provider %s not registered", target.Provider)
			continue
		}

		// Pre-flight budget check against this provider's estimate. If the call
		// would exceed the cap, skip this target (unless it's the last one).
		estCost := prov.EstimateCost(provider.ChatInput{
			Model:     target.Model,
			Messages:  req.Messages,
			MaxTokens: req.MaxTokens,
		})
		if !r.budget.Charge(req.TenantID, estCost) {
			lastErr = fmt.Errorf("budget exceeded for provider %s (est $%.4f)", target.Provider, estCost)
			r.emitLog(*req, nil, false, i > 0, i+1, lastErr.Error())
			continue
		}
		// Refund the estimate so the actual cost (recorded after a successful
		// response) is the only authoritative charge.
		r.budget.Refund(req.TenantID, estCost)

		callCtx, cancel := context.WithTimeout(ctx, route.Budget.Timeout())
		start := time.Now()
		out, err := prov.Chat(callCtx, provider.ChatInput{
			Model:       target.Model,
			Messages:    req.Messages,
			MaxTokens:   req.MaxTokens,
			Temperature: req.Temperature,
		})
		cancel()

		latency := int(time.Since(start) / time.Millisecond)
		if err != nil {
			lastErr = err
			r.emitLog(*req, nil, false, i > 0, i+1, err.Error())
			continue
		}

		resp := &model.ChatResponse{
			Content:          out.Content,
			Model:            out.Model,
			Provider:         prov.Name(),
			PromptTokens:     out.PromptTokens,
			CompletionTokens: out.CompletionTokens,
			TotalTokens:      out.PromptTokens + out.CompletionTokens,
			LatencyMs:        latency,
			CostUSD:          out.CostUSD,
			CacheHit:         false,
			FallbackUsed:     i > 0,
			Attempt:          i + 1,
		}

		// Authoritative charge after a successful response.
		r.budget.Charge(req.TenantID, out.CostUSD)

		// Cache write
		if !req.CacheBypass && route.Cache.Enabled {
			r.cache.Set(key, resp, route.Cache.TTL())
		}

		r.emitLog(*req, resp, false, i > 0, i+1, "")
		return resp, nil
	}

	return nil, fmt.Errorf("all providers failed for task %s: %w", req.Task, lastErr)
}

func (r *Router) buildChain(route *RouteEntry, noFallback bool) []TargetConfig {
	chain := []TargetConfig{route.Primary}
	if !noFallback {
		chain = append(chain, route.Fallback...)
	}
	return chain
}

func (r *Router) emitLog(req model.ChatRequest, resp *model.ChatResponse, cacheHit, fallback bool, attempt int, errMsg string) {
	if r.emit == nil {
		return
	}
	log := model.CallLog{
		TenantID:   req.TenantID,
		Task:       req.Task,
		CacheHit:   cacheHit,
		FallbackUsed: fallback,
		Attempt:    attempt,
		Error:      errMsg,
		CreatedAt:  time.Now(),
		WorkflowID: req.WorkflowID,
		StepID:     req.StepID,
	}
	if resp != nil {
		log.Provider = resp.Provider
		log.Model = resp.Model
		log.PromptTokens = resp.PromptTokens
		log.CompletionTokens = resp.CompletionTokens
		log.LatencyMs = resp.LatencyMs
		log.CostUSD = resp.CostUSD
	}
	r.emit(log)
}

func flattenPrompt(msgs []model.Message) string {
	out := ""
	for _, m := range msgs {
		out += m.Role + ":" + m.Content + "|"
	}
	return out
}

// ErrNoRoute is returned when no route is configured for a task.
var ErrNoRoute = errors.New("no route configured")