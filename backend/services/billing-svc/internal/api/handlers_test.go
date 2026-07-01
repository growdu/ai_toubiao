package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/bidwriter/services/billing-svc/internal/model"
	"github.com/bidwriter/shared/pkg/tenant"
	"github.com/google/uuid"
)

// fakeService is a hand-rolled in-memory billingService for handler tests.
type fakeService struct {
	getCurrentBudgetFn func(ctx context.Context) (*model.BudgetSummary, error)
	setBudgetFn        func(ctx context.Context, month string, limitCents int64) (*model.Budget, error)
	addTransactionFn   func(ctx context.Context, req *model.AddTransactionRequest) (*model.Transaction, error)
	getTransactionsFn  func(ctx context.Context, limit int) ([]*model.Transaction, error)
}

func (f *fakeService) GetCurrentBudget(ctx context.Context) (*model.BudgetSummary, error) {
	return f.getCurrentBudgetFn(ctx)
}
func (f *fakeService) SetBudget(ctx context.Context, month string, limitCents int64) (*model.Budget, error) {
	return f.setBudgetFn(ctx, month, limitCents)
}
func (f *fakeService) AddTransaction(ctx context.Context, req *model.AddTransactionRequest) (*model.Transaction, error) {
	return f.addTransactionFn(ctx, req)
}
func (f *fakeService) GetTransactions(ctx context.Context, limit int) ([]*model.Transaction, error) {
	return f.getTransactionsFn(ctx, limit)
}

func newTestHandler(svc billingService) *Handlers {
	return &Handlers{Service: svc, Log: slog.New(slog.NewTextHandler(io.Discard, nil))}
}

func ctxWithTenant() context.Context {
	return tenant.WithTenant(context.Background(), uuid.NewString())
}

func doRequest(t *testing.T, h http.Handler, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var rdr io.Reader
	if body != nil {
		// Raw-string body for malformed-JSON tests passes through unchanged
		// when the caller wraps the body string in a bytes.Buffer manually;
		// otherwise we marshal the value to JSON.
		if s, ok := body.(string); ok {
			rdr = bytes.NewBufferString(s)
		} else {
			b, err := json.Marshal(body)
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}
			rdr = bytes.NewReader(b)
		}
	}
	req := httptest.NewRequest(method, path, rdr).WithContext(ctxWithTenant())
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	return w
}

// doRequestNoTenant is like doRequest but skips ctxWithTenant() so handlers
// see a request without a tenant_id in context.
func doRequestNoTenant(t *testing.T, h http.Handler, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var rdr io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		rdr = bytes.NewReader(b)
	}
	req := httptest.NewRequest(method, path, rdr)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	return w
}

func TestGetCurrentBudget_Success(t *testing.T) {
	want := &model.BudgetSummary{
		Budget:      &model.Budget{ID: uuid.New(), Month: "2026-06", LimitCents: 10000, SpentCents: 2500},
		SpentCents:  2500,
		LimitCents:  10000,
		PercentUsed: 25,
	}
	svc := &fakeService{
		getCurrentBudgetFn: func(_ context.Context) (*model.BudgetSummary, error) {
			return want, nil
		},
	}
	h := newTestHandler(svc)
	w := doRequest(t, h.Routes(), http.MethodGet, "/api/v1/billing/budget/current", nil)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", w.Code, w.Body.String())
	}
	var got struct {
		Data *model.BudgetSummary `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.Data == nil {
		t.Fatalf("data is nil; body=%s", w.Body.String())
	}
	if got.Data.LimitCents != want.LimitCents || got.Data.SpentCents != want.SpentCents || got.Data.PercentUsed != want.PercentUsed {
		t.Errorf("got %+v, want %+v", got.Data, want)
	}
}

func TestGetCurrentBudget_NoTenant_Unauthorized(t *testing.T) {
	svc := &fakeService{
		getCurrentBudgetFn: func(_ context.Context) (*model.BudgetSummary, error) {
			t.Fatal("service should not be called when tenant is missing")
			return nil, nil
		},
	}
	h := newTestHandler(svc)
	w := doRequestNoTenant(t, h.Routes(), http.MethodGet, "/api/v1/billing/budget/current", nil)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401; body=%s", w.Code, w.Body.String())
	}
	if !bytes.Contains(w.Body.Bytes(), []byte(`"error"`)) {
		t.Errorf("body missing error envelope: %s", w.Body.String())
	}
}

func TestSetBudget_BadJSON_BadRequest(t *testing.T) {
	svc := &fakeService{
		setBudgetFn: func(_ context.Context, _ string, _ int64) (*model.Budget, error) {
			t.Fatal("service should not be called when JSON is malformed")
			return nil, nil
		},
	}
	h := newTestHandler(svc)
	w := doRequest(t, h.Routes(), http.MethodPost, "/api/v1/billing/budget", "{not-valid-json")

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body=%s", w.Code, w.Body.String())
	}
	if !bytes.Contains(w.Body.Bytes(), []byte(`"error"`)) {
		t.Errorf("body missing error envelope: %s", w.Body.String())
	}
}

func TestSetBudget_ServiceError_InternalError(t *testing.T) {
	svc := &fakeService{
		setBudgetFn: func(_ context.Context, _ string, _ int64) (*model.Budget, error) {
			return nil, errors.New("db unavailable")
		},
	}
	h := newTestHandler(svc)
	w := doRequest(t, h.Routes(), http.MethodPost, "/api/v1/billing/budget",
		map[string]any{"month": "2026-07", "limit_cents": 10000})

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500; body=%s", w.Code, w.Body.String())
	}
	if !bytes.Contains(w.Body.Bytes(), []byte(`"error"`)) {
		t.Errorf("body missing error envelope: %s", w.Body.String())
	}
}

func TestSetBudget_Success(t *testing.T) {
	want := &model.Budget{ID: uuid.New(), Month: "2026-07", LimitCents: 50000}
	svc := &fakeService{
		setBudgetFn: func(_ context.Context, m string, l int64) (*model.Budget, error) {
			if m != "2026-07" || l != 50000 {
				t.Errorf("got month=%s limit=%d, want 2026-07/50000", m, l)
			}
			return want, nil
		},
	}
	h := newTestHandler(svc)
	w := doRequest(t, h.Routes(), http.MethodPost, "/api/v1/billing/budget",
		map[string]any{"month": "2026-07", "limit_cents": 50000})

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", w.Code, w.Body.String())
	}
	var got struct {
		Data *model.Budget `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.Data == nil || got.Data.ID != want.ID {
		t.Errorf("unexpected response: %+v", got)
	}
}

func TestAddTransaction_Created201(t *testing.T) {
	want := &model.Transaction{ID: uuid.New(), Provider: "anthropic", Model: "claude", TaskType: "rfp_parse", CostCents: 42}
	svc := &fakeService{
		addTransactionFn: func(_ context.Context, req *model.AddTransactionRequest) (*model.Transaction, error) {
			if req.Provider != "anthropic" || req.CostCents != 42 {
				t.Errorf("unexpected req: %+v", req)
			}
			return want, nil
		},
	}
	h := newTestHandler(svc)
	w := doRequest(t, h.Routes(), http.MethodPost, "/api/v1/billing/transactions",
		map[string]any{
			"provider":      "anthropic",
			"model":         "claude",
			"task_type":     "rfp_parse",
			"input_tokens":  100,
			"output_tokens": 200,
			"cost_cents":    42,
		})

	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body=%s", w.Code, w.Body.String())
	}
	var got struct {
		Data *model.Transaction `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.Data == nil || got.Data.ID != want.ID {
		t.Errorf("unexpected response: %+v", got)
	}
}

func TestGetTransactions_DefaultLimit(t *testing.T) {
	// When the query string is absent, the handler passes 0 to the service;
	// the service is responsible for normalizing that to 50. Here we just
	// verify the handler happily dispatches the request and returns 200.
	var gotLimit int
	var called bool
	svc := &fakeService{
		getTransactionsFn: func(_ context.Context, limit int) ([]*model.Transaction, error) {
			gotLimit = limit
			called = true
			return []*model.Transaction{{ID: uuid.New()}}, nil
		},
	}
	h := newTestHandler(svc)
	w := doRequest(t, h.Routes(), http.MethodGet, "/api/v1/billing/transactions", nil)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", w.Code, w.Body.String())
	}
	if !called {
		t.Fatal("service was not called")
	}
	if gotLimit != 0 {
		t.Errorf("handler should pass raw query value (0 when absent), got %d", gotLimit)
	}
}

func TestGetTransactions_LimitQueryParam(t *testing.T) {
	var gotLimit int
	svc := &fakeService{
		getTransactionsFn: func(_ context.Context, limit int) ([]*model.Transaction, error) {
			gotLimit = limit
			return []*model.Transaction{{ID: uuid.New()}}, nil
		},
	}
	h := newTestHandler(svc)
	w := doRequest(t, h.Routes(), http.MethodGet, "/api/v1/billing/transactions?limit=5", nil)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", w.Code, w.Body.String())
	}
	if gotLimit != 5 {
		t.Errorf("limit passed to service = %d, want 5", gotLimit)
	}
}

func TestGetTransactions_ServiceError_InternalError(t *testing.T) {
	svc := &fakeService{
		getTransactionsFn: func(_ context.Context, _ int) ([]*model.Transaction, error) {
			return nil, errors.New("query failed")
		},
	}
	h := newTestHandler(svc)
	w := doRequest(t, h.Routes(), http.MethodGet, "/api/v1/billing/transactions?limit=10", nil)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500; body=%s", w.Code, w.Body.String())
	}
}