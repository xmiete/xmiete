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

import (
	"fmt"
	"strings"
	"time"

	"github.com/xmiete/server/internal/models"
)

// Validate checks a deposit against XMiete schema requirements and jurisdiction-specific
// statutory rules. monthlyRent (net rent in deposit.currency) enables cap checks; pass 0
// to skip cap validation.
//
// Returns a Report where Valid = true means no ERROR-severity findings were raised.
func Validate(d *models.Deposit, monthlyRent float64) *Report {
	r := &Report{
		CheckedAt:     time.Now().UTC().Format(time.RFC3339),
		SchemaVersion: d.Meta.Version,
		Findings:      []Finding{},
	}

	checkRequired(r, d)
	checkDepositType(r, d)
	checkJurisdiction(r, d, monthlyRent)

	r.Valid = !r.hasErrors()
	return r
}

// ── Schema checks ─────────────────────────────────────────────────────────────

func checkRequired(r *Report, d *models.Deposit) {
	if d.Tenant.FirstName == "" || d.Tenant.LastName == "" || d.Tenant.Email == "" {
		r.add(SeverityError, "MISSING_TENANT_FIELDS", "tenant",
			"tenant.first_name, tenant.last_name and tenant.email are required", "")
	}
	if d.Landlord.Name == "" {
		r.add(SeverityError, "MISSING_LANDLORD_NAME", "landlord.name",
			"landlord.name is required", "")
	}
	if d.Deposit.Amount <= 0 {
		r.add(SeverityError, "INVALID_AMOUNT", "deposit.amount",
			"deposit.amount must be greater than zero", "")
	}
	if d.Deposit.Currency == "" {
		r.add(SeverityError, "MISSING_CURRENCY", "deposit.currency",
			"deposit.currency is required (ISO 4217)", "")
	}
	if d.Deposit.Type == "" {
		r.add(SeverityError, "MISSING_TYPE", "deposit.type",
			"deposit.type is required", "")
	}
	if d.Meta.Version == "" {
		r.add(SeverityWarning, "MISSING_VERSION", "meta.version",
			"meta.version should be set (e.g. \"2.2.0\")", "")
	}
	if d.Property.Address.Country == "" {
		r.add(SeverityWarning, "MISSING_COUNTRY", "property.address.country",
			"property.address.country is required for jurisdiction-specific validation", "")
	}
}

func checkDepositType(r *Report, d *models.Deposit) {
	if d.Deposit.Type == "" {
		return
	}
	known := map[models.DepositType]bool{
		models.DepositTypeCash:          true,
		models.DepositTypeBankGuarantee: true,
		models.DepositTypeInsurance:     true,
		"SURETY":                        true, // added in schema v2.2.0 (Bürgschaft/caution)
	}
	if !known[d.Deposit.Type] {
		r.add(SeverityError, "UNKNOWN_TYPE", "deposit.type",
			fmt.Sprintf("unknown deposit.type %q — must be CASH_EQUIVALENT, BANK_GUARANTEE, INSURANCE, or SURETY", d.Deposit.Type), "")
	}
}

// ── Jurisdiction checks ───────────────────────────────────────────────────────

func checkJurisdiction(r *Report, d *models.Deposit, monthlyRent float64) {
	country := d.Property.Address.Country
	r.Jurisdiction = country
	if country == "" {
		return
	}

	rule, known := jurisdictionRules[country]
	if !known {
		r.add(SeverityInfo, "UNKNOWN_JURISDICTION", "property.address.country",
			fmt.Sprintf("no statutory rules defined for country %q — schema checks only", country), "")
		return
	}

	ref := rule.primaryStatutoryRef()

	// Currency
	if rule.Currency != "" && d.Deposit.Currency != "" && d.Deposit.Currency != rule.Currency {
		r.add(SeverityError, "WRONG_CURRENCY", "deposit.currency",
			fmt.Sprintf("%s requires currency %s, got %s", country, rule.Currency, d.Deposit.Currency), ref)
	}

	// Deposit type
	if len(rule.AllowedTypes) > 0 && d.Deposit.Type != "" {
		allowed := false
		for _, t := range rule.AllowedTypes {
			if string(d.Deposit.Type) == t {
				allowed = true
				break
			}
		}
		if !allowed {
			r.add(SeverityError, "DISALLOWED_TYPE", "deposit.type",
				fmt.Sprintf("%s does not permit deposit type %s (allowed: %s)",
					country, d.Deposit.Type, rule.allowedTypesString()), ref)
		}
	}

	// Cap check — only when monthly rent is supplied
	if rule.MaxMonths > 0 && monthlyRent > 0 && d.Deposit.Amount > 0 {
		maxAmount := rule.MaxMonths * monthlyRent
		if d.Deposit.Amount > maxAmount*1.005 { // 0.5 % tolerance for rounding
			r.add(SeverityError, "CAP_EXCEEDED", "deposit.amount",
				fmt.Sprintf("%s statutory cap is %.2g months rent — deposit %.2f exceeds max %.2f %s",
					country, rule.MaxMonths, d.Deposit.Amount, maxAmount, d.Deposit.Currency), ref)
		}
	}

	// Statutory basis (only checked when pledge is present)
	if d.Pledge != nil && d.Pledge.StatutoryBasis != "" && len(rule.StatutoryBases) > 0 {
		matched := false
		for _, basis := range rule.StatutoryBases {
			if strings.Contains(d.Pledge.StatutoryBasis, basis) {
				matched = true
				break
			}
		}
		if !matched {
			r.add(SeverityWarning, "UNEXPECTED_STATUTORY_BASIS", "pledge.statutory_basis",
				fmt.Sprintf("statutory_basis %q is unexpected for %s (expected one of: %s)",
					d.Pledge.StatutoryBasis, country, strings.Join(rule.StatutoryBases, " / ")), ref)
		}
	}

	// Interest requirement
	if rule.InterestRequired {
		if d.Trusteeship == nil || d.Trusteeship.InterestRate == 0 {
			r.add(SeverityWarning, "INTEREST_REQUIRED", "trusteeship.interest_rate",
				fmt.Sprintf("%s law requires interest to accrue on the deposit for the tenant's benefit; trusteeship.interest_rate is not set", country), ref)
		}
	}

	// Trusteeship plausibility for cash deposits
	if d.Deposit.Type == models.DepositTypeCash && d.Trusteeship == nil {
		r.add(SeverityWarning, "MISSING_TRUSTEESHIP", "trusteeship",
			"CASH_EQUIVALENT deposits should declare a trusteeship with insolvency_protection_confirmed = true", ref)
	}
	if d.Trusteeship != nil && !d.Trusteeship.InsolvencyProtectionConfirmed {
		r.add(SeverityWarning, "INSOLVENCY_PROTECTION_UNCONFIRMED", "trusteeship.insolvency_protection_confirmed",
			"trusteeship.insolvency_protection_confirmed should be true — BGB § 551 Abs. 3 and equivalent require insolvency-proof separation", ref)
	}
}
