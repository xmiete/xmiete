package statemachine

import (
	"fmt"

	"github.com/xmiete/server/internal/models"
)

// validTransitions maps each state to the states it may transition to.
// Rules are derived from LIFECYCLE.md.
var validTransitions = map[models.LifecycleState][]models.LifecycleState{
	models.StateRequested:  {models.StateIdentified},
	models.StateIdentified: {models.StateFunded, models.StatePledged},
	models.StateFunded:     {models.StatePledged},
	models.StatePledged:    {models.StateReleased, models.StateClaimed},
	models.StateClaimed:    {models.StateReleased},
	models.StateReleased:   {models.StateClosed},
	models.StateClosed:     {},
}

// ErrInvalidTransition is returned when a requested state change is not allowed.
type ErrInvalidTransition struct {
	From models.LifecycleState
	To   models.LifecycleState
}

func (e *ErrInvalidTransition) Error() string {
	return fmt.Sprintf("transition from %s to %s is not allowed", e.From, e.To)
}

// CanTransition reports whether moving from -> to is a valid transition.
func CanTransition(from, to models.LifecycleState) bool {
	targets, ok := validTransitions[from]
	if !ok {
		return false
	}
	for _, t := range targets {
		if t == to {
			return true
		}
	}
	return false
}

// Transition validates and returns the new state, or an error.
func Transition(current, next models.LifecycleState) (models.LifecycleState, error) {
	if !CanTransition(current, next) {
		return current, &ErrInvalidTransition{From: current, To: next}
	}
	return next, nil
}

// StateForIdentityUpdate maps the eID outcome to the resulting lifecycle state.
func StateForIdentityUpdate(eidStatus models.EIDStatus) (models.LifecycleState, bool) {
	if eidStatus == models.EIDVerified {
		return models.StateIdentified, true
	}
	return "", false
}
