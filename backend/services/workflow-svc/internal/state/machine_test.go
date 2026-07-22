package state

import (
	"errors"
	"testing"

	"github.com/bidwriter/services/workflow-svc/internal/model"
)

func TestLinearPlan(t *testing.T) {
	got := LinearPlan()
	want := []model.StepName{
		model.StepParsing, model.StepOutlining, model.StepFacts,
		model.StepGenerating, model.StepAuditing, model.StepExporting,
	}
	if len(got) != len(want) {
		t.Fatalf("len(LinearPlan)=%d, want %d", len(got), len(want))
	}
	for i, s := range want {
		if got[i] != s {
			t.Errorf("LinearPlan[%d]=%s, want %s", i, got[i], s)
		}
	}
}

func TestCanTransition_HappyPath(t *testing.T) {
	chain := []model.State{
		model.StatePending, model.StateParsing, model.StateOutlining,
		model.StateFacts, model.StateGenerating, model.StateAwaitingReview,
		model.StateAuditing,
		model.StateExporting, model.StateDone,
	}
	for i := 0; i < len(chain)-1; i++ {
		if !CanTransition(chain[i], chain[i+1]) {
			t.Errorf("expected %s -> %s allowed", chain[i], chain[i+1])
		}
	}
}

func TestCanTransition_TerminalStates(t *testing.T) {
	for _, s := range []model.State{model.StateDone, model.StateCancelled} {
		if CanTransition(s, model.StateParsing) {
			t.Errorf("terminal state %s should not allow any outgoing transition", s)
		}
	}
}

func TestCanTransition_FailureFanout(t *testing.T) {
	// Active states (the happy-path forward states) should each be allowed
	// to transition to failed. Paused and failed themselves are recovery
	// states with restricted edges, so they're excluded.
	canFail := map[model.State]bool{
		model.StatePending: true, model.StateParsing: true,
		model.StateOutlining: true, model.StateFacts: true,
		model.StateGenerating: true, model.StateAuditing: true,
		model.StateExporting: true,
	}
	for from, tos := range validTransitions {
		if !canFail[from] {
			continue
		}
		hasFailed := false
		for _, to := range tos {
			if to == model.StateFailed {
				hasFailed = true
				break
			}
		}
		if !hasFailed {
			t.Errorf("state %s should be allowed to fail", from)
		}
	}
}

func TestCanTransition_RejectBackwards(t *testing.T) {
	// pending -> done is not a valid one-step jump.
	if CanTransition(model.StatePending, model.StateDone) {
		t.Error("pending -> done should not be allowed")
	}
	// done -> anything is rejected.
	if CanTransition(model.StateDone, model.StateParsing) {
		t.Error("done -> parsing should not be allowed")
	}
}

func TestValidate_AllowedReturnsNil(t *testing.T) {
	if err := Validate(model.StatePending, model.StateParsing); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
}

func TestValidate_DisallowedReturnsErr(t *testing.T) {
	err := Validate(model.StatePending, model.StateDone)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, ErrInvalidTransition) {
		t.Errorf("expected ErrInvalidTransition, got %v", err)
	}
}

func TestStepForState(t *testing.T) {
	cases := []struct {
		s    model.State
		want model.StepName
		ok   bool
	}{
		{model.StateParsing, model.StepParsing, true},
		{model.StateOutlining, model.StepOutlining, true},
		{model.StateFacts, model.StepFacts, true},
		{model.StateGenerating, model.StepGenerating, true},
		{model.StateAuditing, model.StepAuditing, true},
		{model.StateExporting, model.StepExporting, true},
		{model.StatePending, "", false},
		{model.StateDone, "", false},
		{model.StateFailed, "", false},
	}
	for _, c := range cases {
		got, ok := StepForState(c.s)
		if ok != c.ok || got != c.want {
			t.Errorf("StepForState(%s) = (%s, %v), want (%s, %v)", c.s, got, ok, c.want, c.ok)
		}
	}
}

func TestNextState(t *testing.T) {
	chain := []model.State{
		model.StatePending, model.StateParsing, model.StateOutlining,
		model.StateFacts, model.StateGenerating, model.StateAwaitingReview,
		model.StateAuditing,
		model.StateExporting, model.StateDone,
	}
	for i := 0; i < len(chain)-1; i++ {
		got, ok := NextState(chain[i])
		if !ok || got != chain[i+1] {
			t.Errorf("NextState(%s) = (%s, %v), want (%s, true)", chain[i], got, ok, chain[i+1])
		}
	}
	if _, ok := NextState(model.StateFailed); ok {
		t.Error("Failed should have no next state")
	}
	if _, ok := NextState(model.State("unknown")); ok {
		t.Error("unknown state should have no next state")
	}
}
