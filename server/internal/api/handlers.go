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
package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"log"

	"github.com/go-chi/chi/v5"
	"github.com/xmiete/server/internal/db"
	"github.com/xmiete/server/internal/issuance"
	"github.com/xmiete/server/internal/mailer"
	"github.com/xmiete/server/internal/models"
	"github.com/xmiete/server/internal/receipt"
	"github.com/xmiete/server/internal/statemachine"
	"github.com/xmiete/server/internal/verification"
)

type Server struct {
	repo       db.Repository
	webhookURL string // optional; POST state-change events here
	sessions   *issuance.Store
	vpSessions *verification.Store
	issuerURL  string // base URL for OID4VCI endpoints, e.g. https://api.xmiete.org
	mailer     mailer.Mailer
}

func NewServer(repo db.Repository, webhookURL, issuerURL string, m mailer.Mailer) *Server {
	return &Server{
		repo:       repo,
		webhookURL: webhookURL,
		sessions:   issuance.NewStore(),
		vpSessions: verification.NewStore(),
		issuerURL:  issuerURL,
		mailer:     m,
	}
}

// POST /deposits
func (s *Server) CreateDeposit(w http.ResponseWriter, r *http.Request) {
	var d models.Deposit
	if err := json.NewDecoder(r.Body).Decode(&d); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON", "BAD_REQUEST")
		return
	}
	if d.Tenant.FirstName == "" || d.Tenant.LastName == "" || d.Tenant.Email == "" {
		writeError(w, http.StatusBadRequest, "tenant first_name, last_name and email are required", "VALIDATION_ERROR")
		return
	}
	if d.Landlord.Name == "" {
		writeError(w, http.StatusBadRequest, "landlord name is required", "VALIDATION_ERROR")
		return
	}
	if d.Deposit.Amount <= 0 || d.Deposit.Type == "" {
		writeError(w, http.StatusBadRequest, "deposit amount and type are required", "VALIDATION_ERROR")
		return
	}
	if d.Meta.Version == "" {
		d.Meta.Version = "1.0.0"
	}
	if d.Deposit.Currency == "" {
		d.Deposit.Currency = "EUR"
	}

	created, err := s.repo.Create(r.Context(), &d)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create deposit", "INTERNAL_ERROR")
		return
	}

	writeJSON(w, http.StatusCreated, created)
}

// GET /deposits/{id}
func (s *Server) GetDeposit(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	d, err := s.repo.GetByID(r.Context(), id)
	if errors.Is(err, db.ErrNotFound) {
		writeError(w, http.StatusNotFound, "deposit not found", "NOT_FOUND")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to fetch deposit", "INTERNAL_ERROR")
		return
	}
	writeJSON(w, http.StatusOK, d)
}

// PATCH /deposits/{id}/identity
func (s *Server) UpdateIdentity(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	var req models.IdentityUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON", "BAD_REQUEST")
		return
	}
	if req.EIDStatus == "" {
		writeError(w, http.StatusBadRequest, "eid_status is required", "VALIDATION_ERROR")
		return
	}

	current, err := s.repo.GetByID(r.Context(), id)
	if errors.Is(err, db.ErrNotFound) {
		writeError(w, http.StatusNotFound, "deposit not found", "NOT_FOUND")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to fetch deposit", "INTERNAL_ERROR")
		return
	}

	nextState, transitions := statemachine.StateForIdentityUpdate(req.EIDStatus)
	if !transitions {
		writeJSON(w, http.StatusOK, current) // FAILED status — no state change
		return
	}

	if _, err := statemachine.Transition(current.Deposit.LifecycleState, nextState); err != nil {
		writeError(w, http.StatusConflict, err.Error(), "INVALID_TRANSITION")
		return
	}

	entry := models.HistoryEntry{State: nextState, Actor: "IDENTITY_PROVIDER"}
	updated, err := s.repo.UpdateState(r.Context(), id, nextState, entry, func(d *models.Deposit) {
		d.Tenant.EIDStatus = req.EIDStatus
		if req.WalletMetadata != nil {
			d.Tenant.WalletMetadata = req.WalletMetadata
		}
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update deposit", "INTERNAL_ERROR")
		return
	}

	s.fireWebhook(updated)
	writeJSON(w, http.StatusOK, updated)
}

// POST /deposits/{id}/pledge
func (s *Server) Pledge(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	var req models.PledgeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON", "BAD_REQUEST")
		return
	}
	if req.PledgeDate == "" || !req.IsConfirmedByBank {
		writeError(w, http.StatusBadRequest, "pledge_date and is_confirmed_by_bank=true are required", "VALIDATION_ERROR")
		return
	}

	current, err := s.repo.GetByID(r.Context(), id)
	if errors.Is(err, db.ErrNotFound) {
		writeError(w, http.StatusNotFound, "deposit not found", "NOT_FOUND")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to fetch deposit", "INTERNAL_ERROR")
		return
	}

	if _, err := statemachine.Transition(current.Deposit.LifecycleState, models.StatePledged); err != nil {
		writeError(w, http.StatusConflict, err.Error(), "INVALID_TRANSITION")
		return
	}

	entry := models.HistoryEntry{State: models.StatePledged, Actor: "BANK"}
	updated, err := s.repo.UpdateState(r.Context(), id, models.StatePledged, entry, func(d *models.Deposit) {
		d.Pledge = &models.Pledge{
			PledgeDate:        req.PledgeDate,
			StatutoryBasis:    "BGB § 551",
			IsConfirmedByBank: true,
		}
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update deposit", "INTERNAL_ERROR")
		return
	}

	s.fireWebhook(updated)
	s.sendReceiptEmail(updated)
	writeJSON(w, http.StatusOK, updated)
}

// POST /deposits/{id}/release
func (s *Server) Release(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	var req models.ReleaseRequest
	// body is optional per spec
	json.NewDecoder(r.Body).Decode(&req) //nolint:errcheck

	current, err := s.repo.GetByID(r.Context(), id)
	if errors.Is(err, db.ErrNotFound) {
		writeError(w, http.StatusNotFound, "deposit not found", "NOT_FOUND")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to fetch deposit", "INTERNAL_ERROR")
		return
	}

	if _, err := statemachine.Transition(current.Deposit.LifecycleState, models.StateReleased); err != nil {
		writeError(w, http.StatusConflict, err.Error(), "INVALID_TRANSITION")
		return
	}

	entry := models.HistoryEntry{State: models.StateReleased, Actor: "LANDLORD"}
	updated, err := s.repo.UpdateState(r.Context(), id, models.StateReleased, entry, nil)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update deposit", "INTERNAL_ERROR")
		return
	}

	s.fireWebhook(updated)
	s.sendReleaseReceiptEmail(updated)
	writeJSON(w, http.StatusOK, updated)
}

// POST /deposits/{id}/claim
func (s *Server) Claim(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	var req models.ClaimRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON", "BAD_REQUEST")
		return
	}
	if req.ClaimAmount <= 0 || req.Reason == "" {
		writeError(w, http.StatusBadRequest, "claim_amount and reason are required", "VALIDATION_ERROR")
		return
	}

	current, err := s.repo.GetByID(r.Context(), id)
	if errors.Is(err, db.ErrNotFound) {
		writeError(w, http.StatusNotFound, "deposit not found", "NOT_FOUND")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to fetch deposit", "INTERNAL_ERROR")
		return
	}

	if _, err := statemachine.Transition(current.Deposit.LifecycleState, models.StateClaimed); err != nil {
		writeError(w, http.StatusConflict, err.Error(), "INVALID_TRANSITION")
		return
	}

	entry := models.HistoryEntry{
		State:   models.StateClaimed,
		Actor:   "LANDLORD",
		Comment: req.Reason,
	}
	updated, err := s.repo.UpdateState(r.Context(), id, models.StateClaimed, entry, nil)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update deposit", "INTERNAL_ERROR")
		return
	}

	s.fireWebhook(updated)
	writeJSON(w, http.StatusOK, updated)
}

// GET /deposits/{id}/receipt
// Returns a PDF receipt (Kautionsquittung) for the deposit.
// Only available once the deposit is PLEDGED (is_confirmed_by_bank = true).
// PDF is generated on demand — serves as the fallback delivery path for
// tenants who do not yet have an EUDI wallet to receive the QEAA credential.
func (s *Server) GetReceipt(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	d, err := s.repo.GetByID(r.Context(), id)
	if errors.Is(err, db.ErrNotFound) {
		writeError(w, http.StatusNotFound, "deposit not found", "NOT_FOUND")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to fetch deposit", "INTERNAL_ERROR")
		return
	}

	switch d.Deposit.LifecycleState {
	case models.StatePledged, models.StateReleased, models.StateClaimed,
		models.StatePartiallyReleased, models.StateSettleProposed, models.StateDisputed, models.StateClosed:
		// receipt available
	default:
		writeError(w, http.StatusConflict,
			fmt.Sprintf("receipt not available in state %s — deposit must be PLEDGED first", d.Deposit.LifecycleState),
			"RECEIPT_NOT_AVAILABLE")
		return
	}

	pdfBytes, err := receipt.Generate(d)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to generate receipt", "INTERNAL_ERROR")
		return
	}

	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="receipt-%s.pdf"`, id))
	w.WriteHeader(http.StatusOK)
	w.Write(pdfBytes) //nolint:errcheck
}

// GET /deposits/{id}/release-receipt
// Returns a PDF release confirmation (Kautionsfreigabe) for the deposit.
// Only available once the deposit is RELEASED or CLOSED.
func (s *Server) GetReleaseReceipt(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	d, err := s.repo.GetByID(r.Context(), id)
	if errors.Is(err, db.ErrNotFound) {
		writeError(w, http.StatusNotFound, "deposit not found", "NOT_FOUND")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to fetch deposit", "INTERNAL_ERROR")
		return
	}

	switch d.Deposit.LifecycleState {
	case models.StateReleased, models.StateClosed:
		// release receipt available
	default:
		writeError(w, http.StatusConflict,
			fmt.Sprintf("release receipt not available in state %s — deposit must be RELEASED first", d.Deposit.LifecycleState),
			"RELEASE_RECEIPT_NOT_AVAILABLE")
		return
	}

	pdfBytes, err := receipt.GenerateReleaseReceipt(d)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to generate release receipt", "INTERNAL_ERROR")
		return
	}

	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="release-receipt-%s.pdf"`, id))
	w.WriteHeader(http.StatusOK)
	w.Write(pdfBytes) //nolint:errcheck
}

// sendReceiptEmail generates a PDF receipt and emails it to the tenant asynchronously.
func (s *Server) sendReceiptEmail(d *models.Deposit) {
	go func() {
		pdf, err := receipt.Generate(d)
		if err != nil {
			log.Printf("receipt generate deposit=%s: %v", d.ID, err)
			return
		}
		if err := s.mailer.SendReceipt(d, pdf); err != nil {
			log.Printf("receipt email deposit=%s to=%s: %v", d.ID, d.Tenant.Email, err)
		}
	}()
}

// sendReleaseReceiptEmail generates a PDF release confirmation and emails it to the tenant asynchronously.
func (s *Server) sendReleaseReceiptEmail(d *models.Deposit) {
	go func() {
		pdf, err := receipt.GenerateReleaseReceipt(d)
		if err != nil {
			log.Printf("release receipt generate deposit=%s: %v", d.ID, err)
			return
		}
		if err := s.mailer.SendReleaseReceipt(d, pdf); err != nil {
			log.Printf("release receipt email deposit=%s to=%s: %v", d.ID, d.Tenant.Email, err)
		}
	}()
}

// POST /deposits/{id}/settle
// Proposes (or counter-proposes) an itemized deposit split. Either party may initiate.
// From PLEDGED/FUNDED → SETTLE_PROPOSED (first proposal).
// From SETTLE_PROPOSED → counter-proposal updates the settlement in place (no state change).
func (s *Server) Settle(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	var req models.SettleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON", "BAD_REQUEST")
		return
	}
	if req.InitiatedBy != "LANDLORD" && req.InitiatedBy != "TENANT" {
		writeError(w, http.StatusBadRequest, "initiated_by must be LANDLORD or TENANT", "VALIDATION_ERROR")
		return
	}
	if len(req.ClaimItems) == 0 {
		writeError(w, http.StatusBadRequest, "claim_items must not be empty", "VALIDATION_ERROR")
		return
	}
	for _, item := range req.ClaimItems {
		if item.AmountClaimed <= 0 || item.Description == "" {
			writeError(w, http.StatusBadRequest, "each claim item requires description and amount_claimed > 0", "VALIDATION_ERROR")
			return
		}
	}

	current, err := s.repo.GetByID(r.Context(), id)
	if errors.Is(err, db.ErrNotFound) {
		writeError(w, http.StatusNotFound, "deposit not found", "NOT_FOUND")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to fetch deposit", "INTERNAL_ERROR")
		return
	}

	state := current.Deposit.LifecycleState
	isFirstProposal := state == models.StatePledged || state == models.StateFunded
	isCounter := state == models.StateSettleProposed

	if !isFirstProposal && !isCounter {
		writeError(w, http.StatusConflict,
			fmt.Sprintf("cannot propose settlement in state %s", state),
			"INVALID_TRANSITION")
		return
	}
	if isCounter && current.Settlement != nil && current.Settlement.LastProposedBy == req.InitiatedBy {
		writeError(w, http.StatusConflict, "cannot counter your own proposal — wait for the other party to respond", "CONFLICT")
		return
	}

	now := time.Now().UTC()
	totalClaimed := 0.0
	items := make([]models.ClaimItem, len(req.ClaimItems))
	for i, inp := range req.ClaimItems {
		totalClaimed += inp.AmountClaimed
		items[i] = models.ClaimItem{
			ID:            fmt.Sprintf("item-%d", i+1),
			Category:      inp.Category,
			Description:   inp.Description,
			AmountClaimed: inp.AmountClaimed,
			EvidenceRefs:  inp.EvidenceRefs,
			RoomOrArea:    inp.RoomOrArea,
		}
	}

	settlement := &models.Settlement{
		LastProposedBy:            req.InitiatedBy,
		LastProposedAt:            now,
		TenancyEndDate:            req.TenancyEndDate,
		HandoverDate:              req.HandoverDate,
		HandoverProtocolRef:       req.HandoverProtocolRef,
		ClaimItems:                items,
		TotalClaimed:              totalClaimed,
		ProposedTenantRefund:      req.ProposedTenantRefund,
		ProposedLandlordRetention: req.ProposedLandlordRetention,
		ResponseDeadline:          now.AddDate(0, 0, 14).Format("2006-01-02"),
	}
	if isFirstProposal {
		settlement.InitiatedBy = req.InitiatedBy
		settlement.InitiatedAt = now
	} else {
		settlement.InitiatedBy = current.Settlement.InitiatedBy
		settlement.InitiatedAt = current.Settlement.InitiatedAt
	}

	entry := models.HistoryEntry{State: models.StateSettleProposed, Actor: req.InitiatedBy}
	if isCounter {
		entry.Comment = "counter-proposal"
	}

	var updated *models.Deposit
	if isFirstProposal {
		if _, err := statemachine.Transition(state, models.StateSettleProposed); err != nil {
			writeError(w, http.StatusConflict, err.Error(), "INVALID_TRANSITION")
			return
		}
		updated, err = s.repo.UpdateState(r.Context(), id, models.StateSettleProposed, entry, func(d *models.Deposit) {
			d.Settlement = settlement
		})
	} else {
		// Counter-proposal: update settlement without a state transition.
		updated, err = s.repo.UpdateState(r.Context(), id, models.StateSettleProposed, entry, func(d *models.Deposit) {
			d.Settlement = settlement
		})
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update deposit", "INTERNAL_ERROR")
		return
	}

	s.fireWebhook(updated)
	writeJSON(w, http.StatusOK, updated)
}

// POST /deposits/{id}/settle/accept
// The non-proposing party accepts the current settlement proposal → CLOSED.
func (s *Server) SettleAccept(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	var req models.SettleAcceptRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON", "BAD_REQUEST")
		return
	}
	if req.AcceptedBy != "LANDLORD" && req.AcceptedBy != "TENANT" {
		writeError(w, http.StatusBadRequest, "accepted_by must be LANDLORD or TENANT", "VALIDATION_ERROR")
		return
	}

	current, err := s.repo.GetByID(r.Context(), id)
	if errors.Is(err, db.ErrNotFound) {
		writeError(w, http.StatusNotFound, "deposit not found", "NOT_FOUND")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to fetch deposit", "INTERNAL_ERROR")
		return
	}

	if current.Deposit.LifecycleState != models.StateSettleProposed {
		writeError(w, http.StatusConflict,
			fmt.Sprintf("cannot accept settlement in state %s", current.Deposit.LifecycleState),
			"INVALID_TRANSITION")
		return
	}
	if current.Settlement == nil {
		writeError(w, http.StatusConflict, "no active settlement proposal", "CONFLICT")
		return
	}
	if current.Settlement.LastProposedBy == req.AcceptedBy {
		writeError(w, http.StatusConflict, "cannot accept your own proposal", "CONFLICT")
		return
	}

	if _, err := statemachine.Transition(models.StateSettleProposed, models.StateClosed); err != nil {
		writeError(w, http.StatusConflict, err.Error(), "INVALID_TRANSITION")
		return
	}

	entry := models.HistoryEntry{State: models.StateClosed, Actor: req.AcceptedBy, Comment: "settlement accepted"}
	updated, err := s.repo.UpdateState(r.Context(), id, models.StateClosed, entry, func(d *models.Deposit) {
		d.Settlement.AgreedTenantRefund = d.Settlement.ProposedTenantRefund
		d.Settlement.AgreedLandlordRetention = d.Settlement.ProposedLandlordRetention
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update deposit", "INTERNAL_ERROR")
		return
	}

	s.fireWebhook(updated)
	writeJSON(w, http.StatusOK, updated)
}

// POST /deposits/{id}/dispute
// Escalates an unresolved settlement to an external authority (Schlichtungsbehörde, Amtsgericht, etc.).
func (s *Server) Dispute(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	var req models.DisputeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON", "BAD_REQUEST")
		return
	}
	if req.EscalatedBy != "LANDLORD" && req.EscalatedBy != "TENANT" {
		writeError(w, http.StatusBadRequest, "escalated_by must be LANDLORD or TENANT", "VALIDATION_ERROR")
		return
	}
	valid := map[string]bool{"PLATFORM_MEDIATION": true, "SCHLICHTUNGSBEHOERDE": true, "AMTSGERICHT": true}
	if !valid[req.EscalationType] {
		writeError(w, http.StatusBadRequest, "escalation_type must be PLATFORM_MEDIATION, SCHLICHTUNGSBEHOERDE, or AMTSGERICHT", "VALIDATION_ERROR")
		return
	}

	current, err := s.repo.GetByID(r.Context(), id)
	if errors.Is(err, db.ErrNotFound) {
		writeError(w, http.StatusNotFound, "deposit not found", "NOT_FOUND")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to fetch deposit", "INTERNAL_ERROR")
		return
	}

	if _, err := statemachine.Transition(current.Deposit.LifecycleState, models.StateDisputed); err != nil {
		writeError(w, http.StatusConflict, err.Error(), "INVALID_TRANSITION")
		return
	}

	entry := models.HistoryEntry{State: models.StateDisputed, Actor: req.EscalatedBy, Comment: req.EscalationType}
	updated, err := s.repo.UpdateState(r.Context(), id, models.StateDisputed, entry, func(d *models.Deposit) {
		if d.Settlement == nil {
			d.Settlement = &models.Settlement{}
		}
		d.Settlement.EscalationType = req.EscalationType
		d.Settlement.ExternalAuthority = req.EscalationType
		d.Settlement.ExternalReference = req.ExternalReference
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update deposit", "INTERNAL_ERROR")
		return
	}

	s.fireWebhook(updated)
	writeJSON(w, http.StatusOK, updated)
}

// POST /deposits/{id}/dispute/resolve
// Records the outcome of external dispute resolution → CLOSED.
func (s *Server) DisputeResolve(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	var req models.DisputeResolveRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON", "BAD_REQUEST")
		return
	}
	if req.AgreedTenantRefund < 0 || req.AgreedLandlordRetention < 0 {
		writeError(w, http.StatusBadRequest, "agreed amounts must be non-negative", "VALIDATION_ERROR")
		return
	}

	current, err := s.repo.GetByID(r.Context(), id)
	if errors.Is(err, db.ErrNotFound) {
		writeError(w, http.StatusNotFound, "deposit not found", "NOT_FOUND")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to fetch deposit", "INTERNAL_ERROR")
		return
	}

	if _, err := statemachine.Transition(current.Deposit.LifecycleState, models.StateClosed); err != nil {
		writeError(w, http.StatusConflict, err.Error(), "INVALID_TRANSITION")
		return
	}

	entry := models.HistoryEntry{State: models.StateClosed, Actor: "EXTERNAL", Comment: req.ResolutionReference}
	updated, err := s.repo.UpdateState(r.Context(), id, models.StateClosed, entry, func(d *models.Deposit) {
		if d.Settlement == nil {
			d.Settlement = &models.Settlement{}
		}
		d.Settlement.AgreedTenantRefund = req.AgreedTenantRefund
		d.Settlement.AgreedLandlordRetention = req.AgreedLandlordRetention
		d.Settlement.ExternalReference = req.ResolutionReference
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update deposit", "INTERNAL_ERROR")
		return
	}

	s.fireWebhook(updated)
	writeJSON(w, http.StatusOK, updated)
}

// POST /deposits/{id}/partial-release
// Landlord releases most of the deposit immediately and holds back a reservation
// while the utility reconciliation is pending. PLEDGED → PARTIALLY_RELEASED.
func (s *Server) PartialRelease(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	var req models.PartialReleaseRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON", "BAD_REQUEST")
		return
	}
	if req.ReleasedAmount < 0 || req.ReservedAmount <= 0 {
		writeError(w, http.StatusBadRequest, "released_amount must be ≥ 0 and reserved_amount must be > 0", "VALIDATION_ERROR")
		return
	}
	if req.BillingPeriodEnd == "" {
		writeError(w, http.StatusBadRequest, "billing_period_end is required", "VALIDATION_ERROR")
		return
	}
	billingEnd, err := time.Parse("2006-01-02", req.BillingPeriodEnd)
	if err != nil {
		writeError(w, http.StatusBadRequest, "billing_period_end must be ISO 8601 date (YYYY-MM-DD)", "VALIDATION_ERROR")
		return
	}

	current, err := s.repo.GetByID(r.Context(), id)
	if errors.Is(err, db.ErrNotFound) {
		writeError(w, http.StatusNotFound, "deposit not found", "NOT_FOUND")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to fetch deposit", "INTERNAL_ERROR")
		return
	}

	if _, err := statemachine.Transition(current.Deposit.LifecycleState, models.StatePartiallyReleased); err != nil {
		writeError(w, http.StatusConflict, err.Error(), "INVALID_TRANSITION")
		return
	}

	reservation := &models.UtilityReservation{
		ReleasedAmount:     req.ReleasedAmount,
		ReservedAmount:     req.ReservedAmount,
		MonthlyAdvance:     req.MonthlyAdvance,
		BillingPeriodEnd:   req.BillingPeriodEnd,
		ResolutionDeadline: billingEnd.AddDate(1, 0, 0).Format("2006-01-02"),
	}

	entry := models.HistoryEntry{State: models.StatePartiallyReleased, Actor: "LANDLORD"}
	updated, err := s.repo.UpdateState(r.Context(), id, models.StatePartiallyReleased, entry, func(d *models.Deposit) {
		d.UtilityReservation = reservation
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update deposit", "INTERNAL_ERROR")
		return
	}

	s.fireWebhook(updated)
	writeJSON(w, http.StatusOK, updated)
}

// POST /deposits/{id}/utility-settle
// Landlord submits the utility statement outcome once the reconciliation is complete.
// No shortfall (actual_cost ≤ total_advance_paid) → CLOSED (full reservation returned).
// Shortfall > 0 → SETTLE_PROPOSED with a pre-populated UTILITY_ARREARS claim so both
// parties can accept or dispute the figures before CLOSED.
func (s *Server) UtilitySettle(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	var req models.UtilitySettleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON", "BAD_REQUEST")
		return
	}
	if req.ActualCost < 0 || req.TotalAdvancePaid < 0 {
		writeError(w, http.StatusBadRequest, "actual_cost and total_advance_paid must be non-negative", "VALIDATION_ERROR")
		return
	}

	current, err := s.repo.GetByID(r.Context(), id)
	if errors.Is(err, db.ErrNotFound) {
		writeError(w, http.StatusNotFound, "deposit not found", "NOT_FOUND")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to fetch deposit", "INTERNAL_ERROR")
		return
	}

	if current.Deposit.LifecycleState != models.StatePartiallyReleased {
		writeError(w, http.StatusConflict,
			fmt.Sprintf("utility-settle requires state PARTIALLY_RELEASED, current state is %s", current.Deposit.LifecycleState),
			"INVALID_TRANSITION")
		return
	}
	if current.UtilityReservation == nil {
		writeError(w, http.StatusConflict, "no utility reservation on this deposit", "CONFLICT")
		return
	}

	reserved := current.UtilityReservation.ReservedAmount
	shortfall := req.ActualCost - req.TotalAdvancePaid
	now := time.Now().UTC()

	if shortfall <= 0 {
		// Tenant overpaid or broke even — return full reservation.
		if _, err := statemachine.Transition(models.StatePartiallyReleased, models.StateClosed); err != nil {
			writeError(w, http.StatusConflict, err.Error(), "INVALID_TRANSITION")
			return
		}
		entry := models.HistoryEntry{State: models.StateClosed, Actor: "LANDLORD", Comment: "utility reconciliation — no shortfall"}
		updated, err := s.repo.UpdateState(r.Context(), id, models.StateClosed, entry, func(d *models.Deposit) {
			d.UtilityReservation.SettlementRef = req.SettlementRef
			d.UtilityReservation.ActualCost = req.ActualCost
			d.UtilityReservation.TotalAdvancePaid = req.TotalAdvancePaid
			d.UtilityReservation.ResolvedAt = now.Format("2006-01-02")
		})
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to update deposit", "INTERNAL_ERROR")
			return
		}
		s.fireWebhook(updated)
		writeJSON(w, http.StatusOK, updated)
		return
	}

	// Shortfall exists — landlord retains up to the reserved amount; propose via settlement flow.
	retention := shortfall
	if retention > reserved {
		retention = reserved
	}
	refund := reserved - retention

	if _, err := statemachine.Transition(models.StatePartiallyReleased, models.StateSettleProposed); err != nil {
		writeError(w, http.StatusConflict, err.Error(), "INVALID_TRANSITION")
		return
	}

	settlement := &models.Settlement{
		InitiatedBy:               "LANDLORD",
		InitiatedAt:               now,
		LastProposedBy:            "LANDLORD",
		LastProposedAt:            now,
		ClaimItems: []models.ClaimItem{{
			ID:            "item-1",
			Category:      models.ClaimCategoryUtilityArrears,
			Description:   fmt.Sprintf("Utility shortfall: actual cost %.2f, advances paid %.2f", req.ActualCost, req.TotalAdvancePaid),
			AmountClaimed: shortfall,
			EvidenceRefs:  []string{req.SettlementRef},
		}},
		TotalClaimed:              shortfall,
		ProposedTenantRefund:      refund,
		ProposedLandlordRetention: retention,
		ResponseDeadline:          now.AddDate(0, 0, 14).Format("2006-01-02"),
	}
	if req.SettlementRef == "" {
		settlement.ClaimItems[0].EvidenceRefs = nil
	}

	entry := models.HistoryEntry{State: models.StateSettleProposed, Actor: "LANDLORD", Comment: "utility reconciliation shortfall"}
	updated, err := s.repo.UpdateState(r.Context(), id, models.StateSettleProposed, entry, func(d *models.Deposit) {
		d.UtilityReservation.SettlementRef = req.SettlementRef
		d.UtilityReservation.ActualCost = req.ActualCost
		d.UtilityReservation.TotalAdvancePaid = req.TotalAdvancePaid
		d.UtilityReservation.ResolvedAt = now.Format("2006-01-02")
		d.Settlement = settlement
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update deposit", "INTERNAL_ERROR")
		return
	}

	s.fireWebhook(updated)
	writeJSON(w, http.StatusOK, updated)
}

// fireWebhook posts a state-change event to s.webhookURL asynchronously.
func (s *Server) fireWebhook(d *models.Deposit) {
	if s.webhookURL == "" {
		return
	}
	event := models.WebhookEvent{
		EventType: "deposit.status_changed",
		DepositID: d.ID,
		NewState:  d.Deposit.LifecycleState,
		Timestamp: time.Now().UTC(),
	}
	body, err := json.Marshal(event)
	if err != nil {
		return
	}
	go func() {
		http.Post(s.webhookURL, "application/json", bytes.NewReader(body)) //nolint:errcheck,gosec
	}()
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v) //nolint:errcheck
}

func writeError(w http.ResponseWriter, status int, msg, code string) {
	writeJSON(w, status, models.ErrorResponse{Error: msg, Code: code})
}
