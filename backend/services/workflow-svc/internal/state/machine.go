// Package state implements the bid workflow state machine.
// See docs/architecture/state-machine.md for the canonical diagram.
package state

import (
	"errors"

	"github.com/bidwriter/services/workflow-svc/internal/model"
)

// ErrInvalidTransition is returned when a state change is not allowed.
var ErrInvalidTransition = errors.New("invalid state transition")

// validTransitions is the canonical adjacency list.
// Any state not listed explicitly has no outgoing transitions (terminal).
var validTransitions = map[model.State][]model.State{
	model.StatePending:        {model.StateParsing, model.StateOutlining, model.StateFailed, model.StateCancelled},
	model.StateParsing:        {model.StateOutlining, model.StateFailed, model.StatePaused, model.StateCancelled},
	model.StateOutlining:      {model.StateFacts, model.StateFailed, model.StatePaused, model.StateCancelled},
	model.StateFacts:          {model.StateGenerating, model.StateFailed, model.StatePaused, model.StateCancelled},
	model.StateGenerating:     {model.StateAwaitingReview, model.StateFailed, model.StatePaused, model.StateCancelled},
	model.StateAwaitingReview: {model.StateAuditing, model.StateGenerating, model.StateFailed, model.StatePaused, model.StateCancelled},
	model.StateAuditing:       {model.StateExporting, model.StateFailed, model.StatePaused, model.StateCancelled},
	model.StateExporting:      {model.StateDone, model.StateFailed, model.StateCancelled},
	model.StatePaused: {
		model.StateParsing, model.StateOutlining, model.StateFacts,
		model.StateGenerating, model.StateAuditing, model.StateExporting,
		model.StateCancelled,
	},
	model.StateFailed:    {model.StateParsing, model.StateCancelled}, // can be resumed
	model.StateDone:      {},                                         // terminal
	model.StateCancelled: {},                                         // terminal
}

// CanTransition reports whether `from → to` is a permitted transition.
func CanTransition(from, to model.State) bool {
	for _, s := range validTransitions[from] {
		if s == to {
			return true
		}
	}
	return false
}

// Validate returns ErrInvalidTransition if the transition is not permitted.
func Validate(from, to model.State) error {
	if !CanTransition(from, to) {
		return ErrInvalidTransition
	}
	return nil
}

// LinearPlan returns the standard happy-path ordering of steps.
// Used to auto-create step records when a workflow is created.
func LinearPlan() []model.StepName {
	return []model.StepName{
		model.StepParsing,
		model.StepOutlining,
		model.StepFacts,
		model.StepGenerating,
		model.StepAuditing,
		model.StepExporting,
	}
}

// StepForState maps a workflow state to the corresponding step name.
func StepForState(s model.State) (model.StepName, bool) {
	switch s {
	case model.StateParsing:
		return model.StepParsing, true
	case model.StateOutlining:
		return model.StepOutlining, true
	case model.StateFacts:
		return model.StepFacts, true
	case model.StateGenerating:
		return model.StepGenerating, true
	case model.StateAuditing:
		return model.StepAuditing, true
	case model.StateExporting:
		return model.StepExporting, true
	}
	return "", false
}

// NextState returns the canonical next happy-path state after the given one.
// Returns false if there is no next state (terminal or unknown).
func NextState(s model.State) (model.State, bool) {
	switch s {
	case model.StatePending:
		return model.StateParsing, true
	case model.StateParsing:
		return model.StateOutlining, true
	case model.StateOutlining:
		return model.StateFacts, true
	case model.StateFacts:
		return model.StateGenerating, true
	case model.StateGenerating:
		return model.StateAwaitingReview, true
	case model.StateAwaitingReview:
		return model.StateAuditing, true
	case model.StateAuditing:
		return model.StateExporting, true
	case model.StateExporting:
		return model.StateDone, true
	}
	return "", false
}
