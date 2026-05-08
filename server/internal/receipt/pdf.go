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
package receipt

import (
	"bytes"
	"fmt"
	"time"

	"github.com/go-pdf/fpdf"
	"github.com/xmiete/server/internal/models"
)

// Generate produces a PDF receipt (Kautionsquittung) for a pledged deposit.
// This is the fallback delivery path for tenants who do not yet have an EUDI wallet.
// The QEAA credential (DepositPledgeAttestation) remains the authoritative artifact
// when issued; this PDF carries the same information in human-readable form.
func Generate(d *models.Deposit) ([]byte, error) {
	f := fpdf.New("P", "mm", "A4", "")
	f.SetMargins(20, 20, 20)
	f.AddPage()

	pageW, _ := f.GetPageSize()
	contentW := pageW - 40

	// Header
	f.SetFont("Helvetica", "B", 18)
	f.CellFormat(contentW, 10, "Rental Deposit Receipt", "", 1, "C", false, 0, "")
	f.SetFont("Helvetica", "", 10)
	f.CellFormat(contentW, 6, "Kautionsquittung gemäß BGB § 551", "", 1, "C", false, 0, "")
	f.Ln(6)

	// Divider
	f.SetDrawColor(180, 180, 180)
	f.Line(20, f.GetY(), pageW-20, f.GetY())
	f.Ln(6)

	// Reference block
	f.SetFont("Helvetica", "", 9)
	f.SetTextColor(100, 100, 100)
	f.CellFormat(contentW/2, 5, fmt.Sprintf("Deposit ID: %s", d.ID), "", 0, "L", false, 0, "")
	f.CellFormat(contentW/2, 5, fmt.Sprintf("Issued: %s", time.Now().UTC().Format("2006-01-02")), "", 1, "R", false, 0, "")
	f.SetTextColor(0, 0, 0)
	f.Ln(6)

	// Two-column: tenant | property
	colW := contentW / 2

	startY := f.GetY()
	f.SetFont("Helvetica", "B", 10)
	f.CellFormat(colW, 6, "TENANT", "", 1, "L", false, 0, "")
	f.SetFont("Helvetica", "", 10)
	f.CellFormat(colW, 5, fmt.Sprintf("%s %s", d.Tenant.FirstName, d.Tenant.LastName), "", 1, "L", false, 0, "")
	f.CellFormat(colW, 5, d.Tenant.Email, "", 1, "L", false, 0, "")
	if d.Tenant.Address != nil {
		f.CellFormat(colW, 5, d.Tenant.Address.Street, "", 1, "L", false, 0, "")
		f.CellFormat(colW, 5, fmt.Sprintf("%s %s", d.Tenant.Address.ZIP, d.Tenant.Address.City), "", 1, "L", false, 0, "")
	}
	tenantEndY := f.GetY()

	f.SetXY(pageW/2, startY)
	f.SetFont("Helvetica", "B", 10)
	f.CellFormat(colW, 6, "PROPERTY", "", 1, "L", false, 0, "")
	f.SetFont("Helvetica", "", 10)
	f.CellFormat(colW, 5, d.Property.Address.Street, "", 1, "L", false, 0, "")
	f.CellFormat(colW, 5, fmt.Sprintf("%s %s", d.Property.Address.ZIP, d.Property.Address.City), "", 1, "L", false, 0, "")
	if d.Property.UnitID != "" {
		f.CellFormat(colW, 5, fmt.Sprintf("Unit: %s", d.Property.UnitID), "", 1, "L", false, 0, "")
	}

	f.SetXY(20, max(tenantEndY, f.GetY()))
	f.Ln(4)

	f.SetFont("Helvetica", "B", 10)
	f.CellFormat(contentW, 6, "LANDLORD", "", 1, "L", false, 0, "")
	f.SetFont("Helvetica", "", 10)
	f.CellFormat(contentW, 5, d.Landlord.Name, "", 1, "L", false, 0, "")
	f.Ln(6)

	// Divider
	f.Line(20, f.GetY(), pageW-20, f.GetY())
	f.Ln(6)

	// Deposit details
	f.SetFont("Helvetica", "B", 10)
	f.CellFormat(contentW, 6, "DEPOSIT DETAILS", "", 1, "L", false, 0, "")
	f.SetFont("Helvetica", "", 10)

	labelW := 60.0
	valueW := contentW - labelW

	row := func(label, value string) {
		f.CellFormat(labelW, 5, label, "", 0, "L", false, 0, "")
		f.CellFormat(valueW, 5, value, "", 1, "L", false, 0, "")
	}

	row("Amount:", fmt.Sprintf("%.2f %s", d.Deposit.Amount, d.Deposit.Currency))
	row("Type:", formatDepositType(d.Deposit.Type))

	if d.Pledge != nil {
		row("Pledge Date:", d.Pledge.PledgeDate)
		row("Statutory Basis:", d.Pledge.StatutoryBasis)
		confirmed := "No"
		if d.Pledge.IsConfirmedByBank {
			confirmed = "Yes"
		}
		row("Confirmed by Bank:", confirmed)
	}

	if d.Trusteeship != nil {
		f.Ln(4)
		f.SetFont("Helvetica", "B", 10)
		f.CellFormat(contentW, 6, "TRUST ACCOUNT (BGB § 551 Abs. 3)", "", 1, "L", false, 0, "")
		f.SetFont("Helvetica", "", 10)
		if d.Trusteeship.TrusteeEntity != "" {
			row("Trustee:", d.Trusteeship.TrusteeEntity)
		}
		if d.Trusteeship.TrustAccountIBAN != "" {
			row("Trust IBAN:", d.Trusteeship.TrustAccountIBAN)
		}
		if d.Trusteeship.InsolvencyProtectionConfirmed {
			protection := "Confirmed"
			if d.Trusteeship.InsolvencyProtectionConfirmedDate != "" {
				protection += " " + d.Trusteeship.InsolvencyProtectionConfirmedDate
			}
			row("Insolvency Protection:", protection)
		}
		if d.Trusteeship.AccruedInterest > 0 {
			row("Accrued Interest:", fmt.Sprintf("%.2f %s", d.Trusteeship.AccruedInterest, d.Deposit.Currency))
		}
	}

	f.Ln(10)
	f.Line(20, f.GetY(), pageW-20, f.GetY())
	f.Ln(4)

	// Footer
	f.SetFont("Helvetica", "I", 8)
	f.SetTextColor(120, 120, 120)
	f.MultiCell(contentW, 4,
		"This receipt was generated automatically by XMiete. "+
			"Where the tenant holds a compatible EUDI Wallet, a Qualified Electronic Attestation of Attributes (QEAA) "+
			"has been or will be issued as the tamper-evident digital credential for this deposit.",
		"", "L", false)
	f.Ln(2)
	f.CellFormat(contentW, 4,
		fmt.Sprintf("Generated %s UTC", time.Now().UTC().Format("2006-01-02 15:04:05")),
		"", 1, "R", false, 0, "")

	var buf bytes.Buffer
	if err := f.Output(&buf); err != nil {
		return nil, fmt.Errorf("pdf output: %w", err)
	}
	return buf.Bytes(), nil
}

// GenerateReleaseReceipt produces a PDF release confirmation (Kautionsfreigabe) for a released deposit.
func GenerateReleaseReceipt(d *models.Deposit) ([]byte, error) {
	releaseDate := releasedAt(d)

	f := fpdf.New("P", "mm", "A4", "")
	f.SetMargins(20, 20, 20)
	f.AddPage()

	pageW, _ := f.GetPageSize()
	contentW := pageW - 40

	// Header
	f.SetFont("Helvetica", "B", 18)
	f.CellFormat(contentW, 10, "Deposit Release Confirmation", "", 1, "C", false, 0, "")
	f.SetFont("Helvetica", "", 10)
	f.CellFormat(contentW, 6, "Kautionsfreigabe gemäß BGB § 551", "", 1, "C", false, 0, "")
	f.Ln(6)

	// Divider
	f.SetDrawColor(180, 180, 180)
	f.Line(20, f.GetY(), pageW-20, f.GetY())
	f.Ln(6)

	// Reference block
	f.SetFont("Helvetica", "", 9)
	f.SetTextColor(100, 100, 100)
	f.CellFormat(contentW/2, 5, fmt.Sprintf("Deposit ID: %s", d.ID), "", 0, "L", false, 0, "")
	f.CellFormat(contentW/2, 5, fmt.Sprintf("Released: %s", releaseDate), "", 1, "R", false, 0, "")
	f.SetTextColor(0, 0, 0)
	f.Ln(6)

	// Two-column: tenant | property
	colW := contentW / 2

	startY := f.GetY()
	f.SetFont("Helvetica", "B", 10)
	f.CellFormat(colW, 6, "TENANT", "", 1, "L", false, 0, "")
	f.SetFont("Helvetica", "", 10)
	f.CellFormat(colW, 5, fmt.Sprintf("%s %s", d.Tenant.FirstName, d.Tenant.LastName), "", 1, "L", false, 0, "")
	f.CellFormat(colW, 5, d.Tenant.Email, "", 1, "L", false, 0, "")
	if d.Tenant.Address != nil {
		f.CellFormat(colW, 5, d.Tenant.Address.Street, "", 1, "L", false, 0, "")
		f.CellFormat(colW, 5, fmt.Sprintf("%s %s", d.Tenant.Address.ZIP, d.Tenant.Address.City), "", 1, "L", false, 0, "")
	}
	tenantEndY := f.GetY()

	f.SetXY(pageW/2, startY)
	f.SetFont("Helvetica", "B", 10)
	f.CellFormat(colW, 6, "PROPERTY", "", 1, "L", false, 0, "")
	f.SetFont("Helvetica", "", 10)
	f.CellFormat(colW, 5, d.Property.Address.Street, "", 1, "L", false, 0, "")
	f.CellFormat(colW, 5, fmt.Sprintf("%s %s", d.Property.Address.ZIP, d.Property.Address.City), "", 1, "L", false, 0, "")
	if d.Property.UnitID != "" {
		f.CellFormat(colW, 5, fmt.Sprintf("Unit: %s", d.Property.UnitID), "", 1, "L", false, 0, "")
	}

	f.SetXY(20, max(tenantEndY, f.GetY()))
	f.Ln(4)

	f.SetFont("Helvetica", "B", 10)
	f.CellFormat(contentW, 6, "LANDLORD", "", 1, "L", false, 0, "")
	f.SetFont("Helvetica", "", 10)
	f.CellFormat(contentW, 5, d.Landlord.Name, "", 1, "L", false, 0, "")
	f.Ln(6)

	// Divider
	f.Line(20, f.GetY(), pageW-20, f.GetY())
	f.Ln(6)

	labelW := 60.0
	valueW := contentW - labelW

	row := func(label, value string) {
		f.CellFormat(labelW, 5, label, "", 0, "L", false, 0, "")
		f.CellFormat(valueW, 5, value, "", 1, "L", false, 0, "")
	}

	// Original deposit summary
	f.SetFont("Helvetica", "B", 10)
	f.CellFormat(contentW, 6, "ORIGINAL DEPOSIT", "", 1, "L", false, 0, "")
	f.SetFont("Helvetica", "", 10)
	row("Amount:", fmt.Sprintf("%.2f %s", d.Deposit.Amount, d.Deposit.Currency))
	row("Type:", formatDepositType(d.Deposit.Type))
	if d.Pledge != nil && d.Pledge.PledgeDate != "" {
		row("Pledge Date:", d.Pledge.PledgeDate)
	}

	f.Ln(4)

	// Release details
	f.SetFont("Helvetica", "B", 10)
	f.CellFormat(contentW, 6, "RELEASE DETAILS", "", 1, "L", false, 0, "")
	f.SetFont("Helvetica", "", 10)
	row("Release Date:", releaseDate)
	row("Released By:", "Landlord")

	returned := d.Deposit.Amount
	if d.Trusteeship != nil && d.Trusteeship.TotalBalance > 0 {
		returned = d.Trusteeship.TotalBalance
		if d.Trusteeship.AccruedInterest > 0 {
			row("Principal:", fmt.Sprintf("%.2f %s", d.Deposit.Amount, d.Deposit.Currency))
			row("Accrued Interest:", fmt.Sprintf("%.2f %s", d.Trusteeship.AccruedInterest, d.Deposit.Currency))
		}
	}
	row("Total Returned:", fmt.Sprintf("%.2f %s", returned, d.Deposit.Currency))

	f.Ln(10)
	f.Line(20, f.GetY(), pageW-20, f.GetY())
	f.Ln(4)

	// Footer
	f.SetFont("Helvetica", "I", 8)
	f.SetTextColor(120, 120, 120)
	f.MultiCell(contentW, 4,
		"This release confirmation was generated automatically by XMiete. "+
			"It confirms that the rental deposit has been released by the landlord and the funds are to be returned to the tenant.",
		"", "L", false)
	f.Ln(2)
	f.CellFormat(contentW, 4,
		fmt.Sprintf("Generated %s UTC", time.Now().UTC().Format("2006-01-02 15:04:05")),
		"", 1, "R", false, 0, "")

	var buf bytes.Buffer
	if err := f.Output(&buf); err != nil {
		return nil, fmt.Errorf("pdf output: %w", err)
	}
	return buf.Bytes(), nil
}

// releasedAt extracts the release date from the deposit history, falling back to today.
func releasedAt(d *models.Deposit) string {
	for _, h := range d.Deposit.History {
		if h.State == models.StateReleased {
			return h.Timestamp.UTC().Format("2006-01-02")
		}
	}
	return time.Now().UTC().Format("2006-01-02")
}

func formatDepositType(t models.DepositType) string {
	switch t {
	case models.DepositTypeCash:
		return "Cash Equivalent (Barkaution)"
	case models.DepositTypeBankGuarantee:
		return "Bank Guarantee (Bankbürgschaft)"
	case models.DepositTypeInsurance:
		return "Insurance (Kautionsversicherung)"
	default:
		return string(t)
	}
}

func max(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}
