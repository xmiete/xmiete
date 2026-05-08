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

// OpenID4VP verifiable presentation endpoints.
//
// Flow:
//   Landlord → POST /v1/deposits/{id}/vp-request  (JWT-authenticated)
//            ← { vp_request, session_id, deep_link, expires_at }
//
//   Wallet   → POST /v1/deposits/{id}/vp-response  (no JWT — wallet self-authenticates via KB-JWT)
//              body: { vp_token, state, presentation_submission }
//            ← { verified_claims }

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/xmiete/server/internal/db"
	"github.com/xmiete/server/internal/issuance"
	"github.com/xmiete/server/internal/models"
	"github.com/xmiete/server/internal/verification"
)

// POST /v1/deposits/{id}/vp-request
// Called by the landlord's app to initiate a wallet presentation request.
func (s *Server) CreateVpRequest(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	_, err := s.repo.GetByID(r.Context(), id)
	if errors.Is(err, db.ErrNotFound) {
		writeError(w, http.StatusNotFound, "deposit not found", "NOT_FOUND")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to fetch deposit", "INTERNAL_ERROR")
		return
	}

	responseURI := s.issuerURL + "/v1/deposits/" + id + "/vp-response"
	sess := s.vpSessions.Create(id, responseURI)

	vpReq := verification.BuildVpRequest(s.issuerURL, responseURI, sess.Nonce, sess.ID)

	requestJSON, err := json.Marshal(vpReq)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to build vp_request", "INTERNAL_ERROR")
		return
	}

	deepLink := "openid4vp://?client_id=" + url.QueryEscape(s.issuerURL) +
		"&request=" + url.QueryEscape(string(requestJSON))

	type vpRequestResponse struct {
		SessionID  string                  `json:"session_id"`
		VpRequest  verification.VpRequest  `json:"vp_request"`
		DeepLink   string                  `json:"deep_link"`
		ExpiresAt  time.Time               `json:"expires_at"`
	}
	writeJSON(w, http.StatusCreated, vpRequestResponse{
		SessionID: sess.ID,
		VpRequest: vpReq,
		DeepLink:  deepLink,
		ExpiresAt: sess.ExpiresAt,
	})
}

// POST /v1/deposits/{id}/vp-response
// Wallet POSTs the SD-JWT VP token; server validates and records the verified credential.
func (s *Server) ReceiveVpResponse(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	var body struct {
		VpToken                string          `json:"vp_token"`
		State                  string          `json:"state"`
		PresentationSubmission json.RawMessage `json:"presentation_submission,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON", "BAD_REQUEST")
		return
	}
	if body.VpToken == "" {
		writeError(w, http.StatusBadRequest, "vp_token is required", "VALIDATION_ERROR")
		return
	}
	if body.State == "" {
		writeError(w, http.StatusBadRequest, "state is required", "VALIDATION_ERROR")
		return
	}

	sess, ok := s.vpSessions.GetByID(body.State)
	if !ok {
		writeError(w, http.StatusNotFound, "vp session not found or already consumed", "NOT_FOUND")
		return
	}
	if time.Now().After(sess.ExpiresAt) {
		writeError(w, http.StatusGone, "vp session has expired", "SESSION_EXPIRED")
		return
	}

	deposit, err := s.repo.GetByID(r.Context(), id)
	if errors.Is(err, db.ErrNotFound) {
		writeError(w, http.StatusNotFound, "deposit not found", "NOT_FOUND")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to fetch deposit", "INTERNAL_ERROR")
		return
	}

	if deposit.Deposit.LifecycleState != models.StatePledged {
		writeError(w, http.StatusConflict,
			"vp presentation requires deposit in PLEDGED state",
			"INVALID_STATE")
		return
	}

	pubKey := issuance.DefaultSigner.PublicKey()
	claims, err := verification.VerifyVpToken(body.VpToken, pubKey, sess.Nonce, sess.ResponseURI)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, err.Error(), "VP_VERIFICATION_FAILED")
		return
	}

	if claims.DepositID != id {
		writeError(w, http.StatusConflict,
			"credential deposit_id does not match requested deposit",
			"DEPOSIT_MISMATCH")
		return
	}

	now := time.Now().UTC()
	entry := models.HistoryEntry{
		State: deposit.Deposit.LifecycleState,
		Actor: "WALLET",
	}
	updated, err := s.repo.UpdateState(r.Context(), id, deposit.Deposit.LifecycleState, entry, func(d *models.Deposit) {
		d.Tenant.WalletMetadata = &models.WalletMetadata{
			PresentationID:  claims.CredentialID,
			AssuranceLevel:  "high",
			CredentialFormat: "vc+sd-jwt",
			VerifiedAt:      now.Format(time.RFC3339),
		}
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update deposit", "INTERNAL_ERROR")
		return
	}

	// Session is single-use; delete after successful verification.
	s.vpSessions.Delete(sess.ID)

	s.fireWebhook(updated)

	type vpResponseResult struct {
		VerifiedClaims *verification.VerifiedClaims `json:"verified_claims"`
	}
	writeJSON(w, http.StatusOK, vpResponseResult{VerifiedClaims: claims})
}
