package workers

import (
	"context"
	"errors"
	"testing"

	"github.com/bidwriter/services/workflow-svc/internal/model"
	"github.com/google/uuid"
)

// ============================================================================
// fakes
// ============================================================================

type fakeProgressStore struct {
	total, succ, termF, blockedF int
	wfID                         uuid.UUID
	wfVersion                    int
	countErr                     error
	findErr                      error
}

func (f *fakeProgressStore) CountChapters(ctx context.Context, bidJobID uuid.UUID, maxRetries int) (int, int, int, int, error) {
	if f.countErr != nil {
		return 0, 0, 0, 0, f.countErr
	}
	return f.total, f.succ, f.termF, f.blockedF, nil
}

func (f *fakeProgressStore) FindWorkflowForBid(ctx context.Context, bidJobID uuid.UUID) (uuid.UUID, int, error) {
	if f.findErr != nil {
		return uuid.Nil, 0, f.findErr
	}
	return f.wfID, f.wfVersion, nil
}

type fakeTransitioner struct {
	called       bool
	gotFrom      model.State
	gotTo        model.State
	gotVersion   int
	gotActor     uuid.UUID
	returnsErr   error
	transitionID uuid.UUID
}

func (f *fakeTransitioner) Transition(ctx context.Context, workflowID uuid.UUID, fromState, toState model.State, expectedVersion int, actorID uuid.UUID) error {
	f.called = true
	f.transitionID = workflowID
	f.gotFrom = fromState
	f.gotTo = toState
	f.gotVersion = expectedVersion
	f.gotActor = actorID
	return f.returnsErr
}

// silentLogger is defined in auditor_test.go and shared across all
// workers test files (Go test compilation unit covers the whole package).

// ============================================================================
// Check — pure decisions, no I/O side-effects
// ============================================================================

func TestProgressWatcher_Check_NoChaptersReturnsStay(t *testing.T) {
	w := &Watcher{Store: &fakeProgressStore{total: 0, succ: 0}}
	d, err := w.Check(context.Background(), uuid.New())
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if d.Kind != "Stay" {
		t.Errorf("expected Stay, got %q", d.Kind)
	}
}

func TestProgressWatcher_Check_PartialCompletionReturnsStay(t *testing.T) {
	// 3 succeeded, 1 still pending → Stay.
	w := &Watcher{Store: &fakeProgressStore{total: 4, succ: 3}}
	d, err := w.Check(context.Background(), uuid.New())
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if d.Kind != "Stay" {
		t.Errorf("expected Stay on partial, got %q", d.Kind)
	}
	if d.TotalCount != 4 || d.SucceededCount != 3 {
		t.Errorf("tally mismatch: %+v", d)
	}
}

func TestProgressWatcher_Check_BlockedByRetryingFailure(t *testing.T) {
	// 5 succeeded, 2 terminal-failed, 1 still retrying → BlockedFailed.
	w := &Watcher{
		Store:      &fakeProgressStore{total: 8, succ: 5, termF: 2, blockedF: 1},
		MaxRetries: 2,
	}
	d, err := w.Check(context.Background(), uuid.New())
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if d.Kind != "BlockedFailed" {
		t.Errorf("expected BlockedFailed, got %q", d.Kind)
	}
	if d.BlockedFailures != 1 || d.TerminalFailures != 2 {
		t.Errorf("tally mismatch: %+v", d)
	}
}

func TestProgressWatcher_Check_AllSucceededAdvances(t *testing.T) {
	w := &Watcher{Store: &fakeProgressStore{total: 6, succ: 6}}
	d, err := w.Check(context.Background(), uuid.New())
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if d.Kind != "Advance" {
		t.Errorf("expected Advance on all-succeeded, got %q", d.Kind)
	}
}

func TestProgressWatcher_Check_TerminalFailureCountsAsDone(t *testing.T) {
	// 4 succeeded + 2 failed-terminal (no retries left) → still Advance.
	w := &Watcher{
		Store:      &fakeProgressStore{total: 6, succ: 4, termF: 2},
		MaxRetries: 2,
	}
	d, err := w.Check(context.Background(), uuid.New())
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if d.Kind != "Advance" {
		t.Errorf("expected Advance with terminal-failed, got %q", d.Kind)
	}
}

func TestProgressWatcher_Check_DefaultsMaxRetriesWhenZero(t *testing.T) {
	// When MaxRetries=0 (uninitialised), the store still gets the default
	// passed (2). We don't check the exact value here, just that it doesn't
	// panic and returns sensibly.
	store := &fakeProgressStore{total: 3, succ: 3}
	w := &Watcher{Store: store, MaxRetries: 0}
	if _, err := w.Check(context.Background(), uuid.New()); err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
}

// ============================================================================
// CheckAndAdvance — interaction with the transitioner
// ============================================================================

func TestProgressWatcher_CheckAndAdvance_AdvancesOnAllDone(t *testing.T) {
	wfID := uuid.New()
	actor := uuid.New()
	store := &fakeProgressStore{
		total: 3, succ: 3,
		wfID: wfID, wfVersion: 7,
	}
	tr := &fakeTransitioner{returnsErr: nil}

	w := &Watcher{
		Store:        store,
		Transitioner: tr,
		Log:          silentLogger(),
		ActorID:      actor,
	}
	if err := w.CheckAndAdvance(context.Background(), uuid.New()); err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if !tr.called {
		t.Fatal("transitioner should have been invoked")
	}
	if tr.gotFrom != model.StateGenerating || tr.gotTo != model.StateAuditing {
		t.Errorf("expected generating->auditing, got %s->%s", tr.gotFrom, tr.gotTo)
	}
	if tr.gotVersion != 7 || tr.gotActor != actor {
		t.Errorf("version/actor mismatch: v=%d actor=%s", tr.gotVersion, tr.gotActor)
	}
	if tr.transitionID != wfID {
		t.Errorf("wrong workflow id: %s", tr.transitionID)
	}
}

func TestProgressWatcher_CheckAndAdvance_NoOpOnStay(t *testing.T) {
	tr := &fakeTransitioner{}
	store := &fakeProgressStore{total: 5, succ: 2} // partial → Stay
	w := &Watcher{Store: store, Transitioner: tr, Log: silentLogger(), ActorID: uuid.New()}
	if err := w.CheckAndAdvance(context.Background(), uuid.New()); err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if tr.called {
		t.Error("transitioner should not have been called on Stay")
	}
}

func TestProgressWatcher_CheckAndAdvance_NoOpOnBlockedFailure(t *testing.T) {
	tr := &fakeTransitioner{}
	store := &fakeProgressStore{total: 6, succ: 4, termF: 1, blockedF: 1}
	w := &Watcher{
		Store: store, Transitioner: tr, Log: silentLogger(),
		ActorID: uuid.New(), MaxRetries: 2,
	}
	if err := w.CheckAndAdvance(context.Background(), uuid.New()); err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if tr.called {
		t.Error("transitioner should not be called when retries still pending")
	}
}

func TestProgressWatcher_CheckAndAdvance_TransitionConflictPropagates(t *testing.T) {
	store := &fakeProgressStore{total: 2, succ: 2, wfID: uuid.New(), wfVersion: 3}
	tr := &fakeTransitioner{returnsErr: ErrTransitionConflict}
	w := &Watcher{Store: store, Transitioner: tr, Log: silentLogger(), ActorID: uuid.New()}
	err := w.CheckAndAdvance(context.Background(), uuid.New())
	if err == nil || !errors.Is(err, ErrTransitionConflict) {
		t.Fatalf("expected conflict err, got %v", err)
	}
}

func TestProgressWatcher_CheckAndAdvance_BidJobWithoutWorkflow(t *testing.T) {
	// If FindWorkflowForBid returns uuid.Nil (orphan bid job, e.g. test
	// fixture), watcher must not panic and must not call the transitioner.
	store := &fakeProgressStore{
		total: 4, succ: 4, wfID: uuid.Nil,
	}
	tr := &fakeTransitioner{}
	w := &Watcher{Store: store, Transitioner: tr, Log: silentLogger(), ActorID: uuid.New()}
	if err := w.CheckAndAdvance(context.Background(), uuid.New()); err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if tr.called {
		t.Error("transitioner should not be called without a workflow")
	}
}

func TestProgressWatcher_CheckAndAdvance_CountErrorPropagates(t *testing.T) {
	store := &fakeProgressStore{countErr: errors.New("db down")}
	w := &Watcher{Store: store, Log: silentLogger(), ActorID: uuid.New()}
	if err := w.CheckAndAdvance(context.Background(), uuid.New()); err == nil {
		t.Fatal("expected err to propagate")
	}
}

func TestProgressWatcher_CheckAndAdvance_NilTransitionerIsNoOp(t *testing.T) {
	// Auto-advance only happens when both Store and Transitioner are wired.
	// Nil transitioner should not panic.
	store := &fakeProgressStore{total: 1, succ: 1, wfID: uuid.New(), wfVersion: 1}
	w := &Watcher{Store: store, Transitioner: nil, Log: silentLogger(), ActorID: uuid.New()}
	if err := w.CheckAndAdvance(context.Background(), uuid.New()); err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
}
