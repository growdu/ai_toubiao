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
