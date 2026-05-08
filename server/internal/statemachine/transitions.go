/*
 * Copyright 2026 XMiete Core Contributors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */
package statemachine

import (
	"fmt"

	"github.com/xmiete/server/internal/models"
)

// validTransitions maps each state to the states it may transition to.
// Rules are derived from LIFECYCLE.md.
var validTransitions = map[models.LifecycleState][]models.LifecycleState{
	models.StateRequested:         {models.StateIdentified},
	models.StateIdentified:        {models.StateFunded, models.StatePledged},
	models.StateFunded:            {models.StatePledged, models.StateSettleProposed},
	models.StatePledged:           {models.StateReleased, models.StateClaimed, models.StateSettleProposed, models.StatePartiallyReleased},
	models.StateClaimed:           {models.StateReleased},
	models.StateReleased:          {models.StateClosed},
	models.StatePartiallyReleased: {models.StateSettleProposed, models.StateClosed},
	models.StateSettleProposed:    {models.StateClosed, models.StateDisputed},
	models.StateDisputed:          {models.StateClosed},
	models.StateClosed:            {},
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
