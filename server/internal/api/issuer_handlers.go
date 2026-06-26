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

// OpenID4VCI credential issuance endpoints for the DepositPledgeAttestation QEAA.
//
// Flow (Pre-Authorized Code, RFC 9396):
//   Bank → POST /v1/deposits/{id}/issue-credential
//        ← { credential_offer_url, session_id, expires_at }
//
//   Wallet → GET /v1/credential-offers/{sessionId}
//          ← CredentialOffer { pre-authorized_code }
//   Wallet → POST /v1/token  (pre-authorized_code grant)
//          ← { access_token, c_nonce }
//   Wallet → POST /v1/credential  (Bearer access_token)
//          ← { credential: "<sd-jwt>" }
//
//   Verifier/Relying-Party → GET /v1/credentials/{credentialId}/status
//          ← { status: "active"|"revoked" }

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/xmiete/server/internal/db"
	"github.com/xmiete/server/internal/issuance"
)

// ── Bank trigger ─────────────────────────────────────────────────────────────

type issueCredentialRequest struct {
	ValidUntil string `json:"valid_until,omitempty"` // ISO 8601 date, pledge end date
}

type issueCredentialResponse struct {
	SessionID          string    `json:"session_id"`
	CredentialOfferURL string    `json:"credential_offer_url"`
	QRCodePayload      string    `json:"qr_code_payload"` // identical to CredentialOfferURL, for QR display
	ExpiresAt          time.Time `json:"expires_at"`
}

// POST /v1/deposits/{id}/issue-credential
// Called by the bank after pledge confirmation to initiate QEAA issuance.
func (s *Server) IssueCredential(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	var req issueCredentialRequest
	json.NewDecoder(r.Body).Decode(&req) //nolint:errcheck

	deposit, err := s.repo.GetByID(r.Context(), id)
	if errors.Is(err, db.ErrNotFound) {
		writeError(w, http.StatusNotFound, "deposit not found", "NOT_FOUND")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to fetch deposit", "INTERNAL_ERROR")
		return
	}

	if deposit.Deposit.LifecycleState != "PLEDGED" {
		writeError(w, http.StatusConflict,
			"credential issuance requires deposit in PLEDGED state",
			"INVALID_STATE")
		return
	}

	idx, err := s.allocator.AllocateIndex(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to allocate status list index", "INTERNAL_ERROR")
		return
	}

	sess, err := s.sessions.Create(r.Context(), id, req.ValidUntil, idx)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create issuance session", "INTERNAL_ERROR")
		return
	}
	offerURI := fmt.Sprintf("%s/v1/credential-offers/%s", s.issuerURL, sess.ID)
	offerURL := "openid-credential-offer://?credential_offer_uri=" + offerURI

	writeJSON(w, http.StatusCreated, issueCredentialResponse{
		SessionID:          sess.ID,
		CredentialOfferURL: offerURL,
		QRCodePayload:      offerURL,
		ExpiresAt:          sess.ExpiresAt,
	})
}

// ── OID4VCI endpoints ─────────────────────────────────────────────────────────

// GET /v1/credential-offers/{sessionId}
// Wallet fetches the credential offer to learn the pre-authorized_code grant.
func (s *Server) GetCredentialOffer(w http.ResponseWriter, r *http.Request) {
	sessionID := chi.URLParam(r, "sessionId")
	sess, ok := s.sessions.GetByID(r.Context(), sessionID)
	if !ok {
		writeError(w, http.StatusNotFound, "offer not found or expired", "NOT_FOUND")
		return
	}
	if time.Now().After(sess.ExpiresAt) {
		writeError(w, http.StatusGone, "offer has expired", "OFFER_EXPIRED")
		return
	}

	type txCode struct {
		InputMode   string `json:"input_mode"`
		Description string `json:"description"`
	}
	type preAuthGrant struct {
		PreAuthorizedCode string `json:"pre-authorized_code"`
	}
	type grants struct {
		PreAuth preAuthGrant `json:"urn:ietf:params:oauth:grant-type:pre-authorized_code"`
	}
	type credentialOffer struct {
		CredentialIssuer               string   `json:"credential_issuer"`
		CredentialConfigurationIDs     []string `json:"credential_configuration_ids"`
		Grants                         grants   `json:"grants"`
	}

	offer := credentialOffer{
		CredentialIssuer:           s.issuerURL,
		CredentialConfigurationIDs: []string{"DepositPledgeAttestation"},
		Grants: grants{
			PreAuth: preAuthGrant{PreAuthorizedCode: sess.PreAuthorizedCode},
		},
	}
	writeJSON(w, http.StatusOK, offer)
}

// POST /v1/token
// Wallet exchanges the pre-authorized_code for an access token (c_nonce included).
func (s *Server) Token(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		writeError(w, http.StatusBadRequest, "invalid form body", "BAD_REQUEST")
		return
	}

	grantType := r.FormValue("grant_type")
	const preAuthGrant = "urn:ietf:params:oauth:grant-type:pre-authorized_code"
	if grantType != preAuthGrant {
		writeError(w, http.StatusBadRequest,
			"unsupported grant_type; only pre-authorized_code is supported",
			"UNSUPPORTED_GRANT_TYPE")
		return
	}

	code := r.FormValue("pre-authorized_code")
	if code == "" {
		writeError(w, http.StatusBadRequest, "pre-authorized_code is required", "INVALID_REQUEST")
		return
	}

	accessToken, nonce, ok := s.sessions.ExchangeCodeForToken(r.Context(), code)
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid, expired, or already-used pre-authorized_code", "INVALID_GRANT")
		return
	}

	type tokenResponse struct {
		AccessToken     string `json:"access_token"`
		TokenType       string `json:"token_type"`
		ExpiresIn       int    `json:"expires_in"`
		CNonce          string `json:"c_nonce"`
		CNonceExpiresIn int    `json:"c_nonce_expires_in"`
	}
	writeJSON(w, http.StatusOK, tokenResponse{
		AccessToken:     accessToken,
		TokenType:       "Bearer",
		ExpiresIn:       300,
		CNonce:          nonce,
		CNonceExpiresIn: 300,
	})
}

// POST /v1/credential
// Wallet presents its access token and proof-of-possession; server issues the SD-JWT.
func (s *Server) Credential(w http.ResponseWriter, r *http.Request) {
	authHeader := r.Header.Get("Authorization")
	if !strings.HasPrefix(authHeader, "Bearer ") {
		writeError(w, http.StatusUnauthorized, "Bearer token required", "UNAUTHORIZED")
		return
	}
	token := strings.TrimPrefix(authHeader, "Bearer ")

	sess, ok := s.sessions.GetByToken(r.Context(), token)
	if !ok || time.Now().After(sess.ExpiresAt) {
		writeError(w, http.StatusUnauthorized, "invalid or expired access token", "INVALID_TOKEN")
		return
	}

	type credentialRequest struct {
		Format string `json:"format"`
		VCT    string `json:"vct"`
		Proof  *struct {
			ProofType string `json:"proof_type"`
			JWT       string `json:"jwt"`
		} `json:"proof,omitempty"`
	}
	var req credentialRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON", "BAD_REQUEST")
		return
	}
	if req.Format != "vc+sd-jwt" || req.VCT != "DepositPledgeAttestation" {
		writeError(w, http.StatusBadRequest,
			"unsupported credential format or type; use format=vc+sd-jwt vct=DepositPledgeAttestation",
			"UNSUPPORTED_CREDENTIAL_TYPE")
		return
	}

	deposit, err := s.repo.GetByID(r.Context(), sess.DepositID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load deposit", "INTERNAL_ERROR")
		return
	}

	sdJWT, credentialID, err := issuance.BuildSDJWT(s.issuerURL, deposit, sess.ValidUntil, sess.StatusListIndex)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "credential signing failed", "SIGNING_ERROR")
		return
	}

	_, ok = s.sessions.ConsumeByToken(r.Context(), token, credentialID)
	if !ok {
		// Token was valid a moment ago but session moved concurrently — rare race.
		writeError(w, http.StatusConflict, "session already consumed", "SESSION_CONSUMED")
		return
	}

	type credentialResponse struct {
		Credential      string `json:"credential"`
		CNonce          string `json:"c_nonce,omitempty"`
		CNonceExpiresIn int    `json:"c_nonce_expires_in,omitempty"`
	}
	writeJSON(w, http.StatusOK, credentialResponse{
		Credential: sdJWT,
	})
}

// ── Status / Revocation ───────────────────────────────────────────────────────

// GET /v1/credentials/{credentialId}/status
// Public endpoint — verifiers poll this to check whether a QEAA is still valid.
func (s *Server) CredentialStatus(w http.ResponseWriter, r *http.Request) {
	credID := chi.URLParam(r, "credentialId")
	status, found := s.sessions.CredentialStatus(r.Context(), credID)
	if !found {
		writeError(w, http.StatusNotFound, "credential not found", "NOT_FOUND")
		return
	}
	type statusResponse struct {
		CredentialID string `json:"credential_id"`
		Status       string `json:"status"`
		CheckedAt    string `json:"checked_at"`
	}
	writeJSON(w, http.StatusOK, statusResponse{
		CredentialID: credID,
		Status:       status,
		CheckedAt:    time.Now().UTC().Format(time.RFC3339),
	})
}

// ── W3C Bitstring Status List ─────────────────────────────────────────────────

// GET /v1/status-list/revocation
// Returns the W3C BitstringStatusListCredential as a signed JWT.
// Verifiers fetch this once and check credential bits locally instead of polling per-credential.
func (s *Server) StatusList(w http.ResponseWriter, r *http.Request) {
	indices, err := s.sessions.RevokedStatusListIndices(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to build status list", "INTERNAL_ERROR")
		return
	}

	signed, err := issuance.BuildStatusListJWT(s.issuerURL, indices)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to sign status list", "INTERNAL_ERROR")
		return
	}

	type statusListResponse struct {
		ID         string `json:"id"`
		Type       string `json:"type"`
		Credential string `json:"credential"`
	}
	writeJSON(w, http.StatusOK, statusListResponse{
		ID:         s.issuerURL + "/v1/status-list/revocation",
		Type:       "BitstringStatusListCredential",
		Credential: signed,
	})
}

// ── Well-Known metadata ───────────────────────────────────────────────────────

// GET /.well-known/openid-credential-issuer
// Wallet discovery: credential types, formats, endpoints.
func (s *Server) IssuerMetadata(w http.ResponseWriter, r *http.Request) {
	type display struct {
		Name        string `json:"name"`
		Locale      string `json:"locale"`
		Description string `json:"description,omitempty"`
	}
	type claimMeta struct {
		Display   []display `json:"display"`
		Mandatory bool      `json:"mandatory"`
		SD        bool      `json:"sd"` // selectively disclosable
	}
	type proofAlgs struct {
		ProofSigningAlgValuesSupported []string `json:"proof_signing_alg_values_supported"`
	}
	type credConfig struct {
		Format                             string               `json:"format"`
		VCT                                string               `json:"vct"`
		Scope                              string               `json:"scope"`
		CryptographicBindingMethods        []string             `json:"cryptographic_binding_methods_supported"`
		CredentialSigningAlgValues         []string             `json:"credential_signing_alg_values_supported"`
		ProofTypesSupported                map[string]proofAlgs `json:"proof_types_supported"`
		Display                            []display            `json:"display"`
		Claims                             map[string]claimMeta `json:"claims"`
	}
	type metadata struct {
		CredentialIssuer                  string                `json:"credential_issuer"`
		CredentialEndpoint                string                `json:"credential_endpoint"`
		TokenEndpoint                     string                `json:"token_endpoint"`
		JWKsURI                           string                `json:"jwks_uri"`
		CredentialConfigurationsSupported map[string]credConfig `json:"credential_configurations_supported"`
	}

	base := s.issuerURL
	meta := metadata{
		CredentialIssuer:   base,
		CredentialEndpoint: base + "/v1/credential",
		TokenEndpoint:      base + "/v1/token",
		JWKsURI:            base + "/.well-known/jwks.json",
		CredentialConfigurationsSupported: map[string]credConfig{
			"DepositPledgeAttestation": {
				Format:                      "vc+sd-jwt",
				VCT:                         "DepositPledgeAttestation",
				Scope:                       "DepositPledgeAttestation",
				CryptographicBindingMethods: []string{"did:key", "jwk"},
				CredentialSigningAlgValues:  []string{"ES256"},
				ProofTypesSupported: map[string]proofAlgs{
					"jwt": {ProofSigningAlgValuesSupported: []string{"ES256", "RS256"}},
				},
				Display: []display{
					{
						Name:        "Deposit Pledge Attestation",
						Locale:      "de-DE",
						Description: "Bestätigung einer rechtssicheren Mietkautionsverpfändung nach BGB § 551",
					},
					{
						Name:        "Deposit Pledge Attestation",
						Locale:      "en-US",
						Description: "Confirmation of a legally binding rental deposit pledge under BGB § 551",
					},
				},
				Claims: map[string]claimMeta{
					"deposit_amount":   {Display: []display{{Name: "Kautionsbetrag", Locale: "de-DE"}}, Mandatory: true, SD: true},
					"currency":         {Display: []display{{Name: "Währung", Locale: "de-DE"}}, Mandatory: true, SD: true},
					"pledge_date":      {Display: []display{{Name: "Verpfändungsdatum", Locale: "de-DE"}}, Mandatory: true, SD: false},
					"pledged_until":    {Display: []display{{Name: "Verpfändet bis", Locale: "de-DE"}}, Mandatory: false, SD: true},
					"statutory_basis":  {Display: []display{{Name: "Rechtsgrundlage", Locale: "de-DE"}}, Mandatory: true, SD: false},
					"issuing_bank":     {Display: []display{{Name: "Ausstellende Bank", Locale: "de-DE"}}, Mandatory: true, SD: false},
					"property_address": {Display: []display{{Name: "Mietobjekt-Adresse", Locale: "de-DE"}}, Mandatory: false, SD: true},
					"tenant_first_name": {Display: []display{{Name: "Vorname Mieter", Locale: "de-DE"}}, Mandatory: false, SD: true},
					"tenant_last_name":  {Display: []display{{Name: "Nachname Mieter", Locale: "de-DE"}}, Mandatory: false, SD: true},
				},
			},
		},
	}
	writeJSON(w, http.StatusOK, meta)
}

// GET /.well-known/jwks.json
// Publishes the issuer's public key so verifiers can check SD-JWT signatures.
func (s *Server) JWKS(w http.ResponseWriter, r *http.Request) {
	pub := issuance.DefaultSigner.PublicKey()

	// Encode EC public key as JWK (P-256)
	encodeCoord := func(b *big.Int) string {
		bs := b.Bytes()
		padded := make([]byte, (pub.Curve.Params().BitSize+7)/8)
		copy(padded[len(padded)-len(bs):], bs)
		return base64.RawURLEncoding.EncodeToString(padded)
	}

	type jwk struct {
		KTY string `json:"kty"`
		CRV string `json:"crv"`
		KID string `json:"kid"`
		Use string `json:"use"`
		Alg string `json:"alg"`
		X   string `json:"x"`
		Y   string `json:"y"`
	}
	type jwks struct {
		Keys []jwk `json:"keys"`
	}

	writeJSON(w, http.StatusOK, jwks{
		Keys: []jwk{
			{
				KTY: "EC",
				CRV: "P-256",
				KID: issuance.DefaultSigner.KeyID,
				Use: "sig",
				Alg: "ES256",
				X:   encodeCoord(pub.X),
				Y:   encodeCoord(pub.Y),
			},
		},
	})
}
