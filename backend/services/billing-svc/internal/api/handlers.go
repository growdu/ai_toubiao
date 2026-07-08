package api

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/bidwriter/services/billing-svc/internal/model"
	"github.com/bidwriter/services/billing-svc/internal/store"
	"github.com/bidwriter/shared/pkg/httperr"
	"github.com/bidwriter/shared/pkg/logger"
	"github.com/bidwriter/shared/pkg/tenant"
	"github.com/go-chi/chi/v5"
)

// billingService is the service contract required by Handlers. Defined at the
// consumer (api package) so handlers can be unit-tested with a fake. The
// concrete *service.BillingService satisfies this interface naturally.
type billingService interface {
	GetCurrentBudget(ctx context.Context) (*model.BudgetSummary, error)
	SetBudget(ctx context.Context, month string, limitCents int64) (*model.Budget, error)
	AddTransaction(ctx context.Context, req *model.AddTransactionRequest) (*model.Transaction, error)
	GetTransactions(ctx context.Context, limit int) ([]*model.Transaction, error)
	Checkout(ctx context.Context, req *model.CheckoutRequest) (*model.CheckoutResult, error)
	GetCurrentPlan(ctx context.Context) (string, error)
}

type Handlers struct {
	Store   *store.Store
	Service billingService
	Log     *slog.Logger
}

func (h *Handlers) Routes() http.Handler {
	r := chi.NewRouter()

	r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte(`{"status":"ok"}`))
	})
	r.Get("/readyz", func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte(`{"status":"ready"}`))
	})

	r.Route("/api/v1/billing", func(r chi.Router) {
		r.Get("/budget/current", h.getCurrentBudget)
		r.Post("/budget", h.setBudget)
		r.Post("/transactions", h.addTransaction)
		r.Get("/transactions", h.getTransactions)
		r.Post("/checkout", h.checkout)
		r.Get("/plan", h.getCurrentPlan)
	})

	return r
}

func currentMonth() string {
	return time.Now().Format("2006-01")
}

// GET /api/v1/billing/budget/current
func (h *Handlers) getCurrentBudget(w http.ResponseWriter, r *http.Request) {
	rid := logger.RequestIDFrom(r.Context())
	if _, err := tenant.FromContext(r.Context()); err != nil {
		httperr.Unauthorized(w, rid)
		return
	}

	summary, err := h.Service.GetCurrentBudget(r.Context())
	if err != nil {
		h.Log.Error("get current budget", slog.String("err", err.Error()))
		httperr.InternalError(w, rid)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": summary})
}

// POST /api/v1/billing/budget
func (h *Handlers) setBudget(w http.ResponseWriter, r *http.Request) {
	rid := logger.RequestIDFrom(r.Context())
	if _, err := tenant.FromContext(r.Context()); err != nil {
		httperr.Unauthorized(w, rid)
		return
	}

	var req model.CreateBudgetRequest
	if err := readJSON(r.Body, &req); err != nil {
		httperr.InvalidInput(w, rid, "invalid JSON", nil)
		return
	}

	budget, err := h.Service.SetBudget(r.Context(), req.Month, req.LimitCents)
	if err != nil {
		h.Log.Error("set budget", slog.String("err", err.Error()))
		httperr.InternalError(w, rid)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": budget})
}

// POST /api/v1/billing/transactions
func (h *Handlers) addTransaction(w http.ResponseWriter, r *http.Request) {
	rid := logger.RequestIDFrom(r.Context())
	if _, err := tenant.FromContext(r.Context()); err != nil {
		httperr.Unauthorized(w, rid)
		return
	}

	var req model.AddTransactionRequest
	if err := readJSON(r.Body, &req); err != nil {
		httperr.InvalidInput(w, rid, "invalid JSON", nil)
		return
	}

	tx, err := h.Service.AddTransaction(r.Context(), &req)
	if err != nil {
		h.Log.Error("add transaction", slog.String("err", err.Error()))
		httperr.InternalError(w, rid)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"data": tx})
}

// GET /api/v1/billing/transactions
func (h *Handlers) getTransactions(w http.ResponseWriter, r *http.Request) {
	rid := logger.RequestIDFrom(r.Context())
	if _, err := tenant.FromContext(r.Context()); err != nil {
		httperr.Unauthorized(w, rid)
		return
	}

	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	txs, err := h.Service.GetTransactions(r.Context(), limit)
	if err != nil {
		h.Log.Error("get transactions", slog.String("err", err.Error()))
		httperr.InternalError(w, rid)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": txs})
}

// POST /api/v1/billing/checkout
func (h *Handlers) checkout(w http.ResponseWriter, r *http.Request) {
	rid := logger.RequestIDFrom(r.Context())
	if _, err := tenant.FromContext(r.Context()); err != nil {
		httperr.Unauthorized(w, rid)
		return
	}

	var req model.CheckoutRequest
	if err := readJSON(r.Body, &req); err != nil {
		httperr.InvalidInput(w, rid, "invalid JSON", nil)
		return
	}

	result, err := h.Service.Checkout(r.Context(), &req)
	if err != nil {
		h.Log.Error("checkout", slog.String("err", err.Error()))
		httperr.InvalidInput(w, rid, err.Error(), nil)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": result})
}

// GET /api/v1/billing/plan
func (h *Handlers) getCurrentPlan(w http.ResponseWriter, r *http.Request) {
	rid := logger.RequestIDFrom(r.Context())
	if _, err := tenant.FromContext(r.Context()); err != nil {
		httperr.Unauthorized(w, rid)
		return
	}

	plan, err := h.Service.GetCurrentPlan(r.Context())
	if err != nil {
		h.Log.Error("get current plan", slog.String("err", err.Error()))
		httperr.InternalError(w, rid)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": map[string]string{"plan": plan}})
}

// ---- helpers ----

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func readJSON(body io.ReadCloser, v any) error {
	if body == nil {
		return errors.New("empty body")
	}
	defer body.Close()
	dec := json.NewDecoder(io.LimitReader(body, 1<<20))
	dec.DisallowUnknownFields()
	return dec.Decode(v)
}

var _ = strconv.Quote
