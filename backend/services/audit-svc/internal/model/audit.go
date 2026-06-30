package model

import (
	"time"

	"github.com/google/uuid"
)

// Severity levels for audit issues.
type Severity string

const (
	SeverityCritical Severity = "critical"
	SeverityMajor    Severity = "major"
	SeverityMinor    Severity = "minor"
)

// Dimension categorizes the type of audit issue.
type Dimension string

const (
	DimensionCompliance   Dimension = "compliance"    // 合规性（废标项）
	DimensionConsistency  Dimension = "consistency"  // 一致性
	DimensionCompleteness Dimension = "completeness" // 完整性
	DimensionFormat       Dimension = "format"       // 格式规范
	DimensionAccuracy     Dimension = "accuracy"     // 准确性
)

// AuditReport is the overall audit result for a bid job.
type AuditReport struct {
	ID              uuid.UUID  `json:"id" db:"id"`
	BidJobID       uuid.UUID  `json:"bid_job_id" db:"bid_job_id"`
	TenantID       uuid.UUID  `json:"tenant_id" db:"tenant_id"`
	Status         string     `json:"status" db:"status"` // pending, running, done, failed
	Critical       int        `json:"critical" db:"critical"`
	Major          int        `json:"major" db:"major"`
	Minor          int        `json:"minor" db:"minor"`
	TotalIssues    int        `json:"total_issues" db:"total_issues"`
	Passed         bool       `json:"passed" db:"passed"`
	StartedAt      *time.Time `json:"started_at,omitempty" db:"started_at"`
	FinishedAt     *time.Time `json:"finished_at,omitempty" db:"finished_at"`
	CreatedAt      time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at" db:"updated_at"`
}

// AuditIssue is a single issue found during audit.
type AuditIssue struct {
	ID            uuid.UUID  `json:"id" db:"id"`
	ReportID      uuid.UUID  `json:"report_id" db:"report_id"`
	BidJobID      uuid.UUID  `json:"bid_job_id" db:"bid_job_id"`
	TenantID      uuid.UUID  `json:"tenant_id" db:"tenant_id"`
	ChapterID     *uuid.UUID `json:"chapter_id,omitempty" db:"chapter_id"`
	ChapterTitle  string     `json:"chapter_title" db:"chapter_title"`
	Severity      Severity   `json:"severity" db:"severity"`
	Dimension     Dimension  `json:"dimension" db:"dimension"`
	Issue         string     `json:"issue" db:"issue"`
	Suggestion    string     `json:"suggestion" db:"suggestion"`
	Evidence      string     `json:"evidence,omitempty" db:"evidence"`
	Status        string     `json:"status" db:"status"` // open, accepted, rejected, resolved
	ResolvedBy    *uuid.UUID `json:"resolved_by,omitempty" db:"resolved_by"`
	ResolvedAt    *time.Time `json:"resolved_at,omitempty" db:"resolved_at"`
	CreatedAt     time.Time  `json:"created_at" db:"created_at"`
}

// TriggerAuditRequest is the payload for triggering an audit.
type TriggerAuditRequest struct {
	Async bool `json:"async"`
}

// ResolveIssueRequest is the payload for resolving an audit issue.
type ResolveIssueRequest struct {
	IssueID  uuid.UUID `json:"issue_id" validate:"required"`
	Decision string     `json:"decision" validate:"required,oneof=accepted rejected resolved"`
	Note     string     `json:"note,omitempty"`
}

// AuditResult aggregates all issues found during an audit run.
type AuditResult struct {
	Report  *AuditReport  `json:"report"`
	Issues  []*AuditIssue `json:"issues"`
}
