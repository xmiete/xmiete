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
package statemachine_test

import (
	"testing"

	"github.com/xmiete/server/internal/models"
	"github.com/xmiete/server/internal/statemachine"
)

func TestValidTransitions(t *testing.T) {
	cases := []struct {
		from models.LifecycleState
		to   models.LifecycleState
	}{
		{models.StateRequested, models.StateIdentified},
		{models.StateIdentified, models.StateFunded},
		{models.StateIdentified, models.StatePledged},
		{models.StateFunded, models.StatePledged},
		{models.StateFunded, models.StateSettleProposed},
		{models.StatePledged, models.StateReleased},
		{models.StatePledged, models.StateClaimed},
		{models.StatePledged, models.StateSettleProposed},
		{models.StatePledged, models.StatePartiallyReleased},
		{models.StateClaimed, models.StateReleased},
		{models.StateReleased, models.StateClosed},
		{models.StatePartiallyReleased, models.StateSettleProposed},
		{models.StatePartiallyReleased, models.StateClosed},
		{models.StateSettleProposed, models.StateClosed},
		{models.StateSettleProposed, models.StateDisputed},
		{models.StateDisputed, models.StateClosed},
	}

	for _, tc := range cases {
		t.Run(string(tc.from)+"→"+string(tc.to), func(t *testing.T) {
			if !statemachine.CanTransition(tc.from, tc.to) {
				t.Errorf("CanTransition: expected valid, got false")
			}
			got, err := statemachine.Transition(tc.from, tc.to)
			if err != nil {
				t.Errorf("Transition returned unexpected error: %v", err)
			}
			if got != tc.to {
				t.Errorf("Transition: got %q, want %q", got, tc.to)
			}
		})
	}
}

func TestInvalidTransitions(t *testing.T) {
	cases := []struct {
		from models.LifecycleState
		to   models.LifecycleState
	}{
		// Terminal state allows no further transitions
		{models.StateClosed, models.StateRequested},
		{models.StateClosed, models.StateIdentified},
		{models.StateClosed, models.StatePledged},
		{models.StateClosed, models.StateReleased},
		// Skipping intermediate states
		{models.StateRequested, models.StatePledged},
		{models.StateRequested, models.StateReleased},
		{models.StateRequested, models.StateClosed},
		{models.StateRequested, models.StateSettleProposed},
		// Reverse transitions
		{models.StatePledged, models.StateRequested},
		{models.StatePledged, models.StateIdentified},
		{models.StateReleased, models.StatePledged},
		{models.StateClosed, models.StateReleased},
		// Dispute requires SETTLE_PROPOSED first
		{models.StatePledged, models.StateDisputed},
		{models.StateIdentified, models.StateDisputed},
		{models.StateReleased, models.StateDisputed},
		// Claim is only valid from PLEDGED
		{models.StateRequested, models.StateClaimed},
		{models.StateIdentified, models.StateClaimed},
		{models.StateReleased, models.StateClaimed},
		// Partial release only from PLEDGED
		{models.StateRequested, models.StatePartiallyReleased},
		{models.StateReleased, models.StatePartiallyReleased},
	}

	for _, tc := range cases {
		t.Run(string(tc.from)+"→"+string(tc.to), func(t *testing.T) {
			if statemachine.CanTransition(tc.from, tc.to) {
				t.Errorf("CanTransition: expected invalid, got true")
			}
			_, err := statemachine.Transition(tc.from, tc.to)
			if err == nil {
				t.Errorf("Transition: expected error, got nil")
			}
		})
	}
}

func TestStateForIdentityUpdate_Verified(t *testing.T) {
	state, ok := statemachine.StateForIdentityUpdate(models.EIDVerified)
	if !ok {
		t.Error("expected ok=true for VERIFIED")
	}
	if state != models.StateIdentified {
		t.Errorf("got %q, want %q", state, models.StateIdentified)
	}
}

func TestStateForIdentityUpdate_NoTransition(t *testing.T) {
	nonTransitioning := []models.EIDStatus{
		models.EIDFailed,
		models.EIDInProgress,
		models.EIDNotStarted,
	}
	for _, s := range nonTransitioning {
		t.Run(string(s), func(t *testing.T) {
			_, ok := statemachine.StateForIdentityUpdate(s)
			if ok {
				t.Errorf("expected ok=false for %s", s)
			}
		})
	}
}
