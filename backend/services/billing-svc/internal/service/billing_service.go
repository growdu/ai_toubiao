package service

import (
	"context"
	"fmt"
	"time"

	"github.com/bidwriter/services/billing-svc/internal/model"
	"github.com/bidwriter/shared/pkg/tenant"
	"github.com/google/uuid"
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
	UpdateTenantPlan(ctx context.Context, plan string) error
	GetTenantPlan(ctx context.Context) (string, error)
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


// ListPlans returns the static plan catalog. We don't load this from PG
// because (a) it's tiny and changes rarely, (b) keeping it in code
// lets the front-end import a single TypeScript type that mirrors
// this exact structure (defined in web/src/api/plans.ts).
func (s *BillingService) ListPlans() []model.Plan {
	return []model.Plan{
		{
			ID:          "free",
			Name:        "免费试用",
			PriceCents:  0,
			Currency:    "CNY",
			Period:      "month",
			Description: "完整功能体验，14 天试用",
			Features: []string{
				"1 个工作区",
				"3 份标书",
				"10 万 token 用量",
				"知识库 100 篇",
				"邮件支持",
			},
			Highlight: false,
		},
		{
			ID:          "pro",
			Name:        "专业版",
			PriceCents:  29900, // ¥299/月
			Currency:    "CNY",
			Period:      "month",
			Description: "中小型投标团队的标配",
			Features: []string{
				"5 个工作区",
				"不限标书数",
				"200 万 token / 月",
				"知识库不限",
				"工作日 8h 响应",
				"DeepSeek / GPT-4 切换",
				"API 接入",
			},
			Highlight: true,
		},
		{
			ID:          "enterprise",
			Name:        "企业版",
			PriceCents:  -1, // 定制报价
			Currency:    "CNY",
			Period:      "custom",
			Description: "私有部署、SSO、专属 SLA",
			Features: []string{
				"不限工作区",
				"不限标书 / token",
				"VPC 私有部署",
				"SSO / SAML",
				"7×24 专属支持",
				"Claude / GPT-4 全模型",
				"定制模型微调",
				"审计日志 12 个月",
			},
			Highlight: false,
		},
	}
}

// Checkout validates the requested plan and updates the tenant row.
// In dev (no real payment provider wired in) we flip the plan column
// directly; in production this would create a Stripe Customer +
// Subscription, then write the plan ID back to tenants after the
// webhook fires.
//
// We do NOT take payment here — that's deliberately out of scope for
// the demo. The handler-side validation guarantees only known plan IDs
// are accepted, so a malicious caller can't escalate themselves to
// "enterprise" via a typo.
func (s *BillingService) Checkout(ctx context.Context, req *model.CheckoutRequest) (*model.CheckoutResult, error) {
	switch req.PlanID {
	case "free", "pro", "enterprise":
	default:
		return nil, fmt.Errorf("invalid plan_id %q (allowed: free|pro|enterprise)", req.PlanID)
	}
	if err := s.store.UpdateTenantPlan(ctx, req.PlanID); err != nil {
		return nil, fmt.Errorf("update tenant plan: %w", err)
	}
	tid, err := tenant.FromContext(ctx)
	if err != nil {
		return nil, err
	}
	tenantID, _ := uuid.Parse(tid)
	return &model.CheckoutResult{
		TenantID:   tenantID,
		Plan:       req.PlanID,
		UpgradedAt: time.Now().UTC(),
	}, nil
}

// GetCurrentPlan is a convenience for the front-end's settings page.
func (s *BillingService) GetCurrentPlan(ctx context.Context) (string, error) {
	return s.store.GetTenantPlan(ctx)
}
