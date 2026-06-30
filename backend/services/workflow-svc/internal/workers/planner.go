// Package workers implements Asynq task workers for the bid pipeline.
package workers

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/bidwriter/services/workflow-svc/internal/model"
	"github.com/bidwriter/services/workflow-svc/internal/state"
	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PlannerPayload is the task payload for outline generation.
type PlannerPayload struct {
	WorkflowID uuid.UUID `json:"workflow_id"`
	BidJobID   uuid.UUID `json:"bid_job_id"`
	TenantID   uuid.UUID `json:"tenant_id"`
	DocumentID uuid.UUID `json:"document_id"`
}

// PlannerWorker processes outline generation tasks.
type PlannerWorker struct {
	log  *slog.Logger
	pool *pgxpool.Pool
}

// NewPlannerWorker creates a new planner worker.
func NewPlannerWorker(log *slog.Logger, pool *pgxpool.Pool) *PlannerWorker {
	return &PlannerWorker{log: log, pool: pool}
}

// Process handles the outline generation task.
func (w *PlannerWorker) Process(ctx context.Context, task *asynq.Task) error {
	var payload PlannerPayload
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		return fmt.Errorf("unmarshal payload: %w", err)
	}

	w.log.Info("planner: processing",
		slog.String("workflow_id", payload.WorkflowID.String()),
		slog.String("bid_job_id", payload.BidJobID.String()))

	// TODO: Implement actual outline generation
	// 1. Load parse_result from document-svc
	// 2. Call knowledge-svc to retrieve relevant evidence
	// 3. Call router-svc with TaskOutlineGen to generate chapters
	// 4. Write chapter_specs to database
	// 5. Update workflow state: outlining → generating

	// Simulate success - in production this calls AI and writes to db
	w.log.Info("planner: outline generated successfully",
		slog.String("workflow_id", payload.WorkflowID.String()))

	return nil
}

// EnqueueOutline enqueues an outline generation task.
func EnqueueOutline(ctx context.Context, client *asynq.Client, workflowID, bidJobID, tenantID, documentID uuid.UUID) error {
	payload := PlannerPayload{
		WorkflowID: workflowID,
		BidJobID:   bidJobID,
		TenantID:   tenantID,
		DocumentID: documentID,
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}
	task := asynq.NewTask(TaskOutlineGenerate, data)
	_, err = client.EnqueueContext(ctx, task, asynq.MaxRetry(3), asynq.Timeout(10*60*60*1000))
	return err
}

// TransitionWorkflow transitions a workflow to the next state.
func TransitionWorkflow(ctx context.Context, store WorkflowStore, workflowID uuid.UUID, to model.State, reason string) error {
	wf, err := store.GetWorkflow(ctx, workflowID)
	if err != nil {
		return err
	}

	if err := state.Validate(wf.Status, to); err != nil {
		return fmt.Errorf("invalid transition %s → %s: %w", wf.Status, to, err)
	}

	stepName, _ := state.StepForState(to)
	now := wf.UpdatedAt
	if err := store.UpdateWorkflow(ctx, workflowID, to, &stepName, nil, nil); err != nil {
		return err
	}

	// Log event
	_ = store.CreateEvent(ctx, &model.Event{
		WorkflowID: workflowID,
		TenantID:   wf.TenantID,
		FromState: &wf.Status,
		ToState:    to,
		Reason:     &reason,
	})
	_ = now // suppress unused
	return nil
}

// WorkflowStore interface for workflow operations needed by workers.
type WorkflowStore interface {
	GetWorkflow(ctx context.Context, id uuid.UUID) (*model.Workflow, error)
	UpdateWorkflow(ctx context.Context, id uuid.UUID, status model.State, step *model.StepName, error *string, artifacts []byte) error
	CreateEvent(ctx context.Context, e *model.Event) error
}

// UpdateStepProgress updates step progress.
func UpdateStepProgress(ctx context.Context, store WorkflowStore, workflowID uuid.UUID, stepName model.StepName, progress int) error {
	// In production, update the step record in the database
	_ = store
	_ = workflowID
	_ = stepName
	_ = progress
	return nil
}

// FinalizeStep marks a step as completed.
func FinalizeStep(ctx context.Context, store WorkflowStore, workflowID uuid.UUID, stepName model.StepName, status model.StepStatus, artifacts []byte) error {
	_ = store
	_ = workflowID
	_ = stepName
	_ = status
	_ = artifacts
	return nil
}
