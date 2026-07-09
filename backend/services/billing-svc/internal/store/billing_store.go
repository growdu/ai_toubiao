package store

import (
	"context"
	"errors"
	"time"

	"github.com/bidwriter/services/billing-svc/internal/model"
	"github.com/bidwriter/shared/pkg/tenant"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrNotFound = errors.New("not found")

type Store struct {
	pool *pgxpool.Pool
}

func New(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

// GetOrCreateBudget gets or creates a budget for the current month.
func (s *Store) GetOrCreateBudget(ctx context.Context, month string) (*model.Budget, error) {
	tid, err := tenant.FromContext(ctx)
	if err != nil {
		return nil, err
	}
	var b model.Budget
	err = s.pool.QueryRow(ctx, `
		SELECT id, tenant_id, month, limit_cents, spent_cents, created_at, updated_at
		FROM billing_budgets WHERE tenant_id = $1 AND month = $2
	`, tid, month).Scan(&b.ID, &b.TenantID, &b.Month, &b.LimitCents, &b.SpentCents, &b.CreatedAt, &b.UpdatedAt)
	if err == nil {
		return &b, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}
	// Create new budget
	b.ID = uuid.New()
	b.TenantID, _ = uuid.Parse(tid)
	b.Month = month
	b.LimitCents = 0
	b.SpentCents = 0
	_, err = s.pool.Exec(ctx, `
		INSERT INTO billing_budgets (id, tenant_id, month, limit_cents, spent_cents, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, NOW(), NOW())
	`, b.ID, b.TenantID, b.Month, b.LimitCents, b.SpentCents)
	return &b, err
}

// UpdateBudget updates budget limit and spent amount.
func (s *Store) UpdateBudget(ctx context.Context, b *model.Budget) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE billing_budgets SET limit_cents=$1, spent_cents=$2, updated_at=NOW()
		WHERE id=$3
	`, b.LimitCents, b.SpentCents, b.ID)
	return err
}

// AddTransaction records a new transaction and updates budget spent.
func (s *Store) AddTransaction(ctx context.Context, req *model.AddTransactionRequest) (*model.Transaction, error) {
	// Get or create current month budget
	month := time.Now().UTC().Format("2006-01")
	budget, err := s.GetOrCreateBudget(ctx, month)
	if err != nil {
		return nil, err
	}

	tx := &model.Transaction{
		ID:           uuid.New(),
		TenantID:     budget.TenantID,
		BudgetID:     &budget.ID,
		Provider:     req.Provider,
		Model:        req.Model,
		TaskType:     req.TaskType,
		InputTokens:  req.InputTokens,
		OutputTokens: req.OutputTokens,
		CostCents:    req.CostCents,
		CallID:       req.CallID,
	}

	if _, err := s.pool.Exec(ctx, `
		INSERT INTO billing_transactions (id, tenant_id, budget_id, provider, model, task_type, input_tokens, output_tokens, cost_cents, call_id, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, NOW())
	`, tx.ID, tx.TenantID, tx.BudgetID, tx.Provider, tx.Model, tx.TaskType, tx.InputTokens, tx.OutputTokens, tx.CostCents, tx.CallID); err != nil {
		return nil, err
	}

	// Update budget spent
	if _, err := s.pool.Exec(ctx, `
		UPDATE billing_budgets SET spent_cents = spent_cents + $1, updated_at = NOW()
		WHERE id = $2
	`, req.CostCents, budget.ID); err != nil {
		return nil, err
	}

	return tx, nil
}

// GetTransactions returns transactions for the current tenant.
func (s *Store) GetTransactions(ctx context.Context, limit int) ([]*model.Transaction, error) {
	tid, err := tenant.FromContext(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := s.pool.Query(ctx, `
		SELECT id, tenant_id, budget_id, provider, model, task_type, input_tokens, output_tokens, cost_cents, call_id, created_at
		FROM billing_transactions WHERE tenant_id = $1
		ORDER BY created_at DESC LIMIT $2
	`, tid, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var txs []*model.Transaction
	for rows.Next() {
		var tx model.Transaction
		if err := rows.Scan(&tx.ID, &tx.TenantID, &tx.BudgetID, &tx.Provider, &tx.Model, &tx.TaskType, &tx.InputTokens, &tx.OutputTokens, &tx.CostCents, &tx.CallID, &tx.CreatedAt); err != nil {
			return nil, err
		}
		txs = append(txs, &tx)
	}
	return txs, rows.Err()
}

// GetBudget retrieves a specific budget.
func (s *Store) GetBudget(ctx context.Context, month string) (*model.Budget, error) {
	tid, err := tenant.FromContext(ctx)
	if err != nil {
		return nil, err
	}
	var b model.Budget
	err = s.pool.QueryRow(ctx, `
		SELECT id, tenant_id, month, limit_cents, spent_cents, created_at, updated_at
		FROM billing_budgets WHERE tenant_id = $1 AND month = $2
	`, tid, month).Scan(&b.ID, &b.TenantID, &b.Month, &b.LimitCents, &b.SpentCents, &b.CreatedAt, &b.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return &b, err
}


// UpdateTenantPlan sets the tenant's plan column (free/pro/enterprise).
// The CHECK constraint on tenants.plan guarantees we can only insert
// one of the three known tiers, so a bad value here errors out at the
// DB level rather than corrupting data.
func (s *Store) UpdateTenantPlan(ctx context.Context, plan string) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE tenants SET plan = $1, updated_at = NOW() WHERE id = $2`,
		plan, mustTenantID(ctx),
	)
	return err
}

// GetTenantPlan returns the current plan of the tenant in context.
func (s *Store) GetTenantPlan(ctx context.Context) (string, error) {
	var plan string
	err := s.pool.QueryRow(ctx,
		`SELECT plan FROM tenants WHERE id = $1`,
		mustTenantID(ctx),
	).Scan(&plan)
	return plan, err
}

// mustTenantID pulls the tenant id from context and panics if missing —
// all billing handlers are auth-gated so the context must carry a tenant.
func mustTenantID(ctx context.Context) uuid.UUID {
	tid, err := tenant.FromContext(ctx)
	if err != nil {
		// Should be unreachable thanks to auth middleware, but if it
		// does happen we'd rather know than silently corrupt rows.
		panic("billing: tenant missing from context: " + err.Error())
	}
	id, _ := uuid.Parse(tid)
	return id
}
