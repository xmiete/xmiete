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
package validator_test

import (
	"testing"

	"github.com/xmiete/server/internal/models"
	"github.com/xmiete/server/internal/validator"
)

// minDeposit returns a minimal valid deposit for a given country and deposit type.
func minDeposit(country, depositType, currency string, amount float64) *models.Deposit {
	return &models.Deposit{
		Meta: models.Meta{Version: "2.2.0"},
		Tenant: models.Tenant{
			FirstName: "Test",
			LastName:  "Tenant",
			Email:     "tenant@example.com",
		},
		Landlord: models.Landlord{Name: "Test Landlord GmbH"},
		Property: models.Property{
			Address: models.Address{
				Street:  "Teststr. 1",
				ZIP:     "10115",
				City:    "Berlin",
				Country: country,
			},
		},
		Deposit: models.DepositData{
			Amount:   amount,
			Currency: currency,
			Type:     models.DepositType(depositType),
		},
	}
}

func hasCode(r *validator.Report, code string) bool {
	for _, f := range r.Findings {
		if f.Code == code {
			return true
		}
	}
	return false
}

func hasError(r *validator.Report, code string) bool {
	for _, f := range r.Findings {
		if f.Code == code && f.Severity == validator.SeverityError {
			return true
		}
	}
	return false
}

// TestValidate_MinimalValid verifies that a minimal well-formed deposit passes.
func TestValidate_MinimalValid(t *testing.T) {
	d := minDeposit("DE", "CASH_EQUIVALENT", "EUR", 1500.0)
	r := validator.Validate(d, 0)
	if !r.Valid {
		t.Errorf("expected valid, got findings: %+v", r.Findings)
	}
	if r.Jurisdiction != "DE" {
		t.Errorf("jurisdiction: got %q, want DE", r.Jurisdiction)
	}
	if r.SchemaVersion != "2.2.0" {
		t.Errorf("schema_version: got %q, want 2.2.0", r.SchemaVersion)
	}
}

// TestValidate_MissingRequiredFields verifies ERROR findings for missing required fields.
func TestValidate_MissingRequiredFields(t *testing.T) {
	d := &models.Deposit{}
	r := validator.Validate(d, 0)
	if r.Valid {
		t.Error("expected invalid for empty deposit")
	}
	for _, code := range []string{"MISSING_TENANT_FIELDS", "MISSING_LANDLORD_NAME", "INVALID_AMOUNT", "MISSING_CURRENCY", "MISSING_TYPE"} {
		if !hasError(r, code) {
			t.Errorf("expected ERROR %q", code)
		}
	}
}

// TestValidate_NegativeAmount verifies that a negative amount is rejected.
func TestValidate_NegativeAmount(t *testing.T) {
	d := minDeposit("DE", "CASH_EQUIVALENT", "EUR", -100)
	r := validator.Validate(d, 0)
	if !hasError(r, "INVALID_AMOUNT") {
		t.Error("expected INVALID_AMOUNT for negative deposit")
	}
}

// TestValidate_UnknownType verifies that an unknown deposit type is rejected.
func TestValidate_UnknownType(t *testing.T) {
	d := minDeposit("DE", "WARP_BOND", "EUR", 1000)
	r := validator.Validate(d, 0)
	if !hasError(r, "UNKNOWN_TYPE") {
		t.Error("expected UNKNOWN_TYPE")
	}
}

// TestValidate_CapCheck_Pass verifies that a deposit within the statutory cap passes.
func TestValidate_CapCheck_Pass(t *testing.T) {
	d := minDeposit("DE", "CASH_EQUIVALENT", "EUR", 1500) // exactly 3 months of 500
	r := validator.Validate(d, 500)
	if !r.Valid {
		t.Errorf("expected valid for 3-month deposit at 500 rent, got: %+v", r.Findings)
	}
	if hasCode(r, "CAP_EXCEEDED") {
		t.Error("unexpected CAP_EXCEEDED")
	}
}

// TestValidate_CapCheck_Exceeded verifies that an over-cap deposit raises an ERROR.
func TestValidate_CapCheck_Exceeded(t *testing.T) {
	d := minDeposit("DE", "CASH_EQUIVALENT", "EUR", 2000) // 4 months of 500 — exceeds 3-month cap
	r := validator.Validate(d, 500)
	if !hasError(r, "CAP_EXCEEDED") {
		t.Error("expected CAP_EXCEEDED for 4-month deposit in DE")
	}
}

// TestValidate_WrongCurrency verifies that a non-EUR deposit in DE is rejected.
func TestValidate_WrongCurrency(t *testing.T) {
	d := minDeposit("DE", "CASH_EQUIVALENT", "USD", 1500)
	r := validator.Validate(d, 0)
	if !hasError(r, "WRONG_CURRENCY") {
		t.Error("expected WRONG_CURRENCY for USD deposit in DE")
	}
}

// TestValidate_DisallowedType_NL verifies that a BANK_GUARANTEE is rejected in NL.
func TestValidate_DisallowedType_NL(t *testing.T) {
	d := minDeposit("NL", "BANK_GUARANTEE", "EUR", 1000)
	r := validator.Validate(d, 0)
	if !hasError(r, "DISALLOWED_TYPE") {
		t.Error("expected DISALLOWED_TYPE for BANK_GUARANTEE in NL")
	}
}

// TestValidate_GB_CapWeeks verifies the 5-week cap for GB (Tenant Fees Act 2019).
func TestValidate_GB_CapWeeks(t *testing.T) {
	// 5 weeks = 5/52 * 12 ≈ 1.154 months. Monthly rent 2000 GBP → max ≈ 2308 GBP.
	d := minDeposit("GB", "CASH_EQUIVALENT", "GBP", 2500)
	r := validator.Validate(d, 2000)
	if !hasError(r, "CAP_EXCEEDED") {
		t.Error("expected CAP_EXCEEDED for 2500 GBP deposit with 2000 GBP/month rent in GB")
	}

	d2 := minDeposit("GB", "CASH_EQUIVALENT", "GBP", 2000)
	r2 := validator.Validate(d2, 2000)
	if hasCode(r2, "CAP_EXCEEDED") {
		t.Error("unexpected CAP_EXCEEDED for 2000 GBP deposit with 2000 GBP/month rent in GB")
	}
}

// TestValidate_CH_InterestRequired verifies the interest warning for CH without trusteeship.
func TestValidate_CH_InterestRequired(t *testing.T) {
	d := minDeposit("CH", "CASH_EQUIVALENT", "CHF", 3000)
	r := validator.Validate(d, 0)
	if !hasCode(r, "INTEREST_REQUIRED") {
		t.Error("expected INTEREST_REQUIRED warning for CH deposit without trusteeship")
	}
	// With interest rate set — warning should clear
	d.Trusteeship = &models.Trusteeship{InterestRate: 0.5, InsolvencyProtectionConfirmed: true}
	r2 := validator.Validate(d, 0)
	if hasCode(r2, "INTEREST_REQUIRED") {
		t.Error("unexpected INTEREST_REQUIRED when trusteeship.interest_rate is set")
	}
}

// TestValidate_MissingTrusteeship verifies the warning for CASH_EQUIVALENT without trusteeship.
func TestValidate_MissingTrusteeship(t *testing.T) {
	d := minDeposit("DE", "CASH_EQUIVALENT", "EUR", 1500)
	r := validator.Validate(d, 0)
	if !hasCode(r, "MISSING_TRUSTEESHIP") {
		t.Error("expected MISSING_TRUSTEESHIP warning for CASH_EQUIVALENT without trusteeship")
	}
	// Bank guarantee does not need a trusteeship warning
	d2 := minDeposit("DE", "BANK_GUARANTEE", "EUR", 1500)
	r2 := validator.Validate(d2, 0)
	if hasCode(r2, "MISSING_TRUSTEESHIP") {
		t.Error("unexpected MISSING_TRUSTEESHIP for BANK_GUARANTEE")
	}
}

// TestValidate_UnknownJurisdiction verifies INFO finding for unsupported country.
func TestValidate_UnknownJurisdiction(t *testing.T) {
	d := minDeposit("JP", "CASH_EQUIVALENT", "JPY", 100000)
	r := validator.Validate(d, 0)
	if !hasCode(r, "UNKNOWN_JURISDICTION") {
		t.Error("expected UNKNOWN_JURISDICTION for JP")
	}
	// Unknown jurisdiction should still be valid (INFO is not an error)
	if !r.Valid {
		t.Error("deposit with unknown jurisdiction should not be invalid")
	}
}

// TestValidate_AllJurisdictions verifies that a typical minimal deposit passes in each supported country.
func TestValidate_AllJurisdictions(t *testing.T) {
	cases := []struct {
		country     string
		depositType string
		currency    string
		amount      float64
		monthly     float64
	}{
		{"DE", "CASH_EQUIVALENT", "EUR", 1500, 600},
		{"CH", "CASH_EQUIVALENT", "CHF", 3000, 1500},
		{"AT", "BANK_GUARANTEE", "EUR", 2000, 800},
		{"BE", "CASH_EQUIVALENT", "EUR", 1200, 700},
		{"NL", "CASH_EQUIVALENT", "EUR", 1400, 800},
		{"NO", "CASH_EQUIVALENT", "NOK", 20000, 5000},
		{"GB", "CASH_EQUIVALENT", "GBP", 1500, 2000},
		{"FR", "SURETY", "EUR", 1000, 1200},
		{"ES", "CASH_EQUIVALENT", "EUR", 900, 900},
	}
	for _, tc := range cases {
		t.Run(tc.country, func(t *testing.T) {
			d := minDeposit(tc.country, tc.depositType, tc.currency, tc.amount)
			if tc.country == "CH" {
				d.Trusteeship = &models.Trusteeship{InterestRate: 0.5, InsolvencyProtectionConfirmed: true}
			}
			r := validator.Validate(d, tc.monthly)
			for _, f := range r.Findings {
				if f.Severity == validator.SeverityError {
					t.Errorf("ERROR in %s: [%s] %s", tc.country, f.Code, f.Message)
				}
			}
		})
	}
}
