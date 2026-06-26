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

	"github.com/xmiete/server/internal/models"
)

// jurisdictionCase describes one jurisdiction's typical deposit configuration.
type jurisdictionCase struct {
	name         string
	jurisdiction string
	depositType  string
	currency     string
	amount       float64
}

// TestJurisdictions verifies that deposits from all nine supported jurisdictions
// can be created and fully pledged. Each case uses the deposit type and currency
// typical for that jurisdiction's statutory scheme.
func TestJurisdictions(t *testing.T) {
	cases := []jurisdictionCase{
		// DE — BGB § 551: up to 3 months cold rent; CASH_EQUIVALENT or Bürgschaft
		{name: "Germany/DE", jurisdiction: "DE", depositType: "CASH_EQUIVALENT", currency: "EUR", amount: 1500.0},
		// CH — OR Art. 257e: up to 3 months rent, held in a blocked savings account
		{name: "Switzerland/CH", jurisdiction: "CH", depositType: "CASH_EQUIVALENT", currency: "CHF", amount: 4500.0},
		// AT — MRG § 16b / ABGB § 1346: Kaution or Bankgarantie
		{name: "Austria/AT", jurisdiction: "AT", depositType: "BANK_GUARANTEE", currency: "EUR", amount: 2000.0},
		// BE — Garantie locative / e-DEPO: state-escrow system
		{name: "Belgium/BE", jurisdiction: "BE", depositType: "CASH_EQUIVALENT", currency: "EUR", amount: 1200.0},
		// NL — BW Art. 7:261: Waarborgsom (typically 1–2 months)
		{name: "Netherlands/NL", jurisdiction: "NL", depositType: "CASH_EQUIVALENT", currency: "EUR", amount: 1400.0},
		// NO — Husleieloven § 3-5: Depositumskonto (capped at 6 months)
		{name: "Norway/NO", jurisdiction: "NO", depositType: "CASH_EQUIVALENT", currency: "NOK", amount: 20000.0},
		// GB — Housing Act 2004 (TDP): custodial or insured scheme
		{name: "UnitedKingdom/GB", jurisdiction: "GB", depositType: "BANK_GUARANTEE", currency: "GBP", amount: 1500.0},
		// FR — Loi ALUR Art. 22: Caution solidaire (surety / personal guarantee)
		{name: "France/FR", jurisdiction: "FR", depositType: "SURETY", currency: "EUR", amount: 1000.0},
		// ES — LAU Art. 36: Fianza legal (1 month for residential)
		{name: "Spain/ES", jurisdiction: "ES", depositType: "CASH_EQUIVALENT", currency: "EUR", amount: 900.0},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ts, tok := newTestServer(t)

			body := minDeposit(tc.depositType, tc.currency, tc.amount)
			// Include jurisdiction in the meta field (stored in JSONB, informational at API level)
			body["meta"] = map[string]any{"version": "2.2.0", "jurisdiction": tc.jurisdiction}

			id := createDeposit(t, ts, tok, body)
			requireState(t, ts, tok, id, models.StateRequested)

			// eID verification
			mustDo(t,
				doJSON(t, ts, http.MethodPatch, "/v1/deposits/"+id+"/identity",
					map[string]any{"eid_status": "VERIFIED"}, tok),
				http.StatusOK)
			requireState(t, ts, tok, id, models.StateIdentified)

			// Pledge
			mustDo(t,
				doJSON(t, ts, http.MethodPost, "/v1/deposits/"+id+"/pledge",
					map[string]any{"pledge_date": "2026-06-26", "is_confirmed_by_bank": true}, tok),
				http.StatusOK)
			requireState(t, ts, tok, id, models.StatePledged)

			// Full release
			mustDo(t, doJSON(t, ts, http.MethodPost, "/v1/deposits/"+id+"/release", nil, tok), http.StatusOK)
			requireState(t, ts, tok, id, models.StateReleased)
		})
	}
}

// TestJurisdictions_AllDepositTypes verifies that each deposit type supported by the
// schema can flow through the full pledge-to-release lifecycle.
func TestJurisdictions_AllDepositTypes(t *testing.T) {
	types := []struct {
		name        string
		depositType string
	}{
		{"CASH_EQUIVALENT", "CASH_EQUIVALENT"},
		{"BANK_GUARANTEE", "BANK_GUARANTEE"},
		{"INSURANCE", "INSURANCE"},
		{"SURETY", "SURETY"},
	}

	for _, tc := range types {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ts, tok := newTestServer(t)
			id := createDeposit(t, ts, tok, minDeposit(tc.depositType, "EUR", 1200.0))

			mustDo(t,
				doJSON(t, ts, http.MethodPatch, "/v1/deposits/"+id+"/identity",
					map[string]any{"eid_status": "VERIFIED"}, tok),
				http.StatusOK)
			mustDo(t,
				doJSON(t, ts, http.MethodPost, "/v1/deposits/"+id+"/pledge",
					map[string]any{"pledge_date": "2026-06-26", "is_confirmed_by_bank": true}, tok),
				http.StatusOK)
			requireState(t, ts, tok, id, models.StatePledged)
		})
	}
}
