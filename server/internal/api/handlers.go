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
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/xmiete/server/internal/db"
	"github.com/xmiete/server/internal/issuance"
	"github.com/xmiete/server/internal/models"
	"github.com/xmiete/server/internal/statemachine"
	"github.com/xmiete/server/internal/verification"
)

type Server struct {
	repo       db.Repository
	webhookURL string // optional; POST state-change events here
	sessions   *issuance.Store
	vpSessions *verification.Store
	issuerURL  string // base URL for OID4VCI endpoints, e.g. https://api.xmiete.org
}

func NewServer(repo db.Repository, webhookURL, issuerURL string) *Server {
	return &Server{
		repo:       repo,
		webhookURL: webhookURL,
		sessions:   issuance.NewStore(),
		vpSessions: verification.NewStore(),
		issuerURL:  issuerURL,
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
