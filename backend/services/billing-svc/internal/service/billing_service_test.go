package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/bidwriter/services/billing-svc/internal/model"
	"github.com/bidwriter/shared/pkg/tenant"
	"github.com/google/uuid"
)

// fakeStore is a hand-rolled in-memory Store for service tests.
type fakeStore struct {
	getOrCreateBudgetFn func(ctx context.Context, month string) (*model.Budget, error)
	updateBudgetFn      func(ctx context.Context, b *model.Budget) error
	addTransactionFn    func(ctx context.Context, req *model.AddTransactionRequest) (*model.Transaction, error)
	getTransactionsFn   func(ctx context.Context, limit int) ([]*model.Transaction, error)
}

func (f *fakeStore) GetOrCreateBudget(ctx context.Context, month string) (*model.Budget, error) {
	return f.getOrCreateBudgetFn(ctx, month)
}
func (f *fakeStore) UpdateBudget(ctx context.Context, b *model.Budget) error {
	return f.updateBudgetFn(ctx, b)
}
func (f *fakeStore) AddTransaction(ctx context.Context, req *model.AddTransactionRequest) (*model.Transaction, error) {
	return f.addTransactionFn(ctx, req)
}
func (f *fakeStore) GetTransactions(ctx context.Context, limit int) ([]*model.Transaction, error) {
	return f.getTransactionsFn(ctx, limit)
}

func ctxWithTenant() context.Context {
	return tenant.WithTenant(context.Background(), uuid.NewString())
}

func TestGetCurrentBudget_LimitZero_PercentUsedZero(t *testing.T) {
	st := &fakeStore{
		getOrCreateBudgetFn: func(_ context.Context, _ string) (*model.Budget, error) {
			return &model.Budget{ID: uuid.New(), Month: "2026-06", LimitCents: 0, SpentCents: 0}, nil
		},
	}
	svc := NewBillingService(st)

	got, err := svc.GetCurrentBudget(ctxWithTenant())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.PercentUsed != 0 {
		t.Errorf("percentUsed = %v, want 0", got.PercentUsed)
	}
	if got.LimitCents != 0 || got.SpentCents != 0 {
		t.Errorf("limit/spent = %d/%d, want 0/0", got.LimitCents, got.SpentCents)
	}
}

func TestGetCurrentBudget_LimitPositiveSpentZero_PercentZero(t *testing.T) {
	st := &fakeStore{
		getOrCreateBudgetFn: func(_ context.Context, _ string) (*model.Budget, error) {
			return &model.Budget{ID: uuid.New(), Month: "2026-06", LimitCents: 10000, SpentCents: 0}, nil
		},
	}
	svc := NewBillingService(st)

	got, err := svc.GetCurrentBudget(ctxWithTenant())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.PercentUsed != 0 {
		t.Errorf("percentUsed = %v, want 0", got.PercentUsed)
	}
}

func TestGetCurrentBudget_HalfSpent_FiftyPercent(t *testing.T) {
	st := &fakeStore{
		getOrCreateBudgetFn: func(_ context.Context, _ string) (*model.Budget, error) {
			return &model.Budget{ID: uuid.New(), Month: "2026-06", LimitCents: 2000, SpentCents: 1000}, nil
		},
	}
	svc := NewBillingService(st)

	got, err := svc.GetCurrentBudget(ctxWithTenant())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.PercentUsed != 50 {
		t.Errorf("percentUsed = %v, want 50", got.PercentUsed)
	}
	if got.LimitCents != 2000 || got.SpentCents != 1000 {
		t.Errorf("limit/spent = %d/%d, want 2000/1000", got.LimitCents, got.SpentCents)
	}
	if got.Budget == nil || got.Budget.Month != "2026-06" {
		t.Errorf("expected populated budget pointer, got %+v", got.Budget)
	}
}

func TestGetCurrentBudget_UsesCurrentUTCMonth(t *testing.T) {
	wantMonth := time.Now().UTC().Format("2006-01")
	var gotMonth string
	st := &fakeStore{
		getOrCreateBudgetFn: func(_ context.Context, month string) (*model.Budget, error) {
			gotMonth = month
			return &model.Budget{ID: uuid.New(), Month: month, LimitCents: 0, SpentCents: 0}, nil
		},
	}
	svc := NewBillingService(st)

	if _, err := svc.GetCurrentBudget(ctxWithTenant()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotMonth != wantMonth {
		t.Errorf("month passed to store = %q, want %q (current UTC month)", gotMonth, wantMonth)
	}
}

func TestGetCurrentBudget_StoreErrorPropagates(t *testing.T) {
	wantErr := errors.New("db down")
	st := &fakeStore{
		getOrCreateBudgetFn: func(_ context.Context, _ string) (*model.Budget, error) {
			return nil, wantErr
		},
	}
	svc := NewBillingService(st)

	got, err := svc.GetCurrentBudget(ctxWithTenant())
	if !errors.Is(err, wantErr) {
		t.Fatalf("err = %v, want %v", err, wantErr)
	}
	if got != nil {
		t.Errorf("summary = %+v, want nil", got)
	}
}

func TestSetBudget_FlowsThrough_ReturnsUpdatedBudget(t *testing.T) {
	id := uuid.New()
	existing := &model.Budget{ID: id, Month: "2026-07", LimitCents: 0, SpentCents: 0}

	st := &fakeStore{
		getOrCreateBudgetFn: func(_ context.Context, month string) (*model.Budget, error) {
			if month != "2026-07" {
				t.Errorf("month = %s, want 2026-07", month)
			}
			return existing, nil
		},
		updateBudgetFn: func(_ context.Context, b *model.Budget) error {
			if b.LimitCents != 50000 {
				t.Errorf("limit = %d, want 50000", b.LimitCents)
			}
			if b.ID != id {
				t.Errorf("budget id = %s, want %s", b.ID, id)
			}
			return nil
		},
	}
	svc := NewBillingService(st)

	got, err := svc.SetBudget(ctxWithTenant(), "2026-07", 50000)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.LimitCents != 50000 {
		t.Errorf("returned limit = %d, want 50000", got.LimitCents)
	}
	if got.ID != id {
		t.Errorf("returned id = %s, want %s", got.ID, id)
	}
}

func TestSetBudget_UpdateBudgetError(t *testing.T) {
	wantErr := errors.New("update failed")
	st := &fakeStore{
		getOrCreateBudgetFn: func(_ context.Context, _ string) (*model.Budget, error) {
			return &model.Budget{ID: uuid.New(), Month: "2026-07", LimitCents: 0}, nil
		},
		updateBudgetFn: func(_ context.Context, _ *model.Budget) error {
			return wantErr
		},
	}
	svc := NewBillingService(st)

	got, err := svc.SetBudget(ctxWithTenant(), "2026-07", 1000)
	if !errors.Is(err, wantErr) {
		t.Fatalf("err = %v, want %v", err, wantErr)
	}
	if got != nil {
		t.Errorf("budget = %+v, want nil", got)
	}
}

func TestSetBudget_GetOrCreateError(t *testing.T) {
	wantErr := errors.New("lookup failed")
	st := &fakeStore{
		getOrCreateBudgetFn: func(_ context.Context, _ string) (*model.Budget, error) {
			return nil, wantErr
		},
	}
	svc := NewBillingService(st)

	got, err := svc.SetBudget(ctxWithTenant(), "2026-07", 1000)
	if !errors.Is(err, wantErr) {
		t.Fatalf("err = %v, want %v", err, wantErr)
	}
	if got != nil {
		t.Errorf("budget = %+v, want nil", got)
	}
}

func TestGetTransactions_LimitZero_NormalizedTo50(t *testing.T) {
	var gotLimit int
	st := &fakeStore{
		getTransactionsFn: func(_ context.Context, limit int) ([]*model.Transaction, error) {
			gotLimit = limit
			return nil, nil
		},
	}
	svc := NewBillingService(st)

	if _, err := svc.GetTransactions(ctxWithTenant(), 0); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotLimit != 50 {
		t.Errorf("limit passed to store = %d, want 50", gotLimit)
	}
}

func TestGetTransactions_LimitNegative_NormalizedTo50(t *testing.T) {
	var gotLimit int
	st := &fakeStore{
		getTransactionsFn: func(_ context.Context, limit int) ([]*model.Transaction, error) {
			gotLimit = limit
			return nil, nil
		},
	}
	svc := NewBillingService(st)

	if _, err := svc.GetTransactions(ctxWithTenant(), -5); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotLimit != 50 {
		t.Errorf("limit passed to store = %d, want 50", gotLimit)
	}
}

func TestGetTransactions_LimitTen_PassedThrough(t *testing.T) {
	var gotLimit int
	st := &fakeStore{
		getTransactionsFn: func(_ context.Context, limit int) ([]*model.Transaction, error) {
			gotLimit = limit
			return []*model.Transaction{{ID: uuid.New()}}, nil
		},
	}
	svc := NewBillingService(st)

	txs, err := svc.GetTransactions(ctxWithTenant(), 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotLimit != 10 {
		t.Errorf("limit passed to store = %d, want 10", gotLimit)
	}
	if len(txs) != 1 {
		t.Errorf("txs len = %d, want 1", len(txs))
	}
}