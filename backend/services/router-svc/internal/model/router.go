// Package model defines domain types used by router-svc.
package model

import (
	"time"

	"github.com/google/uuid"
)

// Task represents a logical AI task category used for routing decisions.
// Values are stable strings — they end up in DB columns and logs.
//
// The constant list below defines the canonical tasks for observability and
// stats. Routes may still be configured for arbitrary task names (e.g.
// "test_task" in unit tests) — use HasKnownTasks() to check.
type Task string

const (
	TaskRFPParse    Task = "rfp_parse"
	TaskOutlineGen  Task = "outline_generate"
	TaskContentGen  Task = "content_generate"
	TaskAuditNormal Task = "audit_normal"
	TaskAuditAgent  Task = "audit_agent"
	TaskImageGen    Task = "image_generate"
	TaskEmbed       Task = "embed"
	TaskGeneric     Task = "generic"
)

// KnownTasks returns every well-known task.
func KnownTasks() []Task {
	return []Task{
		TaskRFPParse, TaskOutlineGen, TaskContentGen,
		TaskAuditNormal, TaskAuditAgent, TaskImageGen, TaskEmbed, TaskGeneric,
	}
}

// IsKnown reports whether the task is one of the canonical tasks.
// Routes can still match arbitrary task names via the "*" wildcard — this
// function only powers cataloging and stats grouping.
func (t Task) IsKnown() bool {
	for _, k := range KnownTasks() {
		if k == t {
			return true
		}
	}
	return false
}

// Message is a single chat turn.
type Message struct {
	Role    string `json:"role" validate:"required,oneof=system user assistant"`
	Content string `json:"content" validate:"required"`
}

// ChatRequest is what callers send to /router/chat.
type ChatRequest struct {
	TenantID    uuid.UUID `json:"tenant_id"`
	WorkflowID  *uuid.UUID `json:"workflow_id,omitempty"`
	StepID      *uuid.UUID `json:"step_id,omitempty"`
	Task        Task      `json:"task" validate:"required"`
	Messages    []Message `json:"messages" validate:"required,min=1,dive"`
	MaxTokens   int       `json:"max_tokens,omitempty"`
	Temperature float64   `json:"temperature,omitempty"`
	// NoFallback forces a single-provider call (used by audit_agent).
	NoFallback bool `json:"no_fallback,omitempty"`
	// CacheBypass skips the prompt cache lookup/store.
	CacheBypass bool `json:"cache_bypass,omitempty"`
}

// ChatResponse is what callers receive.
type ChatResponse struct {
	Content          string  `json:"content"`
	Model            string  `json:"model"`
	Provider         string  `json:"provider"`
	PromptTokens     int     `json:"prompt_tokens"`
	CompletionTokens int     `json:"completion_tokens"`
	TotalTokens      int     `json:"total_tokens"`
	LatencyMs        int     `json:"latency_ms"`
	CostUSD          float64 `json:"cost_usd"`
	CacheHit         bool    `json:"cache_hit"`
	FallbackUsed     bool    `json:"fallback_used"`
	Attempt          int     `json:"attempt"`
}

// CallLog is a row destined for router_call_logs (before batching).
type CallLog struct {
	TenantID         uuid.UUID
	WorkflowID       *uuid.UUID
	StepID           *uuid.UUID
	Task             Task
	Provider         string
	Model            string
	PromptTokens     int
	CompletionTokens int
	LatencyMs        int
	CostUSD          float64
	CacheHit         bool
	FallbackUsed     bool
	Attempt          int
	Error            string
	Metadata         map[string]any
	CreatedAt        time.Time
}

// BudgetStatus is the result of a budget check.
type BudgetStatus struct {
	TenantID uuid.UUID
	Task     Task
	MonthlyCapUSD float64
	SpentUSD      float64
	RemainingUSD  float64
	Exhausted     bool
}