package service

import (
	"context"
	"time"

	"github.com/bidwriter/services/billing-svc/internal/model"
)

// Store is the storage contract required by BillingService. Defined at the
// consumer (service package) so the service can be unit-tested with a fake
// without a live PG. The concrete *store.Store satisfies this interface
// naturally.
type Store interface {
	GetOrCreateBudget(ctx context.Context, month string) (*model.Budget, error)
	UpdateBudget(ctx context.Context, b *model.Budget) error
	AddTransaction(ctx context.Context, req *model.AddTransactionRequest) (*model.Transaction, error)
	GetTransactions(ctx context.Context, limit int) ([]*model.Transaction, error)
}

// BillingService handles billing operations.
type BillingService struct {
	store Store
}

func NewBillingService(s Store) *BillingService {
	return &BillingService{store: s}
}

// GetCurrentBudget returns the current month's budget summary.
func (s *BillingService) GetCurrentBudget(ctx context.Context) (*model.BudgetSummary, error) {
	month := time.Now().UTC().Format("2006-01")
	budget, err := s.store.GetOrCreateBudget(ctx, month)
	if err != nil {
		return nil, err
	}
	var percentUsed float64
	if budget.LimitCents > 0 {
		percentUsed = float64(budget.SpentCents) / float64(budget.LimitCents) * 100
	}
	return &model.BudgetSummary{
		Budget:       budget,
		SpentCents:   budget.SpentCents,
		LimitCents:  budget.LimitCents,
		PercentUsed: percentUsed,
	}, nil
}

// SetBudget sets the budget limit for a month.
func (s *BillingService) SetBudget(ctx context.Context, month string, limitCents int64) (*model.Budget, error) {
	budget, err := s.store.GetOrCreateBudget(ctx, month)
	if err != nil {
		return nil, err
	}
	budget.LimitCents = limitCents
	if err := s.store.UpdateBudget(ctx, budget); err != nil {
		return nil, err
	}
	return budget, nil
}

// AddTransaction records an AI API call transaction.
func (s *BillingService) AddTransaction(ctx context.Context, req *model.AddTransactionRequest) (*model.Transaction, error) {
	return s.store.AddTransaction(ctx, req)
}

// GetTransactions returns recent transactions.
func (s *BillingService) GetTransactions(ctx context.Context, limit int) ([]*model.Transaction, error) {
	if limit <= 0 {
		limit = 50
	}
	return s.store.GetTransactions(ctx, limit)
}
