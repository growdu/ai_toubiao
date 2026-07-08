package model

import (
	"time"

	"github.com/google/uuid"
)

// Budget represents a tenant's spending budget.
type Budget struct {
	ID         uuid.UUID  `json:"id" db:"id"`
	TenantID   uuid.UUID  `json:"tenant_id" db:"tenant_id"`
	Month      string     `json:"month" db:"month"` // "2026-06"
	LimitCents int64      `json:"limit_cents" db:"limit_cents"`
	SpentCents int64      `json:"spent_cents" db:"spent_cents"`
	CreatedAt  time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at" db:"updated_at"`
}

// Transaction records a single AI API call.
type Transaction struct {
	ID          uuid.UUID  `json:"id" db:"id"`
	TenantID    uuid.UUID  `json:"tenant_id" db:"tenant_id"`
	BudgetID    *uuid.UUID `json:"budget_id,omitempty" db:"budget_id"`
	Provider    string     `json:"provider" db:"provider"` // "anthropic", "openai", "deepseek"
	Model       string     `json:"model" db:"model"`
	TaskType    string     `json:"task_type" db:"task_type"` // "rfp_parse", "outline_gen", "content_gen", etc.
	InputTokens int        `json:"input_tokens" db:"input_tokens"`
	OutputTokens int       `json:"output_tokens" db:"output_tokens"`
	CostCents   int64      `json:"cost_cents" db:"cost_cents"`
	CallID      string     `json:"call_id" db:"call_id"` // router call log ID
	CreatedAt   time.Time  `json:"created_at" db:"created_at"`
}

// CreateBudgetRequest for setting a budget.
type CreateBudgetRequest struct {
	Month      string `json:"month" validate:"required"`
	LimitCents int64  `json:"limit_cents" validate:"required,min=0"`
}

// AddTransactionRequest for recording a transaction.
type AddTransactionRequest struct {
	Provider     string `json:"provider" validate:"required"`
	Model       string `json:"model" validate:"required"`
	TaskType    string `json:"task_type" validate:"required"`
	InputTokens int    `json:"input_tokens" validate:"min=0"`
	OutputTokens int   `json:"output_tokens" validate:"min=0"`
	CostCents   int64  `json:"cost_cents" validate:"min=0"`
	CallID      string `json:"call_id"`
}

// BudgetSummary is the current month budget status.
type BudgetSummary struct {
	Budget      *Budget `json:"budget,omitempty"`
	SpentCents int64   `json:"spent_cents"`
	LimitCents int64   `json:"limit_cents"`
	PercentUsed float64 `json:"percent_used"`
}

// Plan is a public billing tier. Plans are read-only catalog data —
// they're not stored in PG because they don't change often and the
// front-end renders them from this single source.
type Plan struct {
	// ID is the canonical plan key: "free" | "pro" | "enterprise".
	// Must match the tenants.plan CHECK constraint exactly.
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	PriceCents  int64    `json:"price_cents"`  // 0 for free, -1 for enterprise (custom)
	Currency    string   `json:"currency"`
	Period      string   `json:"period"`        // "month" | "year" | "custom"
	Description string   `json:"description"`
	Features    []string `json:"features"`
	Highlight   bool     `json:"highlight"`     // marks the recommended tier
}

// CheckoutRequest is what the front-end POSTs to upgrade a tenant to
// a paid plan. In dev mode (no real payment provider configured) we
// flip the tenant's plan column directly. In production the same
// shape would feed Stripe Checkout / WeChat Pay / etc.
type CheckoutRequest struct {
	PlanID string `json:"plan_id" validate:"required,oneof=free pro enterprise"`
	// PaymentMethodID is opaque to us — in dev it's a fake string like
	// "pm_demo_visa", in production it's a Stripe PaymentMethod id.
	// Optional: free upgrades don't need one.
	PaymentMethodID string `json:"payment_method_id,omitempty"`
}

// CheckoutResult is the response after a successful plan change. It
// includes the new tenant state so the UI can refresh without a
// second round-trip.
type CheckoutResult struct {
	TenantID  uuid.UUID `json:"tenant_id"`
	Plan      string    `json:"plan"`
	UpgradedAt time.Time `json:"upgraded_at"`
	// In production, this would include a Stripe subscription id,
	// checkout session URL, etc.
}
