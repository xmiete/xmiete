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
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// fetchStatusList calls GET /v1/status-list/revocation and returns the parsed response.
func fetchStatusList(t *testing.T, ts *httptest.Server) (id, typ, credential string) {
	t.Helper()
	resp, err := http.Get(ts.URL + "/v1/status-list/revocation")
	if err != nil {
		t.Fatalf("GET status-list: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET status-list: HTTP %d", resp.StatusCode)
	}
	var body struct {
		ID         string `json:"id"`
		Type       string `json:"type"`
		Credential string `json:"credential"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode status-list response: %v", err)
	}
	return body.ID, body.Type, body.Credential
}

// decodeEncodedList decompresses a W3C base64url(gzip(bitstring)) and returns the raw bitstring.
func decodeEncodedList(t *testing.T, encodedList string) []byte {
	t.Helper()
	gz, err := base64.RawURLEncoding.DecodeString(encodedList)
	if err != nil {
		t.Fatalf("base64url decode encodedList: %v", err)
	}
	r, err := gzip.NewReader(bytes.NewReader(gz))
	if err != nil {
		t.Fatalf("gzip.NewReader: %v", err)
	}
	defer r.Close()
	bs, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("gzip read: %v", err)
	}
	return bs
}

// bitAt returns the bit value at position index in the MSB-first bitstring.
func bitAt(bs []byte, index int) int {
	if index < 0 || index/8 >= len(bs) {
		return 0
	}
	return int((bs[index/8] >> uint(7-index%8)) & 1)
}

// extractStatusListIndex decodes the SD-JWT and returns the credentialStatus.statusListIndex.
func extractStatusListIndex(t *testing.T, sdJWT string) int {
	t.Helper()
	jwtPart := strings.SplitN(sdJWT, "~", 2)[0]
	parts := strings.Split(jwtPart, ".")
	if len(parts) != 3 {
		t.Fatalf("invalid SD-JWT structure")
	}
	payloadJSON, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		t.Fatalf("decode SD-JWT payload: %v", err)
	}
	var claims struct {
		CredentialStatus struct {
			StatusListIndex      string `json:"statusListIndex"`
			StatusListCredential string `json:"statusListCredential"`
		} `json:"credentialStatus"`
	}
	if err := json.Unmarshal(payloadJSON, &claims); err != nil {
		t.Fatalf("unmarshal SD-JWT claims: %v", err)
	}
	if claims.CredentialStatus.StatusListIndex == "" {
		t.Fatal("missing credentialStatus.statusListIndex in SD-JWT")
	}
	var idx int
	for _, c := range claims.CredentialStatus.StatusListIndex {
		if c < '0' || c > '9' {
			t.Fatalf("non-digit in statusListIndex: %q", claims.CredentialStatus.StatusListIndex)
		}
		idx = idx*10 + int(c-'0')
	}
	return idx
}

// ── Tests ──────────────────────────────────────────────────────────────────────

// TestCSL_EmptyList verifies that the status list is accessible before any credentials are revoked.
func TestCSL_EmptyList(t *testing.T) {
	ts, _ := newTestServer(t)
	id, typ, credential := fetchStatusList(t, ts)

	if id == "" {
		t.Error("missing id in status list response")
	}
	if typ != "BitstringStatusListCredential" {
		t.Errorf("type: got %q, want BitstringStatusListCredential", typ)
	}
	if credential == "" {
		t.Error("missing credential JWT in status list response")
	}
}

// TestCSL_JWTStructure verifies that the status list JWT has the correct header and payload structure.
func TestCSL_JWTStructure(t *testing.T) {
	ts, _ := newTestServer(t)
	_, _, credential := fetchStatusList(t, ts)

	parts := strings.Split(credential, ".")
	if len(parts) != 3 {
		t.Fatalf("expected 3-part JWT, got %d parts", len(parts))
	}

	// Check header
	headerJSON, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		t.Fatalf("decode JWT header: %v", err)
	}
	var header map[string]any
	if err := json.Unmarshal(headerJSON, &header); err != nil {
		t.Fatalf("unmarshal JWT header: %v", err)
	}
	if header["typ"] != "vc+ld+jwt" {
		t.Errorf("header typ: got %v, want vc+ld+jwt", header["typ"])
	}
	if header["alg"] != "ES256" {
		t.Errorf("header alg: got %v, want ES256", header["alg"])
	}

	// Check payload
	payloadJSON, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		t.Fatalf("decode JWT payload: %v", err)
	}
	var payload struct {
		VC struct {
			Type              []string `json:"type"`
			CredentialSubject struct {
				Type          string `json:"type"`
				StatusPurpose string `json:"statusPurpose"`
				EncodedList   string `json:"encodedList"`
			} `json:"credentialSubject"`
		} `json:"vc"`
	}
	if err := json.Unmarshal(payloadJSON, &payload); err != nil {
		t.Fatalf("unmarshal JWT payload: %v", err)
	}

	hasCSLType := false
	for _, typ := range payload.VC.Type {
		if typ == "BitstringStatusListCredential" {
			hasCSLType = true
		}
	}
	if !hasCSLType {
		t.Error("status list VC missing BitstringStatusListCredential type")
	}
	if payload.VC.CredentialSubject.StatusPurpose != "revocation" {
		t.Errorf("statusPurpose: got %q, want revocation", payload.VC.CredentialSubject.StatusPurpose)
	}
	if payload.VC.CredentialSubject.EncodedList == "" {
		t.Error("missing encodedList in credentialSubject")
	}

	// Verify the encoded list decodes to at least 16 KB
	bs := decodeEncodedList(t, payload.VC.CredentialSubject.EncodedList)
	if len(bs) < 131_072/8 {
		t.Errorf("bitstring length: got %d bytes, want at least %d", len(bs), 131_072/8)
	}
}

// TestCSL_CredentialContainsStatusEntry verifies that issued credentials include a credentialStatus claim.
func TestCSL_CredentialContainsStatusEntry(t *testing.T) {
	ts, tok := newTestServer(t)
	id := createAndPledge(t, ts, tok)

	code := fetchPreAuthorizedCode(t, ts, issueCredential(t, ts, tok, id))
	sdJWT := fetchCredential(t, ts, exchangeCodeForToken(t, ts, code))

	// Extract credentialStatus from the SD-JWT payload
	jwtPart := strings.SplitN(sdJWT, "~", 2)[0]
	parts := strings.Split(jwtPart, ".")
	payloadJSON, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		t.Fatalf("decode SD-JWT payload: %v", err)
	}
	var claims struct {
		CredentialStatus struct {
			ID                   string `json:"id"`
			Type                 string `json:"type"`
			StatusPurpose        string `json:"statusPurpose"`
			StatusListIndex      string `json:"statusListIndex"`
			StatusListCredential string `json:"statusListCredential"`
		} `json:"credentialStatus"`
	}
	if err := json.Unmarshal(payloadJSON, &claims); err != nil {
		t.Fatalf("unmarshal SD-JWT claims: %v", err)
	}
	cs := claims.CredentialStatus
	if cs.Type != "BitstringStatusListEntry" {
		t.Errorf("credentialStatus.type: got %q, want BitstringStatusListEntry", cs.Type)
	}
	if cs.StatusPurpose != "revocation" {
		t.Errorf("credentialStatus.statusPurpose: got %q, want revocation", cs.StatusPurpose)
	}
	if cs.StatusListIndex == "" {
		t.Error("missing credentialStatus.statusListIndex")
	}
	if !strings.Contains(cs.StatusListCredential, "/v1/status-list/revocation") {
		t.Errorf("credentialStatus.statusListCredential: got %q, want URL containing /v1/status-list/revocation", cs.StatusListCredential)
	}
	if !strings.HasSuffix(cs.ID, "#"+cs.StatusListIndex) {
		t.Errorf("credentialStatus.id %q should end with #%s", cs.ID, cs.StatusListIndex)
	}
}

// TestCSL_RevocationSetsBit verifies that releasing a deposit sets the credential's bit in the status list.
func TestCSL_RevocationSetsBit(t *testing.T) {
	ts, tok := newTestServer(t)
	id := createAndPledge(t, ts, tok)

	code := fetchPreAuthorizedCode(t, ts, issueCredential(t, ts, tok, id))
	sdJWT := fetchCredential(t, ts, exchangeCodeForToken(t, ts, code))

	// Get the assigned status list index from the credential
	statusListIndex := extractStatusListIndex(t, sdJWT)

	// Before release: bit must be 0 (not revoked)
	_, _, credential := fetchStatusList(t, ts)
	payloadJSON, _ := base64.RawURLEncoding.DecodeString(strings.Split(credential, ".")[1])
	var pre struct {
		VC struct{ CredentialSubject struct{ EncodedList string } } `json:"vc"`
	}
	json.Unmarshal(payloadJSON, &pre) //nolint:errcheck
	bs := decodeEncodedList(t, pre.VC.CredentialSubject.EncodedList)
	if bitAt(bs, statusListIndex) != 0 {
		t.Errorf("bit %d should be 0 before release", statusListIndex)
	}

	// Release deposit — triggers revocation
	mustDo(t, doJSON(t, ts, http.MethodPost, "/v1/deposits/"+id+"/release", nil, tok), http.StatusOK)

	// After release: bit must be 1 (revoked)
	_, _, credential = fetchStatusList(t, ts)
	payloadJSON, _ = base64.RawURLEncoding.DecodeString(strings.Split(credential, ".")[1])
	var post struct {
		VC struct{ CredentialSubject struct{ EncodedList string } } `json:"vc"`
	}
	json.Unmarshal(payloadJSON, &post) //nolint:errcheck
	bs = decodeEncodedList(t, post.VC.CredentialSubject.EncodedList)
	if bitAt(bs, statusListIndex) != 1 {
		t.Errorf("bit %d should be 1 after release, got 0", statusListIndex)
	}
}

// TestCSL_MultipleCredentials verifies that different credentials get different indices
// and that only the revoked one has its bit set.
func TestCSL_MultipleCredentials(t *testing.T) {
	ts, tok := newTestServer(t)

	id1 := createAndPledge(t, ts, tok)
	id2 := createAndPledge(t, ts, tok)

	sdJWT1 := fetchCredential(t, ts, exchangeCodeForToken(t, ts,
		fetchPreAuthorizedCode(t, ts, issueCredential(t, ts, tok, id1))))
	sdJWT2 := fetchCredential(t, ts, exchangeCodeForToken(t, ts,
		fetchPreAuthorizedCode(t, ts, issueCredential(t, ts, tok, id2))))

	idx1 := extractStatusListIndex(t, sdJWT1)
	idx2 := extractStatusListIndex(t, sdJWT2)

	if idx1 == idx2 {
		t.Errorf("two credentials got the same status list index %d", idx1)
	}

	// Release only deposit 1
	mustDo(t, doJSON(t, ts, http.MethodPost, "/v1/deposits/"+id1+"/release", nil, tok), http.StatusOK)

	_, _, credential := fetchStatusList(t, ts)
	payloadJSON, _ := base64.RawURLEncoding.DecodeString(strings.Split(credential, ".")[1])
	var payload struct {
		VC struct{ CredentialSubject struct{ EncodedList string } } `json:"vc"`
	}
	json.Unmarshal(payloadJSON, &payload) //nolint:errcheck
	bs := decodeEncodedList(t, payload.VC.CredentialSubject.EncodedList)

	if bitAt(bs, idx1) != 1 {
		t.Errorf("released credential bit %d should be 1", idx1)
	}
	if bitAt(bs, idx2) != 0 {
		t.Errorf("active credential bit %d should be 0", idx2)
	}
}
