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
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

// ── OID4VCI helpers ────────────────────────────────────────────────────────────

// issueCredential triggers QEAA issuance and returns the session_id.
func issueCredential(t *testing.T, ts *httptest.Server, tok, depositID string) string {
	t.Helper()
	var resp struct {
		SessionID string `json:"session_id"`
	}
	mustDoJSON(t,
		doJSON(t, ts, http.MethodPost, "/v1/deposits/"+depositID+"/issue-credential",
			map[string]any{"valid_until": "2027-06-26"}, tok),
		http.StatusCreated, &resp)
	if resp.SessionID == "" {
		t.Fatal("missing session_id in issue-credential response")
	}
	return resp.SessionID
}

// fetchPreAuthorizedCode retrieves the pre-authorized_code from the credential offer.
func fetchPreAuthorizedCode(t *testing.T, ts *httptest.Server, sessionID string) string {
	t.Helper()
	var offer struct {
		Grants struct {
			PreAuth struct {
				Code string `json:"pre-authorized_code"`
			} `json:"urn:ietf:params:oauth:grant-type:pre-authorized_code"`
		} `json:"grants"`
	}
	mustDoJSON(t,
		doJSON(t, ts, http.MethodGet, "/v1/credential-offers/"+sessionID, nil, ""),
		http.StatusOK, &offer)
	code := offer.Grants.PreAuth.Code
	if code == "" {
		t.Fatal("missing pre-authorized_code in credential offer")
	}
	return code
}

// exchangeCodeForToken exchanges the pre-authorized_code for an access token.
func exchangeCodeForToken(t *testing.T, ts *httptest.Server, code string) string {
	t.Helper()
	var tokenResp struct {
		AccessToken string `json:"access_token"`
	}
	mustDoJSON(t,
		doForm(t, ts, "/v1/token", url.Values{
			"grant_type":          {"urn:ietf:params:oauth:grant-type:pre-authorized_code"},
			"pre-authorized_code": {code},
		}),
		http.StatusOK, &tokenResp)
	if tokenResp.AccessToken == "" {
		t.Fatal("missing access_token in token response")
	}
	return tokenResp.AccessToken
}

// fetchCredential presents an access token and returns the SD-JWT credential string.
func fetchCredential(t *testing.T, ts *httptest.Server, accessToken string) string {
	t.Helper()
	var credResp struct {
		Credential string `json:"credential"`
	}
	mustDoJSON(t,
		doJSON(t, ts, http.MethodPost, "/v1/credential",
			map[string]any{"format": "vc+sd-jwt", "vct": "DepositPledgeAttestation"},
			accessToken),
		http.StatusOK, &credResp)
	if credResp.Credential == "" {
		t.Fatal("missing credential in response")
	}
	return credResp.Credential
}

// extractCredentialID decodes the JWT payload of an SD-JWT and returns the jti claim.
func extractCredentialID(t *testing.T, sdJWT string) string {
	t.Helper()
	jwtPart := strings.SplitN(sdJWT, "~", 2)[0]
	parts := strings.Split(jwtPart, ".")
	if len(parts) != 3 {
		t.Fatalf("invalid JWT structure in SD-JWT (got %d parts)", len(parts))
	}
	payloadJSON, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		t.Fatalf("decode SD-JWT payload: %v", err)
	}
	var claims struct {
		JTI string `json:"jti"`
	}
	if err := json.Unmarshal(payloadJSON, &claims); err != nil {
		t.Fatalf("unmarshal SD-JWT claims: %v", err)
	}
	if claims.JTI == "" {
		t.Fatal("missing jti in SD-JWT claims")
	}
	return claims.JTI
}

// getCredentialStatus fetches the credential status and returns the status string.
func getCredentialStatus(t *testing.T, ts *httptest.Server, credID string) string {
	t.Helper()
	var statusResp struct {
		Status string `json:"status"`
	}
	mustDoJSON(t,
		doJSON(t, ts, http.MethodGet, "/v1/credentials/"+credID+"/status", nil, ""),
		http.StatusOK, &statusResp)
	return statusResp.Status
}

// ── Tests ──────────────────────────────────────────────────────────────────────

// TestOID4VCI_HappyPath runs the full Pre-Authorized Code issuance flow:
// bank trigger → wallet offer → token exchange → credential → status check.
func TestOID4VCI_HappyPath(t *testing.T) {
	ts, tok := newTestServer(t)
	id := createAndPledge(t, ts, tok)

	sessionID := issueCredential(t, ts, tok, id)
	code := fetchPreAuthorizedCode(t, ts, sessionID)
	accessToken := exchangeCodeForToken(t, ts, code)
	sdJWT := fetchCredential(t, ts, accessToken)
	credID := extractCredentialID(t, sdJWT)

	status := getCredentialStatus(t, ts, credID)
	if status != "active" {
		t.Errorf("credential status: got %q, want %q", status, "active")
	}
}

// TestOID4VCI_CredentialOfferURL verifies the offer URL starts with the openid-credential-offer scheme.
func TestOID4VCI_CredentialOfferURL(t *testing.T) {
	ts, tok := newTestServer(t)
	id := createAndPledge(t, ts, tok)

	var offerResp struct {
		CredentialOfferURL string `json:"credential_offer_url"`
		QRCodePayload      string `json:"qr_code_payload"`
	}
	mustDoJSON(t,
		doJSON(t, ts, http.MethodPost, "/v1/deposits/"+id+"/issue-credential", nil, tok),
		http.StatusCreated, &offerResp)

	if !strings.HasPrefix(offerResp.CredentialOfferURL, "openid-credential-offer://") {
		t.Errorf("unexpected credential_offer_url: %s", offerResp.CredentialOfferURL)
	}
	if offerResp.QRCodePayload != offerResp.CredentialOfferURL {
		t.Errorf("qr_code_payload should equal credential_offer_url")
	}
}

// TestOID4VCI_NotPledgedRejected verifies that issue-credential is rejected unless the deposit is PLEDGED.
func TestOID4VCI_NotPledgedRejected(t *testing.T) {
	ts, tok := newTestServer(t)

	// REQUESTED state — should fail
	id := createDeposit(t, ts, tok, minDeposit("CASH_EQUIVALENT", "EUR", 1500.0))
	mustDo(t,
		doJSON(t, ts, http.MethodPost, "/v1/deposits/"+id+"/issue-credential", nil, tok),
		http.StatusConflict)

	// IDENTIFIED state — should fail
	mustDo(t,
		doJSON(t, ts, http.MethodPatch, "/v1/deposits/"+id+"/identity",
			map[string]any{"eid_status": "VERIFIED"}, tok),
		http.StatusOK)
	mustDo(t,
		doJSON(t, ts, http.MethodPost, "/v1/deposits/"+id+"/issue-credential", nil, tok),
		http.StatusConflict)
}

// TestOID4VCI_InvalidSessionID verifies that requesting a non-existent session returns 404.
func TestOID4VCI_InvalidSessionID(t *testing.T) {
	ts, _ := newTestServer(t)
	mustDo(t,
		doJSON(t, ts, http.MethodGet, "/v1/credential-offers/00000000-0000-0000-0000-000000000000", nil, ""),
		http.StatusNotFound)
}

// TestOID4VCI_InvalidGrantType verifies that unsupported grant types are rejected.
func TestOID4VCI_InvalidGrantType(t *testing.T) {
	ts, _ := newTestServer(t)
	mustDo(t,
		doForm(t, ts, "/v1/token", url.Values{
			"grant_type":   {"authorization_code"},
			"code":         {"some-code"},
		}),
		http.StatusBadRequest)
}

// TestOID4VCI_InvalidPreAuthorizedCode verifies that bad codes are rejected at the token endpoint.
func TestOID4VCI_InvalidPreAuthorizedCode(t *testing.T) {
	ts, _ := newTestServer(t)
	mustDo(t,
		doForm(t, ts, "/v1/token", url.Values{
			"grant_type":          {"urn:ietf:params:oauth:grant-type:pre-authorized_code"},
			"pre-authorized_code": {"invalid-code-that-does-not-exist"},
		}),
		http.StatusBadRequest)
}

// TestOID4VCI_InvalidBearerToken verifies that the credential endpoint rejects bad access tokens.
func TestOID4VCI_InvalidBearerToken(t *testing.T) {
	ts, _ := newTestServer(t)
	mustDo(t,
		doJSON(t, ts, http.MethodPost, "/v1/credential",
			map[string]any{"format": "vc+sd-jwt", "vct": "DepositPledgeAttestation"},
			"invalid-access-token"),
		http.StatusUnauthorized)
}

// TestOID4VCI_DoubleConsume verifies that the same access token cannot be used twice to issue a credential.
func TestOID4VCI_DoubleConsume(t *testing.T) {
	ts, tok := newTestServer(t)
	id := createAndPledge(t, ts, tok)

	code := fetchPreAuthorizedCode(t, ts, issueCredential(t, ts, tok, id))
	accessToken := exchangeCodeForToken(t, ts, code)

	// First issuance — succeeds
	fetchCredential(t, ts, accessToken)

	// Second issuance with the same token — must fail with 401 or 409
	resp := doJSON(t, ts, http.MethodPost, "/v1/credential",
		map[string]any{"format": "vc+sd-jwt", "vct": "DepositPledgeAttestation"},
		accessToken)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized && resp.StatusCode != http.StatusConflict {
		t.Errorf("expected 401 or 409 for double-consume, got %d", resp.StatusCode)
	}
}

// TestOID4VCI_RevocationOnRelease verifies that releasing a deposit revokes any issued credentials.
func TestOID4VCI_RevocationOnRelease(t *testing.T) {
	ts, tok := newTestServer(t)
	id := createAndPledge(t, ts, tok)

	// Issue credential
	code := fetchPreAuthorizedCode(t, ts, issueCredential(t, ts, tok, id))
	sdJWT := fetchCredential(t, ts, exchangeCodeForToken(t, ts, code))
	credID := extractCredentialID(t, sdJWT)

	if status := getCredentialStatus(t, ts, credID); status != "active" {
		t.Fatalf("expected active before release, got %q", status)
	}

	// Release deposit
	mustDo(t, doJSON(t, ts, http.MethodPost, "/v1/deposits/"+id+"/release", nil, tok), http.StatusOK)

	// Credential must now be revoked
	if status := getCredentialStatus(t, ts, credID); status != "revoked" {
		t.Errorf("credential status after release: got %q, want %q", status, "revoked")
	}
}

// TestOID4VCI_RevocationOnClose verifies that closing via settlement revokes credentials.
func TestOID4VCI_RevocationOnClose(t *testing.T) {
	ts, tok := newTestServer(t)
	id := createAndPledge(t, ts, tok)

	// Issue credential
	code := fetchPreAuthorizedCode(t, ts, issueCredential(t, ts, tok, id))
	sdJWT := fetchCredential(t, ts, exchangeCodeForToken(t, ts, code))
	credID := extractCredentialID(t, sdJWT)

	// Settle and close
	mustDo(t,
		doJSON(t, ts, http.MethodPost, "/v1/deposits/"+id+"/settle", map[string]any{
			"initiated_by": "LANDLORD",
			"claim_items":  []map[string]any{{"description": "Cleaning", "amount_claimed": 100.0, "category": "CLEANING"}},
			"proposed_tenant_refund": 1400.0, "proposed_landlord_retention": 100.0,
		}, tok),
		http.StatusOK)
	mustDo(t,
		doJSON(t, ts, http.MethodPost, "/v1/deposits/"+id+"/settle/accept",
			map[string]any{"accepted_by": "TENANT"}, tok),
		http.StatusOK)

	if status := getCredentialStatus(t, ts, credID); status != "revoked" {
		t.Errorf("credential status after close: got %q, want %q", status, "revoked")
	}
}

// TestOID4VCI_WellKnown verifies that the issuer metadata endpoint is accessible and well-formed.
func TestOID4VCI_WellKnown(t *testing.T) {
	ts, _ := newTestServer(t)
	var meta struct {
		CredentialIssuer               string         `json:"credential_issuer"`
		CredentialEndpoint             string         `json:"credential_endpoint"`
		TokenEndpoint                  string         `json:"token_endpoint"`
		CredentialConfigurationsSupported map[string]any `json:"credential_configurations_supported"`
	}
	mustDoJSON(t,
		doJSON(t, ts, http.MethodGet, "/.well-known/openid-credential-issuer", nil, ""),
		http.StatusOK, &meta)

	if meta.CredentialIssuer == "" {
		t.Error("missing credential_issuer in well-known metadata")
	}
	if _, ok := meta.CredentialConfigurationsSupported["DepositPledgeAttestation"]; !ok {
		t.Error("missing DepositPledgeAttestation in credential_configurations_supported")
	}
}

// TestOID4VCI_JWKS verifies that the JWKS endpoint returns a valid EC P-256 public key.
func TestOID4VCI_JWKS(t *testing.T) {
	ts, _ := newTestServer(t)
	var jwks struct {
		Keys []struct {
			KTY string `json:"kty"`
			CRV string `json:"crv"`
			Alg string `json:"alg"`
			X   string `json:"x"`
			Y   string `json:"y"`
		} `json:"keys"`
	}
	mustDoJSON(t,
		doJSON(t, ts, http.MethodGet, "/.well-known/jwks.json", nil, ""),
		http.StatusOK, &jwks)

	if len(jwks.Keys) == 0 {
		t.Fatal("JWKS contains no keys")
	}
	key := jwks.Keys[0]
	if key.KTY != "EC" {
		t.Errorf("key type: got %q, want %q", key.KTY, "EC")
	}
	if key.CRV != "P-256" {
		t.Errorf("key curve: got %q, want %q", key.CRV, "P-256")
	}
	if key.X == "" || key.Y == "" {
		t.Error("missing x or y coordinate in JWK")
	}
}

// TestOID4VCI_UnsupportedCredentialType verifies that requesting an unknown credential type is rejected.
func TestOID4VCI_UnsupportedCredentialType(t *testing.T) {
	ts, tok := newTestServer(t)
	id := createAndPledge(t, ts, tok)

	code := fetchPreAuthorizedCode(t, ts, issueCredential(t, ts, tok, id))
	accessToken := exchangeCodeForToken(t, ts, code)

	mustDo(t,
		doJSON(t, ts, http.MethodPost, "/v1/credential",
			map[string]any{"format": "vc+sd-jwt", "vct": "UnknownCredentialType"},
			accessToken),
		http.StatusBadRequest)
}
