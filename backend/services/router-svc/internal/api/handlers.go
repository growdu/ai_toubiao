// Package api wires HTTP handlers to the router core.
package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/bidwriter/services/router-svc/internal/middleware"
	"github.com/bidwriter/services/router-svc/internal/model"
	"github.com/bidwriter/services/router-svc/internal/provider"
	"github.com/bidwriter/services/router-svc/internal/router"
)

// Handlers bundles all dependencies needed by HTTP handlers.
type Handlers struct {
	Router      *router.Router
	Registry    provider.Registry
	RoutesStore *router.InMemoryRoutesStore
	Budget      *router.BudgetChecker
	Cache       *router.LRUCache
	Batcher     *router.CallLogBatcher
	Pool        *pgxpool.Pool
}

// Routes builds a chi router with all routes mounted under /api/v1.
func (h *Handlers) Routes() http.Handler {
	r := chi.NewRouter()

	r.Get("/healthz", h.Health)
	r.Get("/readyz", h.Ready)

	r.Route("/api/v1/router", func(r chi.Router) {
		r.Post("/chat", h.Chat)
		r.Post("/embed", h.Embed)
		r.Get("/routes", h.ListRoutes)
		r.Get("/calls", h.ListCalls)
		r.Get("/stats", h.Stats)
		r.Get("/health", h.ProviderHealth)
		r.Get("/budget", h.GetBudget)
		r.Put("/budget", h.SetBudget)
		r.Get("/cache/stats", h.CacheStats)
		r.Post("/cache/clear", h.CacheClear)
	})

	return r
}

// Health is a liveness probe.
func (h *Handlers) Health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// Ready is a readiness probe (checks DB connectivity).
func (h *Handlers) Ready(w http.ResponseWriter, r *http.Request) {
	if h.Pool != nil {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if err := h.Pool.Ping(ctx); err != nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]any{
				"status": "not_ready",
				"error":  err.Error(),
			})
			return
		}
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
}

// Chat handles POST /api/v1/router/chat.
func (h *Handlers) Chat(w http.ResponseWriter, r *http.Request) {
	var req model.ChatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_INPUT", "malformed JSON body")
		return
	}

	// Tenant resolution: prefer JWT claim, fall back to body (service-to-service).
	if claims := middleware.ClaimsFrom(r.Context()); claims != nil && claims.TenantID != "" {
		if tid, err := uuid.Parse(claims.TenantID); err == nil {
			req.TenantID = tid
		}
	}
	if req.TenantID == uuid.Nil {
		writeError(w, http.StatusBadRequest, "INVALID_INPUT", "tenant_id is required")
		return
	}
	if len(req.Messages) == 0 {
		writeError(w, http.StatusBadRequest, "INVALID_INPUT", "messages must not be empty")
		return
	}
	for i, m := range req.Messages {
		if m.Role != "system" && m.Role != "user" && m.Role != "assistant" {
			writeError(w, http.StatusBadRequest, "INVALID_INPUT", "messages["+strconv.Itoa(i)+"].role must be system|user|assistant")
			return
		}
		if m.Content == "" {
			writeError(w, http.StatusBadRequest, "INVALID_INPUT", "messages["+strconv.Itoa(i)+"].content must not be empty")
			return
		}
	}
	if req.Task == "" {
		writeError(w, http.StatusBadRequest, "INVALID_INPUT", "task is required")
		return
	}

	resp, err := h.Router.Route(r.Context(), &req)
	if err != nil {
		writeError(w, http.StatusBadGateway, "ROUTER_FAILED", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": resp})
}

// EmbedRequest is the request body for /embed.
type EmbedRequest struct {
	TenantID uuid.UUID   `json:"tenant_id"`
	Texts    []string    `json:"texts"`
	Model    string      `json:"model,omitempty"`
	Task     model.Task  `json:"task,omitempty"`
}

// Embed handles POST /api/v1/router/embed.
func (h *Handlers) Embed(w http.ResponseWriter, r *http.Request) {
	var req EmbedRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_INPUT", "malformed JSON body")
		return
	}
	if len(req.Texts) == 0 {
		writeError(w, http.StatusBadRequest, "INVALID_INPUT", "texts must not be empty")
		return
	}
	if req.TenantID == uuid.Nil {
		if claims := middleware.ClaimsFrom(r.Context()); claims != nil && claims.TenantID != "" {
			if tid, err := uuid.Parse(claims.TenantID); err == nil {
				req.TenantID = tid
			}
		}
	}
	if req.TenantID == uuid.Nil {
		writeError(w, http.StatusBadRequest, "INVALID_INPUT", "tenant_id is required")
		return
	}

	// Resolve model from task route if not specified.
	model := req.Model
	if model == "" && req.Task != "" {
		if route := h.RoutesStore.Get().GetRoute(string(req.Task)); route != nil {
			model = route.Primary.Model
		}
	}
	if model == "" {
		model = "text-embedding-3-small"
	}

	// Find an embedding-capable provider.
	providerName := ""
	var embOutput *provider.EmbeddingOutput
	var lastErr error

	// Try all providers that support embedding.
	for _, name := range h.Registry.Names() {
		p, _ := h.Registry.Get(name)
		if p == nil {
			continue
		}
		out, err := p.Embed(r.Context(), provider.EmbeddingInput{
			Model:  model,
			Texts:  req.Texts,
			Format: "float",
		})
		if err == nil {
			providerName = name
			embOutput = out
			break
		}
		lastErr = err
	}
	if embOutput == nil {
		writeError(w, http.StatusBadGateway, "EMBED_FAILED", fmt.Sprintf("no provider succeeded: %v", lastErr))
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"data": map[string]any{
			"embeddings": embOutput.Embeddings,
			"model":      embOutput.Model,
			"provider":   providerName,
			"usage":      embOutput.Usage,
		},
	})
}

// ListRoutes returns the active routing table.
func (h *Handlers) ListRoutes(w http.ResponseWriter, r *http.Request) {
	cfg := h.RoutesStore.Get()
	writeJSON(w, http.StatusOK, map[string]any{
		"version": cfg.Version,
		"routes":  cfg.Tasks(),
	})
}

// ListCalls returns recent call logs for the calling tenant.
func (h *Handlers) ListCalls(w http.ResponseWriter, r *http.Request) {
	if h.Pool == nil {
		writeError(w, http.StatusServiceUnavailable, "DB_UNAVAILABLE", "call logs require database")
		return
	}
	tenantID, ok := tenantFromContext(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "INVALID_INPUT", "tenant_id required")
		return
	}
	limit := parseIntQuery(r, "limit", 50, 500)

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	rows, err := h.Pool.Query(ctx, `
		SELECT id, task, provider, model, prompt_tokens, completion_tokens,
		       latency_ms, cost_usd, cache_hit, fallback_used, attempt, error, created_at
		FROM router_call_logs
		WHERE tenant_id = $1
		ORDER BY created_at DESC
		LIMIT $2
	`, tenantID, limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}
	defer rows.Close()

	logs := []map[string]any{}
	for rows.Next() {
		var (
			id                  uuid.UUID
			task, providerName, model string
			promptTok, compTok, latencyMs, attempt int
			costUSD             float64
			cacheHit, fallbackUsed bool
			errMsg              *string
			createdAt           time.Time
		)
		if err := rows.Scan(&id, &task, &providerName, &model, &promptTok, &compTok,
			&latencyMs, &costUSD, &cacheHit, &fallbackUsed, &attempt, &errMsg, &createdAt); err != nil {
			writeError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
			return
		}
		entry := map[string]any{
			"id":                id,
			"task":              task,
			"provider":          providerName,
			"model":             model,
			"prompt_tokens":     promptTok,
			"completion_tokens": compTok,
			"latency_ms":        latencyMs,
			"cost_usd":          costUSD,
			"cache_hit":         cacheHit,
			"fallback_used":     fallbackUsed,
			"attempt":           attempt,
			"created_at":        createdAt,
		}
		if errMsg != nil {
			entry["error"] = *errMsg
		}
		logs = append(logs, entry)
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"data": logs,
		"meta": map[string]any{"count": len(logs), "limit": limit},
	})
}

// Stats returns aggregate metrics for the calling tenant.
func (h *Handlers) Stats(w http.ResponseWriter, r *http.Request) {
	if h.Pool == nil {
		writeError(w, http.StatusServiceUnavailable, "DB_UNAVAILABLE", "stats require database")
		return
	}
	tenantID, ok := tenantFromContext(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "INVALID_INPUT", "tenant_id required")
		return
	}
	days := parseIntQuery(r, "days", 7, 90)
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	rows, err := h.Pool.Query(ctx, `
		SELECT task,
		       COUNT(*)                              AS calls,
		       COUNT(*) FILTER (WHERE cache_hit)     AS cache_hits,
		       COUNT(*) FILTER (WHERE fallback_used) AS fallbacks,
		       COUNT(*) FILTER (WHERE error IS NOT NULL) AS errors,
		       COALESCE(SUM(prompt_tokens), 0)      AS pt,
		       COALESCE(SUM(completion_tokens), 0)   AS ct,
		       COALESCE(SUM(cost_usd), 0)            AS cost,
		       COALESCE(AVG(latency_ms)::int, 0)     AS avg_latency
		FROM router_call_logs
		WHERE tenant_id = $1 AND created_at > NOW() - ($2 || ' days')::interval
		GROUP BY task
		ORDER BY task
	`, tenantID, strconv.Itoa(days))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}
	defer rows.Close()

	stats := []map[string]any{}
	for rows.Next() {
		var task string
		var calls, cacheHits, fallbacks, errors, pt, ct, avgLatency int
		var cost float64
		if err := rows.Scan(&task, &calls, &cacheHits, &fallbacks, &errors, &pt, &ct, &cost, &avgLatency); err != nil {
			writeError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
			return
		}
		stats = append(stats, map[string]any{
			"task":              task,
			"calls":             calls,
			"cache_hits":        cacheHits,
			"fallbacks":         fallbacks,
			"errors":            errors,
			"prompt_tokens":     pt,
			"completion_tokens": ct,
			"total_tokens":      pt + ct,
			"cost_usd":          cost,
			"avg_latency_ms":    avgLatency,
		})
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"data": stats,
		"meta": map[string]any{"days": days, "tenant_id": tenantID},
	})
}

// ProviderHealth pings every registered provider.
func (h *Handlers) ProviderHealth(w http.ResponseWriter, r *http.Request) {
	type result struct {
		Provider string `json:"provider"`
		OK       bool   `json:"ok"`
		Error    string `json:"error,omitempty"`
	}
	results := []result{}
	for _, name := range h.Registry.Names() {
		p, _ := h.Registry.Get(name)
		if p == nil {
			continue
		}
		err := p.HealthCheck(r.Context())
		entry := result{Provider: name, OK: err == nil}
		if err != nil {
			entry.Error = err.Error()
		}
		results = append(results, entry)
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": results})
}

// GetBudget returns the current tenant budget status.
func (h *Handlers) GetBudget(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := tenantFromContext(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "INVALID_INPUT", "tenant_id required")
		return
	}
	task := model.Task(defaultStr(r.URL.Query().Get("task"), "generic"))
	snap := h.Budget.Snapshot(tenantID, task)
	writeJSON(w, http.StatusOK, map[string]any{"data": snap})
}

// SetBudget updates the tenant cap. Body: {"cap_usd": 50.0}.
func (h *Handlers) SetBudget(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := tenantFromContext(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "INVALID_INPUT", "tenant_id required")
		return
	}
	var body struct {
		CapUSD float64 `json:"cap_usd"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_INPUT", "malformed JSON")
		return
	}
	if body.CapUSD < 0 {
		writeError(w, http.StatusBadRequest, "INVALID_INPUT", "cap_usd must be non-negative")
		return
	}
	h.Budget.SetCap(tenantID, body.CapUSD)
	writeJSON(w, http.StatusOK, map[string]any{
		"data": h.Budget.Snapshot(tenantID, "generic"),
	})
}

// CacheStats returns cache size and pending log buffer.
func (h *Handlers) CacheStats(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"data": map[string]any{
			"size":    h.Cache.Len(),
			"pending": h.Batcher.Pending(),
		},
	})
}

// CacheClear empties the L1 cache.
func (h *Handlers) CacheClear(w http.ResponseWriter, r *http.Request) {
	h.Cache.Clear()
	writeJSON(w, http.StatusOK, map[string]string{"status": "cleared"})
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, code, msg string) {
	writeJSON(w, status, map[string]any{
		"error": map[string]string{
			"code":    code,
			"message": msg,
		},
	})
}

func tenantFromContext(r *http.Request) (uuid.UUID, bool) {
	if claims := middleware.ClaimsFrom(r.Context()); claims != nil && claims.TenantID != "" {
		if tid, err := uuid.Parse(claims.TenantID); err == nil {
			return tid, true
		}
	}
	if v := r.URL.Query().Get("tenant_id"); v != "" {
		if tid, err := uuid.Parse(v); err == nil {
			return tid, true
		}
	}
	return uuid.Nil, false
}

func parseIntQuery(r *http.Request, key string, def, max int) int {
	v := r.URL.Query().Get(key)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil || n <= 0 {
		return def
	}
	if n > max {
		return max
	}
	return n
}

func defaultStr(s, def string) string {
	if s == "" {
		return def
	}
	return s
}