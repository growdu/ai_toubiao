package state

import (
	"testing"

	"github.com/bidwriter/services/workflow-svc/internal/model"
)

func TestCanTransition_HappyPath(t *testing.T) {
	happy := []struct{ from, to model.State }{
		{model.StatePending, model.StateParsing},
		{model.StateParsing, model.StateOutlining},
		{model.StateOutlining, model.StateFacts},
		{model.StateFacts, model.StateGenerating},
		{model.StateGenerating, model.StateAuditing},
		{model.StateAuditing, model.StateExporting},
		{model.StateExporting, model.StateDone},
	}
	for _, c := range happy {
		if !CanTransition(c.from, c.to) {
			t.Errorf("expected %s -> %s to be allowed", c.from, c.to)
		}
	}
}

func TestCanTransition_CancellationFromAnywhere(t *testing.T) {
	states := []model.State{
		model.StatePending, model.StateParsing, model.StateOutlining,
		model.StateFacts, model.StateGenerating, model.StateAuditing,
		model.StateExporting, model.StatePaused,
	}
	for _, s := range states {
		if !CanTransition(s, model.StateCancelled) {
			t.Errorf("expected %s -> cancelled to be allowed", s)
		}
	}
}

func TestCanTransition_FailureFromAnywhere(t *testing.T) {
	// any non-terminal state can fail
	for _, s := range []model.State{
		model.StatePending, model.StateParsing, model.StateOutlining,
		model.StateFacts, model.StateGenerating, model.StateAuditing,
		model.StateExporting,
	} {
		if !CanTransition(s, model.StateFailed) {
			t.Errorf("expected %s -> failed to be allowed", s)
		}
	}
}

func TestCanTransition_TerminalNoOutgoing(t *testing.T) {
	// Done and Cancelled have no outgoing transitions
	for _, from := range []model.State{model.StateDone, model.StateCancelled} {
		for _, to := range []model.State{
			model.StatePending, model.StateParsing, model.StateOutlining,
			model.StateFacts, model.StateGenerating, model.StateAuditing,
			model.StateExporting, model.StateDone, model.StateFailed,
			model.StateCancelled, model.StatePaused,
		} {
			if CanTransition(from, to) {
				t.Errorf("terminal state %s should NOT transition to %s", from, to)
			}
		}
	}
}

func TestCanTransition_FailedCanResume(t *testing.T) {
	if !CanTransition(model.StateFailed, model.StateParsing) {
		t.Error("failed -> parsing should be allowed (resume)")
	}
}

func TestCanTransition_InvalidSkip(t *testing.T) {
	// Cannot skip steps in the happy path
	if CanTransition(model.StatePending, model.StateGenerating) {
		t.Error("pending -> generating should NOT be allowed (must go through parsing, outlining, facts)")
	}
	if CanTransition(model.StateParsing, model.StateDone) {
		t.Error("parsing -> done should NOT be allowed")
	}
}

func TestCanTransition_Backwards(t *testing.T) {
	// Cannot go backwards in the happy path (except via pause)
	if CanTransition(model.StateGenerating, model.StateFacts) {
		t.Error("generating -> facts should NOT be allowed (no backwards)")
	}
}

func TestValidate(t *testing.T) {
	if err := Validate(model.StatePending, model.StateParsing); err != nil {
		t.Errorf("Validate(pending->parsing): %v", err)
	}
	if err := Validate(model.StatePending, model.StateDone); err == nil {
		t.Error("Validate(pending->done) should fail")
	}
}

func TestIsTerminal(t *testing.T) {
	terminal := []model.State{model.StateDone, model.StateCancelled}
	for _, s := range terminal {
		if !s.IsTerminal() {
			t.Errorf("%s should be terminal", s)
		}
	}
	nonTerminal := []model.State{
		model.StatePending, model.StateParsing, model.StateOutlining,
		model.StateFacts, model.StateGenerating, model.StateAuditing,
		model.StateExporting, model.StateFailed, model.StatePaused,
	}
	for _, s := range nonTerminal {
		if s.IsTerminal() {
			t.Errorf("%s should NOT be terminal", s)
		}
	}
}

func TestNextState(t *testing.T) {
	cases := []struct {
		from   model.State
		want   model.State
		wantOK bool
	}{
		{model.StatePending, model.StateParsing, true},
		{model.StateParsing, model.StateOutlining, true},
		{model.StateExporting, model.StateDone, true},
		{model.StateDone, "", false},
		{model.StateFailed, "", false},
	}
	for _, c := range cases {
		got, ok := NextState(c.from)
		if ok != c.wantOK || got != c.want {
			t.Errorf("NextState(%s) = (%s, %v), want (%s, %v)", c.from, got, ok, c.want, c.wantOK)
		}
	}
}

func TestLinearPlan(t *testing.T) {
	plan := LinearPlan()
	if len(plan) != 6 {
		t.Fatalf("LinearPlan should have 6 steps, got %d", len(plan))
	}
	expected := []model.StepName{
		model.StepParsing, model.StepOutlining, model.StepFacts,
		model.StepGenerating, model.StepAuditing, model.StepExporting,
	}
	for i, s := range plan {
		if s != expected[i] {
			t.Errorf("step[%d] = %s, want %s", i, s, expected[i])
		}
	}
}