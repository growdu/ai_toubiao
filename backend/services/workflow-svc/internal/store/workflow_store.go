// Package store is the data access layer for workflows.
package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/bidwriter/services/workflow-svc/internal/model"
	"github.com/bidwriter/services/workflow-svc/internal/state"
	"github.com/bidwriter/shared/pkg/tenant"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrNotFound         = errors.New("workflow not found")
	ErrVersionConflict  = errors.New("version conflict")
	ErrActiveExists     = errors.New("an active workflow already exists for this project")
	ErrInvalidState     = errors.New("invalid state transition")
)

type Store struct {
	pool *pgxpool.Pool
}

func New(pool *pgxpool.Pool) *Store { return &Store{pool: pool} }

// Create creates a new workflow and seeds its step records (all pending).
func (s *Store) Create(ctx context.Context, req *model.CreateRequest, actorID uuid.UUID) (*model.Workflow, error) {
	tid, err := tenant.FromContext(ctx)
	if err != nil {
		return nil, err
	}
	tenantID := uuid.MustParse(tid)

	wf := &model.Workflow{
		ID:        uuid.New(),
		TenantID:  tenantID,
		ProjectID: req.ProjectID,
		Status:    model.StatePending,
		Metadata:  []byte(`{}`),
		CreatedBy: actorID,
		Version:   1,
	}
	if len(req.Metadata) > 0 {
		wf.Metadata = req.Metadata
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	const insertWF = `
		INSERT INTO workflows (id, tenant_id, project_id, status, metadata, created_by, version)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING created_at, updated_at`
	if err := tx.QueryRow(ctx, insertWF,
		wf.ID, wf.TenantID, wf.ProjectID, wf.Status, wf.Metadata, wf.CreatedBy, wf.Version,
	).Scan(&wf.CreatedAt, &wf.UpdatedAt); err != nil {
		// Check for active-exists constraint violation
		if isUniqueViolation(err, "uq_workflows_active_per_project") {
			return nil, ErrActiveExists
		}
		return nil, fmt.Errorf("insert workflow: %w", err)
	}

	// Seed step records
	for _, stepName := range state.LinearPlan() {
		_, err := tx.Exec(ctx, `
			INSERT INTO workflow_steps (workflow_id, tenant_id, name, status, progress)
			VALUES ($1, $2, $3, 'pending', 0)`,
			wf.ID, wf.TenantID, stepName)
		if err != nil {
			return nil, fmt.Errorf("seed step %s: %w", stepName, err)
		}
	}

	// Initial event log
	_, err = tx.Exec(ctx, `
		INSERT INTO workflow_events (workflow_id, tenant_id, from_state, to_state, actor_id, reason)
		VALUES ($1, $2, NULL, 'pending', $3, 'workflow created')`,
		wf.ID, wf.TenantID, actorID)
	if err != nil {
		return nil, fmt.Errorf("seed event: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return wf, nil
}

func (s *Store) Get(ctx context.Context, id uuid.UUID) (*model.Workflow, error) {
	tid, err := tenant.FromContext(ctx)
	if err != nil {
		return nil, err
	}
	const q = `
		SELECT id, tenant_id, project_id, status, current_step, error, metadata,
		       started_at, finished_at, created_by, version, created_at, updated_at
		FROM workflows
		WHERE id = $1 AND tenant_id = $2 AND deleted_at IS NULL`
	var w model.Workflow
	err = s.pool.QueryRow(ctx, q, id, tid).Scan(
		&w.ID, &w.TenantID, &w.ProjectID, &w.Status, &w.CurrentStep, &w.Error, &w.Metadata,
		&w.StartedAt, &w.FinishedAt, &w.CreatedBy, &w.Version, &w.CreatedAt, &w.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return &w, err
}

func (s *Store) List(ctx context.Context, projectID *uuid.UUID, status *model.State, limit int) ([]*model.Workflow, error) {
	tid, err := tenant.FromContext(ctx)
	if err != nil {
		return nil, err
	}
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	const q = `
		SELECT id, tenant_id, project_id, status, current_step, error, metadata,
		       started_at, finished_at, created_by, version, created_at, updated_at
		FROM workflows
		WHERE tenant_id = $1
		  AND ($2::uuid IS NULL OR project_id = $2)
		  AND ($3::text IS NULL OR status = $3)
		  AND deleted_at IS NULL
		ORDER BY created_at DESC
		LIMIT $4`
	rows, err := s.pool.Query(ctx, q, tid, projectID, status, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*model.Workflow
	for rows.Next() {
		var w model.Workflow
		if err := rows.Scan(
			&w.ID, &w.TenantID, &w.ProjectID, &w.Status, &w.CurrentStep, &w.Error, &w.Metadata,
			&w.StartedAt, &w.FinishedAt, &w.CreatedBy, &w.Version, &w.CreatedAt, &w.UpdatedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, &w)
	}
	return out, rows.Err()
}

// Transition applies a state change with optimistic locking and audit logging.
func (s *Store) Transition(ctx context.Context, id uuid.UUID, req *model.TransitionRequest, expectedVersion int, actorID uuid.UUID) (*model.Workflow, error) {
	tid, err := tenant.FromContext(ctx)
	if err != nil {
		return nil, err
	}
	tenantID := uuid.MustParse(tid)

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	var current model.Workflow
	err = tx.QueryRow(ctx, `
		SELECT id, tenant_id, project_id, status, version, created_by
		FROM workflows
		WHERE id = $1 AND tenant_id = $2 AND deleted_at IS NULL
		FOR UPDATE`, id, tenantID).Scan(
		&current.ID, &current.TenantID, &current.ProjectID, &current.Status, &current.Version, &current.CreatedBy,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if current.Version != expectedVersion {
		return nil, ErrVersionConflict
	}
	if err := state.Validate(current.Status, req.To); err != nil {
		return nil, ErrInvalidState
	}

	now := time.Now()
	stepName, _ := state.StepForState(req.To)
	var startedAt, finishedAt *time.Time
	switch req.To {
	case model.StatePending:
		// Reset: started_at cleared
		startedAt = nil
		finishedAt = nil
	case model.StateParsing:
		t := now
		startedAt = &t
	case model.StateDone, model.StateCancelled, model.StateFailed:
		t := now
		finishedAt = &t
	}

	const updateQ = `
		UPDATE workflows
		SET status = $1,
		    current_step = COALESCE($2, current_step),
		    error = CASE WHEN $1 = 'failed' THEN $3 ELSE NULL END,
		    started_at = COALESCE($4, started_at),
		    finished_at = $5,
		    version = version + 1,
		    updated_at = NOW()
		WHERE id = $6 AND tenant_id = $7
		RETURNING id, tenant_id, project_id, status, current_step, error, metadata,
		          started_at, finished_at, created_by, version, created_at, updated_at`
	var cnStep *string
	if stepName != "" {
		s := string(stepName)
		cnStep = &s
	}
	var updated model.Workflow
	err = tx.QueryRow(ctx, updateQ, req.To, cnStep, req.Reason, startedAt, finishedAt, id, tenantID).Scan(
		&updated.ID, &updated.TenantID, &updated.ProjectID, &updated.Status,
		&updated.CurrentStep, &updated.Error, &updated.Metadata,
		&updated.StartedAt, &updated.FinishedAt, &updated.CreatedBy,
		&updated.Version, &updated.CreatedAt, &updated.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	// Mark corresponding step as running/succeeded
	if stepName != "" {
		var stepStatus model.StepStatus
		switch req.To {
		case model.StateFailed:
			stepStatus = model.StepFailed
		case model.StateDone:
			stepStatus = model.StepSucceeded
		default:
			stepStatus = model.StepRunning
		}
		_, err = tx.Exec(ctx, `
			UPDATE workflow_steps
			SET status = $1,
			    progress = CASE WHEN $1 = 'succeeded' THEN 100 ELSE progress END,
			    started_at = COALESCE(started_at, NOW()),
			    finished_at = CASE WHEN $1 IN ('succeeded','failed') THEN NOW() ELSE finished_at END
			WHERE workflow_id = $2 AND name = $3`,
			stepStatus, id, stepName)
		if err != nil {
			return nil, err
		}
	}

	// Audit event
	_, err = tx.Exec(ctx, `
		INSERT INTO workflow_events (workflow_id, tenant_id, from_state, to_state, actor_id, reason)
		VALUES ($1, $2, $3, $4, $5, $6)`,
		id, tenantID, current.Status, req.To, actorID, req.Reason)
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return &updated, nil
}

func (s *Store) ListSteps(ctx context.Context, workflowID uuid.UUID) ([]*model.Step, error) {
	tid, err := tenant.FromContext(ctx)
	if err != nil {
		return nil, err
	}
	const q = `
		SELECT id, workflow_id, tenant_id, name, status, progress,
		       started_at, finished_at, error, artifacts, created_at, updated_at
		FROM workflow_steps
		WHERE workflow_id = $1 AND tenant_id = $2
		ORDER BY id`
	rows, err := s.pool.Query(ctx, q, workflowID, tid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*model.Step
	for rows.Next() {
		var st model.Step
		if err := rows.Scan(
			&st.ID, &st.WorkflowID, &st.TenantID, &st.Name, &st.Status, &st.Progress,
			&st.StartedAt, &st.FinishedAt, &st.Error, &st.Artifacts, &st.CreatedAt, &st.UpdatedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, &st)
	}
	return out, rows.Err()
}

func (s *Store) ListEvents(ctx context.Context, workflowID uuid.UUID, limit int) ([]*model.Event, error) {
	tid, err := tenant.FromContext(ctx)
	if err != nil {
		return nil, err
	}
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	const q = `
		SELECT id, workflow_id, tenant_id, from_state, to_state, actor_id, reason, payload, created_at
		FROM workflow_events
		WHERE workflow_id = $1 AND tenant_id = $2
		ORDER BY id DESC
		LIMIT $3`
	rows, err := s.pool.Query(ctx, q, workflowID, tid, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*model.Event
	for rows.Next() {
		var e model.Event
		if err := rows.Scan(
			&e.ID, &e.WorkflowID, &e.TenantID, &e.FromState, &e.ToState,
			&e.ActorID, &e.Reason, &e.Payload, &e.CreatedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, &e)
	}
	return out, rows.Err()
}

// isUniqueViolation checks if err is a Postgres unique-constraint violation
// on the named index. Used to translate DB errors to domain errors.
func isUniqueViolation(err error, indexName string) bool {
	if err == nil {
		return false
	}
	// pgx error format: "ERROR: duplicate key value violates unique constraint \"<name>\""
	msg := err.Error()
	return contains(msg, "duplicate key value violates unique constraint") &&
		contains(msg, indexName)
}

func contains(s, substr string) bool {
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}