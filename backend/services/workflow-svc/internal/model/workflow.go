// Package model defines Workflow domain types per docs/architecture/state-machine.md.
package model

import (
	"time"

	"github.com/google/uuid"
)

// State is a workflow lifecycle state.
type State string

const (
	StatePending    State = "pending"
	StateParsing    State = "parsing"
	StateOutlining  State = "outlining"
	StateFacts      State = "facts"
	StateGenerating State = "generating"
	StateAuditing   State = "auditing"
	StateExporting  State = "exporting"
	StateDone       State = "done"
	StateFailed     State = "failed"
	StateCancelled  State = "cancelled"
	StatePaused     State = "paused"
)

// IsTerminal reports whether the state is a terminal (no outgoing transitions).
func (s State) IsTerminal() bool {
	return s == StateDone || s == StateCancelled
}

// StepName is a logical step in the workflow (Step02-07 in the framework).
type StepName string

const (
	StepParsing    StepName = "parsing"
	StepOutlining  StepName = "outlining"
	StepFacts      StepName = "facts"
	StepGenerating StepName = "generating"
	StepAuditing   StepName = "auditing"
	StepExporting  StepName = "exporting"
)

// StepStatus is the per-step lifecycle.
type StepStatus string

const (
	StepPending   StepStatus = "pending"
	StepRunning   StepStatus = "running"
	StepSucceeded StepStatus = "succeeded"
	StepFailed    StepStatus = "failed"
	StepSkipped   StepStatus = "skipped"
)

// Workflow is the aggregate root for one bid pipeline run.
type Workflow struct {
	ID          uuid.UUID  `json:"id"           db:"id"`
	TenantID    uuid.UUID  `json:"tenant_id"    db:"tenant_id"`
	ProjectID   uuid.UUID  `json:"project_id"   db:"project_id"`
	Status      State      `json:"status"       db:"status"`
	CurrentStep *StepName  `json:"current_step,omitempty" db:"current_step"`
	Error       *string    `json:"error,omitempty"      db:"error"`
	Metadata    []byte     `json:"metadata"     db:"metadata"`
	StartedAt   *time.Time `json:"started_at,omitempty" db:"started_at"`
	FinishedAt  *time.Time `json:"finished_at,omitempty" db:"finished_at"`
	CreatedBy   uuid.UUID  `json:"created_by"   db:"created_by"`
	Version     int        `json:"version"      db:"version"`
	CreatedAt   time.Time  `json:"created_at"   db:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"   db:"updated_at"`
}

// Step is one logical unit of work within a workflow.
type Step struct {
	ID         uuid.UUID  `json:"id"          db:"id"`
	WorkflowID uuid.UUID  `json:"workflow_id" db:"workflow_id"`
	TenantID   uuid.UUID  `json:"tenant_id"   db:"tenant_id"`
	Name       StepName   `json:"name"        db:"name"`
	Status     StepStatus `json:"status"      db:"status"`
	Progress   int        `json:"progress"    db:"progress"`
	StartedAt  *time.Time `json:"started_at,omitempty"  db:"started_at"`
	FinishedAt *time.Time `json:"finished_at,omitempty" db:"finished_at"`
	Error      *string    `json:"error,omitempty"        db:"error"`
	Artifacts  []byte     `json:"artifacts"   db:"artifacts"`
	CreatedAt  time.Time  `json:"created_at"  db:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"  db:"updated_at"`
}

// Event is an audit record of a state transition.
type Event struct {
	ID         int64     `json:"id"          db:"id"`
	WorkflowID uuid.UUID `json:"workflow_id" db:"workflow_id"`
	TenantID   uuid.UUID `json:"tenant_id"   db:"tenant_id"`
	FromState  *State    `json:"from_state,omitempty" db:"from_state"`
	ToState    State     `json:"to_state"    db:"to_state"`
	ActorID    uuid.UUID `json:"actor_id"    db:"actor_id"`
	Reason     *string   `json:"reason,omitempty" db:"reason"`
	Payload    []byte    `json:"payload"     db:"payload"`
	CreatedAt  time.Time `json:"created_at"  db:"created_at"`
}

// CreateRequest is the API payload.
type CreateRequest struct {
	ProjectID uuid.UUID `json:"project_id" validate:"required"`
	Metadata  []byte    `json:"metadata"   validate:"omitempty"`
}

// TransitionRequest triggers a state change.
type TransitionRequest struct {
	To     State  `json:"to"     validate:"required,oneof=parsing outlining facts generating auditing exporting done failed cancelled paused"`
	Reason string `json:"reason" validate:"omitempty,max=500"`
}

// PauseRequest is the API payload for POST /bids/{id}/pause.
type PauseRequest struct {
	Reason string `json:"reason" validate:"omitempty,max=500"`
}

// ResumeRequest is the API payload for POST /bids/{id}/resume. `To` is
// optional — when omitted, the workflow resumes to whatever state was
// recorded by the matching Pause call (i.e. metadata.paused_from).
type ResumeRequest struct {
	To     State  `json:"to"     validate:"omitempty,oneof=parsing outlining facts generating auditing exporting"`
	Reason string `json:"reason" validate:"omitempty,max=500"`
}

// StepUpdateRequest updates progress on a step.
type StepUpdateRequest struct {
	Status     *StepStatus `json:"status"     validate:"omitempty,oneof=pending running succeeded failed skipped"`
	Progress   *int        `json:"progress"   validate:"omitempty,between=0,100"`
	Error      *string     `json:"error"      validate:"omitempty,max=2000"`
	Artifacts  []byte      `json:"artifacts"  validate:"omitempty"`
}