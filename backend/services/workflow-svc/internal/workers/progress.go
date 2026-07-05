package workers

import (
	"context"
	"errors"
	"log/slog"

	"github.com/bidwriter/services/workflow-svc/internal/model"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ChapterProgressStore is the read-side contract ProgressWatcher needs
// from the database. The concrete *pgxpool.Pool + Store pair satisfy it via
// PGProgressStore; tests use a hand-rolled fake.
type ChapterProgressStore interface {
	// CountChapters returns the (total, succeeded, failedTerminal, blockedFailed)
	// tally for a bid job.
	//   total          = chapter_specs row count for bid_job_id
	//   succeeded      = status IN ('succeeded')  + user-edited 'human' content rows
	//                    (we use chapter_specs.status to count, see caller)
	//   failedTerminal = status = 'failed' AND retry_count >= MaxRetries
	//   blockedFailed  = status = 'failed' AND retry_count <  MaxRetries
	CountChapters(ctx context.Context, bidJobID uuid.UUID, maxRetries int) (total, succeeded, failedTerminal, blockedFailed int, err error)

	// FindWorkflowForBid returns the workflow id and current version for the
	// bid job. Returns uuid.Nil and an error if not found.
	FindWorkflowForBid(ctx context.Context, bidJobID uuid.UUID) (workflowID uuid.UUID, version int, err error)
}

// WorkflowTransitioner is the write-side contract ProgressWatcher needs.
// The watcher does its own conditional UPDATE (`status=fromState AND
// version=expectedVersion`) instead of going through store.Store.Transition,
// because in async auto-advance we don't have a fresh expected-version from
// the user. Tests use a hand-rolled fake.
type WorkflowTransitioner interface {
	Transition(ctx context.Context, workflowID uuid.UUID, fromState, toState model.State, expectedVersion int, actorID uuid.UUID) error
}

// PGTransitioner implements WorkflowTransitioner against a pgx pool. It
// does an atomic CAS UPDATE so concurrent manual transitions and watcher
// transitions can't both succeed.
type PGTransitioner struct {
	Pool *pgxpool.Pool
}

// Transition updates the workflow row from `fromState` to `toState` iff the
// row is currently in `fromState` AND version matches. Returns
// ErrTransitionConflict if either guard fails. Records a workflow_events
// row on success.
func (p *PGTransitioner) Transition(ctx context.Context, workflowID uuid.UUID, fromState, toState model.State, expectedVersion int, actorID uuid.UUID) error {
	res, err := p.Pool.Exec(ctx, `
		UPDATE workflows
		SET status = $2, version = version + 1,
		    current_step = $3, updated_at = NOW()
		WHERE id = $1 AND status = $4 AND version = $5`,
		workflowID, toState, string(toState), fromState, expectedVersion)
	if err != nil {
		return err
	}
	if res.RowsAffected() == 0 {
		return ErrTransitionConflict
	}
	_, _ = p.Pool.Exec(ctx, `
		INSERT INTO workflow_events (workflow_id, tenant_id, from_state, to_state, actor_id, reason)
		VALUES ($1, (SELECT tenant_id FROM workflows WHERE id = $1), $2, $3, $4, $5)`,
		workflowID, fromState, toState, actorID, "auto-advance: "+string(fromState)+" -> "+string(toState))
	return nil
}

// ErrTransitionConflict is returned when the CAS UPDATE found no row in the
// expected state/version (e.g. a manual transition raced us).
var ErrTransitionConflict = errors.New("workflow transition conflict: state or version mismatch")

// Watcher is what chapter.go calls after persisting a chapter's content.
// It checks whether the bid job has finished its chapter generation phase
// and, if so, transitions the workflow from generating to auditing.
//
// Decisions:
//   - Stay:               not all chapters are terminal yet; do nothing
//   - BlockedFailed:      at least one chapter is in 'failed' status but still
//                         has retries left; wait for Asynq to retry it
//   - AdvanceAllSucceeded: every chapter is succeeded OR failed-terminal AND
//                          no blocked-failed → safe to advance
type Watcher struct {
	Store        ChapterProgressStore
	Transitioner WorkflowTransitioner
	Log          *slog.Logger
	MaxRetries   int // per chapter, default 2
	ActorID      uuid.UUID
}

// Decision enumerates what ProgressWatcher.CheckAndAdvance wants to do.
type Decision struct {
	Kind            string // "Stay" | "Advance" | "BlockedFailed"
	SucceededCount  int
	TotalCount      int
	BlockedFailures int
	TerminalFailures int
}

// Check inspects the chapter state for a bid job and returns the decision.
// It does NOT mutate state on its own — callers translate the decision into
// transitions. This keeps the watcher pure and trivially unit-testable.
func (w *Watcher) Check(ctx context.Context, bidJobID uuid.UUID) (Decision, error) {
	maxRetries := w.MaxRetries
	if maxRetries == 0 {
		maxRetries = 2
	}
	total, succ, termF, blockedF, err := w.Store.CountChapters(ctx, bidJobID, maxRetries)
	if err != nil {
		return Decision{}, err
	}
	d := Decision{
		SucceededCount:   succ,
		TotalCount:       total,
		BlockedFailures:  blockedF,
		TerminalFailures: termF,
	}
	switch {
	case total == 0:
		// No spec rows yet (e.g. user just opened a blank bid). Wait.
		d.Kind = "Stay"
	case blockedF > 0:
		// Some failures still have retries left; let Asynq retry them.
		d.Kind = "BlockedFailed"
	case succ+termF == total:
		// Every spec is in a terminal state and nothing is blocked.
		d.Kind = "Advance"
	default:
		// Some specs haven't finished; wait for them.
		d.Kind = "Stay"
	}
	return d, nil
}

// CheckAndAdvance runs Check and, when the decision is Advance, transitions
// the workflow from generating -> auditing. transition failures are logged
// and returned (the caller can decide whether to swallow).
func (w *Watcher) CheckAndAdvance(ctx context.Context, bidJobID uuid.UUID) error {
	d, err := w.Check(ctx, bidJobID)
	if err != nil {
		return err
	}
	if d.Kind != "Advance" {
		if w.Log != nil && d.Kind == "BlockedFailed" {
			w.Log.Info("progress: blocked by failures with retries left",
				slog.String("bid_job_id", bidJobID.String()),
				slog.Int("blocked", d.BlockedFailures),
				slog.Int("total", d.TotalCount))
		}
		return nil
	}

	wfID, version, err := w.Store.FindWorkflowForBid(ctx, bidJobID)
	if err != nil {
		return err
	}
	if wfID == uuid.Nil {
		return nil // no workflow yet, e.g. auto-advance safe no-op
	}
	if w.Transitioner == nil {
		return nil
	}
	if err := w.Transitioner.Transition(ctx, wfID,
		model.StateGenerating, model.StateAuditing,
		version, w.ActorID); err != nil {
		if w.Log != nil {
			w.Log.Warn("progress: transition to auditing failed",
				slog.String("workflow_id", wfID.String()),
				slog.String("bid_job_id", bidJobID.String()),
				slog.Any("err", err))
		}
		return err
	}
	if w.Log != nil {
		w.Log.Info("progress: advanced workflow to auditing",
			slog.String("workflow_id", wfID.String()),
			slog.String("bid_job_id", bidJobID.String()),
			slog.Int("chapters", d.TotalCount),
			slog.Int("succeeded", d.SucceededCount))
	}
	return nil
}

// ============================================================================
// PG-backed implementation (wired in cmd/workflow-svc/main.go)
// ============================================================================

// PGProgressStore implements ChapterProgressStore against a pgx pool. The
// queries deliberately scan chapter_specs.status so we don't depend on the
// optimistic-locked counter on bid_jobs.done_chapters (which can drift under
// concurrent worker writes).
type PGProgressStore struct {
	Pool *pgxpool.Pool
}

// CountChapters classifies chapter_specs for the bid job and returns the
// (total, succeeded, failedTerminal, blockedFailed) tally.
func (p *PGProgressStore) CountChapters(ctx context.Context, bidJobID uuid.UUID, maxRetries int) (int, int, int, int, error) {
	var total, succ, termF, blockedF int
	err := p.Pool.QueryRow(ctx, `
		SELECT
			COUNT(*)::int,
			COUNT(*) FILTER (WHERE status = 'succeeded')::int,
			COUNT(*) FILTER (WHERE status = 'failed' AND retry_count >= $2)::int,
			COUNT(*) FILTER (WHERE status = 'failed' AND retry_count <  $2)::int
		FROM chapter_specs WHERE bid_job_id = $1`,
		bidJobID, maxRetries,
	).Scan(&total, &succ, &termF, &blockedF)
	if err != nil {
		return 0, 0, 0, 0, err
	}
	return total, succ, termF, blockedF, nil
}

// FindWorkflowForBid returns the workflow id + current version for the bid job.
func (p *PGProgressStore) FindWorkflowForBid(ctx context.Context, bidJobID uuid.UUID) (uuid.UUID, int, error) {
	var wfID uuid.UUID
	var version int
	err := p.Pool.QueryRow(ctx, `
		SELECT workflow_id, version
		FROM bid_jobs WHERE id = $1`,
		bidJobID,
	).Scan(&wfID, &version)
	if err != nil {
		return uuid.Nil, 0, err
	}
	return wfID, version, nil
}
