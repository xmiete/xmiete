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
package api_test

import (
	"net/http"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/xmiete/server/internal/models"
)

// TestLifecycle_HappyPath tests the primary deposit lifecycle:
// REQUESTED → IDENTIFIED → PLEDGED → RELEASED.
func TestLifecycle_HappyPath(t *testing.T) {
	ts, tok := newTestServer(t)

	id := createDeposit(t, ts, tok, minDeposit("CASH_EQUIVALENT", "EUR", 1500.0))
	requireState(t, ts, tok, id, models.StateRequested)

	mustDo(t,
		doJSON(t, ts, http.MethodPatch, "/v1/deposits/"+id+"/identity",
			map[string]any{"eid_status": "VERIFIED"}, tok),
		http.StatusOK)
	requireState(t, ts, tok, id, models.StateIdentified)

	mustDo(t,
		doJSON(t, ts, http.MethodPost, "/v1/deposits/"+id+"/pledge",
			map[string]any{"pledge_date": "2026-06-26", "is_confirmed_by_bank": true}, tok),
		http.StatusOK)
	requireState(t, ts, tok, id, models.StatePledged)

	mustDo(t, doJSON(t, ts, http.MethodPost, "/v1/deposits/"+id+"/release", nil, tok), http.StatusOK)
	requireState(t, ts, tok, id, models.StateReleased)
}

// TestLifecycle_EIDFailed verifies that a FAILED eID result does not advance the state.
func TestLifecycle_EIDFailed(t *testing.T) {
	ts, tok := newTestServer(t)
	id := createDeposit(t, ts, tok, minDeposit("CASH_EQUIVALENT", "EUR", 1500.0))

	mustDo(t,
		doJSON(t, ts, http.MethodPatch, "/v1/deposits/"+id+"/identity",
			map[string]any{"eid_status": "FAILED"}, tok),
		http.StatusOK)
	requireState(t, ts, tok, id, models.StateRequested)
}

// TestLifecycle_Settlement_Accept tests the full settlement accept path:
// PLEDGED → SETTLE_PROPOSED → CLOSED.
func TestLifecycle_Settlement_Accept(t *testing.T) {
	ts, tok := newTestServer(t)
	id := createAndPledge(t, ts, tok)

	mustDo(t,
		doJSON(t, ts, http.MethodPost, "/v1/deposits/"+id+"/settle", map[string]any{
			"initiated_by": "LANDLORD",
			"claim_items":  []map[string]any{{"description": "Cleaning costs", "amount_claimed": 200.0, "category": "CLEANING"}},
			"proposed_tenant_refund": 1300.0, "proposed_landlord_retention": 200.0,
		}, tok),
		http.StatusOK)
	requireState(t, ts, tok, id, models.StateSettleProposed)

	mustDo(t,
		doJSON(t, ts, http.MethodPost, "/v1/deposits/"+id+"/settle/accept",
			map[string]any{"accepted_by": "TENANT"}, tok),
		http.StatusOK)
	requireState(t, ts, tok, id, models.StateClosed)
}

// TestLifecycle_Settlement_CounterProposal verifies that a counter-proposal keeps the
// state at SETTLE_PROPOSED and that the other party can then accept.
func TestLifecycle_Settlement_CounterProposal(t *testing.T) {
	ts, tok := newTestServer(t)
	id := createAndPledge(t, ts, tok)

	// Landlord proposes
	mustDo(t,
		doJSON(t, ts, http.MethodPost, "/v1/deposits/"+id+"/settle", map[string]any{
			"initiated_by": "LANDLORD",
			"claim_items":  []map[string]any{{"description": "Damage", "amount_claimed": 500.0, "category": "DAMAGE"}},
			"proposed_tenant_refund": 1000.0, "proposed_landlord_retention": 500.0,
		}, tok),
		http.StatusOK)
	requireState(t, ts, tok, id, models.StateSettleProposed)

	// Tenant counter-proposes
	mustDo(t,
		doJSON(t, ts, http.MethodPost, "/v1/deposits/"+id+"/settle", map[string]any{
			"initiated_by": "TENANT",
			"claim_items":  []map[string]any{{"description": "Damage (disputed)", "amount_claimed": 200.0, "category": "DAMAGE"}},
			"proposed_tenant_refund": 1300.0, "proposed_landlord_retention": 200.0,
		}, tok),
		http.StatusOK)
	requireState(t, ts, tok, id, models.StateSettleProposed)

	// Landlord accepts the counter
	mustDo(t,
		doJSON(t, ts, http.MethodPost, "/v1/deposits/"+id+"/settle/accept",
			map[string]any{"accepted_by": "LANDLORD"}, tok),
		http.StatusOK)
	requireState(t, ts, tok, id, models.StateClosed)
}

// TestLifecycle_Settlement_Dispute tests the dispute escalation path:
// PLEDGED → SETTLE_PROPOSED → DISPUTED → CLOSED.
func TestLifecycle_Settlement_Dispute(t *testing.T) {
	ts, tok := newTestServer(t)
	id := createAndPledge(t, ts, tok)

	mustDo(t,
		doJSON(t, ts, http.MethodPost, "/v1/deposits/"+id+"/settle", map[string]any{
			"initiated_by": "LANDLORD",
			"claim_items":  []map[string]any{{"description": "Damage", "amount_claimed": 800.0, "category": "DAMAGE"}},
			"proposed_tenant_refund": 700.0, "proposed_landlord_retention": 800.0,
		}, tok),
		http.StatusOK)

	mustDo(t,
		doJSON(t, ts, http.MethodPost, "/v1/deposits/"+id+"/dispute", map[string]any{
			"escalated_by": "TENANT", "escalation_type": "AMTSGERICHT",
		}, tok),
		http.StatusOK)
	requireState(t, ts, tok, id, models.StateDisputed)

	mustDo(t,
		doJSON(t, ts, http.MethodPost, "/v1/deposits/"+id+"/dispute/resolve", map[string]any{
			"agreed_tenant_refund": 1100.0, "agreed_landlord_retention": 400.0,
			"resolution_reference": "AZ-2026-1234",
		}, tok),
		http.StatusOK)
	requireState(t, ts, tok, id, models.StateClosed)
}

// TestLifecycle_Claim tests the landlord claim path:
// PLEDGED → CLAIMED → RELEASED.
func TestLifecycle_Claim(t *testing.T) {
	ts, tok := newTestServer(t)
	id := createAndPledge(t, ts, tok)

	mustDo(t,
		doJSON(t, ts, http.MethodPost, "/v1/deposits/"+id+"/claim",
			map[string]any{"claim_amount": 800.0, "reason": "Rent arrears"}, tok),
		http.StatusOK)
	requireState(t, ts, tok, id, models.StateClaimed)

	mustDo(t, doJSON(t, ts, http.MethodPost, "/v1/deposits/"+id+"/release", nil, tok), http.StatusOK)
	requireState(t, ts, tok, id, models.StateReleased)
}

// TestLifecycle_PartialRelease_NoShortfall tests the utility reservation path
// when the actual cost is within the advance paid:
// PLEDGED → PARTIALLY_RELEASED → CLOSED.
func TestLifecycle_PartialRelease_NoShortfall(t *testing.T) {
	ts, tok := newTestServer(t)
	id := createAndPledge(t, ts, tok)

	mustDo(t,
		doJSON(t, ts, http.MethodPost, "/v1/deposits/"+id+"/partial-release", map[string]any{
			"released_amount": 1200.0, "reserved_amount": 300.0, "billing_period_end": "2026-12-31",
		}, tok),
		http.StatusOK)
	requireState(t, ts, tok, id, models.StatePartiallyReleased)

	mustDo(t,
		doJSON(t, ts, http.MethodPost, "/v1/deposits/"+id+"/utility-settle",
			map[string]any{"actual_cost": 250.0, "total_advance_paid": 300.0}, tok),
		http.StatusOK)
	requireState(t, ts, tok, id, models.StateClosed)
}

// TestLifecycle_PartialRelease_Shortfall tests the utility reservation path
// when the tenant owes more than the advance paid:
// PLEDGED → PARTIALLY_RELEASED → SETTLE_PROPOSED → CLOSED.
func TestLifecycle_PartialRelease_Shortfall(t *testing.T) {
	ts, tok := newTestServer(t)
	id := createAndPledge(t, ts, tok)

	mustDo(t,
		doJSON(t, ts, http.MethodPost, "/v1/deposits/"+id+"/partial-release", map[string]any{
			"released_amount": 1200.0, "reserved_amount": 300.0, "billing_period_end": "2026-12-31",
		}, tok),
		http.StatusOK)

	// Shortfall: actual 450 > advances 300 → SETTLE_PROPOSED
	mustDo(t,
		doJSON(t, ts, http.MethodPost, "/v1/deposits/"+id+"/utility-settle",
			map[string]any{"actual_cost": 450.0, "total_advance_paid": 300.0}, tok),
		http.StatusOK)
	requireState(t, ts, tok, id, models.StateSettleProposed)

	mustDo(t,
		doJSON(t, ts, http.MethodPost, "/v1/deposits/"+id+"/settle/accept",
			map[string]any{"accepted_by": "TENANT"}, tok),
		http.StatusOK)
	requireState(t, ts, tok, id, models.StateClosed)
}

// TestLifecycle_InvalidTransitions verifies that invalid state transitions return 409 Conflict.
func TestLifecycle_InvalidTransitions(t *testing.T) {
	ts, tok := newTestServer(t)
	id := createDeposit(t, ts, tok, minDeposit("CASH_EQUIVALENT", "EUR", 1500.0))

	// Cannot pledge before identity verification
	mustDo(t,
		doJSON(t, ts, http.MethodPost, "/v1/deposits/"+id+"/pledge",
			map[string]any{"pledge_date": "2026-06-26", "is_confirmed_by_bank": true}, tok),
		http.StatusConflict)

	// Cannot release from REQUESTED
	mustDo(t, doJSON(t, ts, http.MethodPost, "/v1/deposits/"+id+"/release", nil, tok), http.StatusConflict)

	// Cannot dispute from REQUESTED (dispute requires SETTLE_PROPOSED)
	mustDo(t,
		doJSON(t, ts, http.MethodPost, "/v1/deposits/"+id+"/dispute",
			map[string]any{"escalated_by": "TENANT", "escalation_type": "AMTSGERICHT"}, tok),
		http.StatusConflict)

	// Advance to PLEDGED
	mustDo(t,
		doJSON(t, ts, http.MethodPatch, "/v1/deposits/"+id+"/identity",
			map[string]any{"eid_status": "VERIFIED"}, tok),
		http.StatusOK)
	mustDo(t,
		doJSON(t, ts, http.MethodPost, "/v1/deposits/"+id+"/pledge",
			map[string]any{"pledge_date": "2026-06-26", "is_confirmed_by_bank": true}, tok),
		http.StatusOK)

	// Cannot dispute directly from PLEDGED
	mustDo(t,
		doJSON(t, ts, http.MethodPost, "/v1/deposits/"+id+"/dispute",
			map[string]any{"escalated_by": "TENANT", "escalation_type": "AMTSGERICHT"}, tok),
		http.StatusConflict)

	// Release
	mustDo(t, doJSON(t, ts, http.MethodPost, "/v1/deposits/"+id+"/release", nil, tok), http.StatusOK)

	// Cannot settle after release
	mustDo(t,
		doJSON(t, ts, http.MethodPost, "/v1/deposits/"+id+"/settle", map[string]any{
			"initiated_by": "LANDLORD",
			"claim_items":  []map[string]any{{"description": "X", "amount_claimed": 100.0, "category": "OTHER"}},
			"proposed_tenant_refund": 1400.0, "proposed_landlord_retention": 100.0,
		}, tok),
		http.StatusConflict)

	// Cannot pledge again after release
	mustDo(t,
		doJSON(t, ts, http.MethodPost, "/v1/deposits/"+id+"/pledge",
			map[string]any{"pledge_date": "2026-06-26", "is_confirmed_by_bank": true}, tok),
		http.StatusConflict)
}

// TestLifecycle_ValidationErrors verifies input validation on deposit creation.
func TestLifecycle_ValidationErrors(t *testing.T) {
	ts, tok := newTestServer(t)

	// Missing tenant last_name and email
	mustDo(t,
		doJSON(t, ts, http.MethodPost, "/v1/deposits",
			map[string]any{"tenant": map[string]any{"first_name": "Test"}}, tok),
		http.StatusBadRequest)

	// Missing landlord
	mustDo(t,
		doJSON(t, ts, http.MethodPost, "/v1/deposits", map[string]any{
			"meta":   map[string]any{"version": "2.2.0"},
			"tenant": map[string]any{"first_name": "T", "last_name": "T", "email": "t@t.com"},
		}, tok),
		http.StatusBadRequest)

	// Negative deposit amount
	mustDo(t,
		doJSON(t, ts, http.MethodPost, "/v1/deposits",
			minDeposit("CASH_EQUIVALENT", "EUR", -100.0), tok),
		http.StatusBadRequest)

	// Zero deposit amount
	mustDo(t,
		doJSON(t, ts, http.MethodPost, "/v1/deposits",
			minDeposit("CASH_EQUIVALENT", "EUR", 0), tok),
		http.StatusBadRequest)

	// Non-existent deposit
	mustDo(t,
		doJSON(t, ts, http.MethodGet, "/v1/deposits/00000000-0000-0000-0000-000000000000", nil, tok),
		http.StatusNotFound)
}

// TestLifecycle_Unauthenticated verifies that JWT-protected endpoints reject missing/invalid tokens.
func TestLifecycle_Unauthenticated(t *testing.T) {
	ts, _ := newTestServer(t)

	// No Authorization header
	mustDo(t,
		doJSON(t, ts, http.MethodPost, "/v1/deposits", minDeposit("CASH_EQUIVALENT", "EUR", 1500.0), ""),
		http.StatusUnauthorized)

	// Malformed token
	mustDo(t,
		doJSON(t, ts, http.MethodPost, "/v1/deposits", minDeposit("CASH_EQUIVALENT", "EUR", 1500.0), "not-a-jwt"),
		http.StatusUnauthorized)

	// Wrong secret
	wrongToken := func() string {
		tok := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
			"sub": "attacker", "exp": time.Now().Add(time.Hour).Unix(),
		})
		s, _ := tok.SignedString([]byte("wrong-secret"))
		return s
	}()
	mustDo(t,
		doJSON(t, ts, http.MethodPost, "/v1/deposits", minDeposit("CASH_EQUIVALENT", "EUR", 1500.0), wrongToken),
		http.StatusUnauthorized)
}
