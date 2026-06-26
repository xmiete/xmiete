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
package validator

import "strings"

// jurisdictionRule contains the statutory constraints for one jurisdiction.
type jurisdictionRule struct {
	// StatutoryBases are acceptable pledge.statutory_basis values (substring match).
	StatutoryBases []string
	// Currency is the required ISO 4217 code.
	Currency string
	// MaxMonths is the deposit cap expressed as months of net rent (0 = no cap rule).
	MaxMonths float64
	// AllowedTypes lists the permitted deposit types (empty = all allowed).
	AllowedTypes []string
	// InterestRequired flags that the jurisdiction mandates interest accrual for the tenant.
	InterestRequired bool
}

func (r jurisdictionRule) primaryStatutoryRef() string {
	if len(r.StatutoryBases) == 0 {
		return ""
	}
	return r.StatutoryBases[0]
}

func (r jurisdictionRule) allowedTypesString() string {
	return strings.Join(r.AllowedTypes, ", ")
}

// jurisdictionRules is the authoritative table of statutory deposit rules per country.
// Sources: BGB § 551 (DE), OR Art. 257e (CH), MRG § 16b (AT), Woninghuurwet (BE),
// BW Art. 7:261 (NL), Husleieloven § 3-5 (NO), Housing Act 2004 + Tenant Fees Act 2019 (GB),
// Loi ALUR / Loi 89-462 (FR), LAU Art. 36 (ES).
var jurisdictionRules = map[string]jurisdictionRule{
	// Germany — BGB § 551: max 3 Kaltmieten; Treuhandkonto required; no advance interest obligation
	"DE": {
		StatutoryBases: []string{"BGB § 551"},
		Currency:       "EUR",
		MaxMonths:      3.0,
		AllowedTypes:   []string{"CASH_EQUIVALENT", "BANK_GUARANTEE", "SURETY"},
	},
	// Switzerland — OR Art. 257e: max 3 months; bank holds in blocked savings; interest accrues to tenant
	"CH": {
		StatutoryBases:   []string{"OR Art. 257e"},
		Currency:         "CHF",
		MaxMonths:        3.0,
		InterestRequired: true,
	},
	// Austria — MRG § 16b / ABGB § 1346: max 3 Bruttomonatsmieten
	"AT": {
		StatutoryBases: []string{"MRG § 16b", "ABGB § 1346"},
		Currency:       "EUR",
		MaxMonths:      3.0,
	},
	// Belgium — Woninghuurwet / Code civil: max 2 months (guaranteed via Fonds du Logement / e-DEPO)
	"BE": {
		StatutoryBases: []string{"Woninghuurwet", "Code civil belge"},
		Currency:       "EUR",
		MaxMonths:      2.0,
	},
	// Netherlands — BW Art. 7:261: max 2 months (reduced from 3 in July 2023)
	"NL": {
		StatutoryBases: []string{"BW Art. 7:261"},
		Currency:       "EUR",
		MaxMonths:      2.0,
		AllowedTypes:   []string{"CASH_EQUIVALENT"},
	},
	// Norway — Husleieloven § 3-5: max 6 months; Depositumskonto; interest accrues to tenant
	"NO": {
		StatutoryBases:   []string{"Husleieloven § 3-5"},
		Currency:         "NOK",
		MaxMonths:        6.0,
		InterestRequired: true,
	},
	// Great Britain — Tenant Fees Act 2019: max 5 weeks (≈1.15 months) for AST ≤ £50k p.a.;
	// must be held in a government-authorised TDP scheme
	"GB": {
		StatutoryBases: []string{"Housing Act 2004", "Tenant Fees Act 2019"},
		Currency:       "GBP",
		MaxMonths:      1.15,
	},
	// France — Loi ALUR / Loi 89-462: max 1 month net rent (unfurnished); 2 months (furnished)
	"FR": {
		StatutoryBases: []string{"Loi ALUR", "Loi 89-462"},
		Currency:       "EUR",
		MaxMonths:      1.0,
		AllowedTypes:   []string{"CASH_EQUIVALENT", "SURETY"},
	},
	// Spain — LAU Art. 36: 1 month fianza for residential (2 months for non-residential)
	"ES": {
		StatutoryBases: []string{"LAU Art. 36"},
		Currency:       "EUR",
		MaxMonths:      1.0,
	},
}
