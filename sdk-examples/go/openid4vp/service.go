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

// Package openid4vp implements OpenID4VP presentation request building and
// SD-JWT VP verification for DepositPledgeAttestation credentials.
//
// Flow:
//  1. Call VpVerifier.BuildVpRequest → store the returned nonce and send VpRequest to wallet
//  2. Wallet POSTs a vp_token to your response_uri
//  3. Call VpVerifier.VerifyVpToken with the stored nonce → receive VerifiedClaims
package openid4vp

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// VpVerifier is the interface a verifier (landlord app, property management system)
// must satisfy to integrate OpenID4VP wallet-based credential presentation.
// The included VpVerifierService provides a complete implementation.
type VpVerifier interface {
	// BuildVpRequest creates a VP request for a DepositPledgeAttestation presentation.
	// The caller must persist the returned VpRequestResult.Nonce and pass it to VerifyVpToken.
	BuildVpRequest(ctx context.Context, depositID, responseURI string) (*VpRequestResult, error)

	// VerifyVpToken verifies a vp_token received from the wallet.
	// vpToken is the SD-JWT VP wire format: issuerJWT~disc1~...~discN~kbJWT
	// expectedNonce must match the nonce from the corresponding BuildVpRequest call.
	VerifyVpToken(ctx context.Context, vpToken, expectedNonce, responseURI string) (*VerifiedClaims, error)
}

// VpVerifierService implements VpVerifier.
// Set JWKSUri to enable issuer signature verification (currently stubbed — see verifyIssuerJWT).
type VpVerifierService struct {
	ClientID   string
	JWKSUri    string
	httpClient *http.Client
}

// NewVpVerifierService creates a VpVerifierService.
func NewVpVerifierService(clientID, jwksURI string) *VpVerifierService {
	return &VpVerifierService{
		ClientID:   clientID,
		JWKSUri:    jwksURI,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

var _ VpVerifier = (*VpVerifierService)(nil)

func (s *VpVerifierService) BuildVpRequest(_ context.Context, depositID, responseURI string) (*VpRequestResult, error) {
	nonce, err := newUUID()
	if err != nil {
		return nil, fmt.Errorf("openid4vp: generate nonce: %w", err)
	}
	state := depositID

	constVct := "DepositPledgeAttestation"
	req := VpRequest{
		ClientID:     s.ClientID,
		ResponseType: "vp_token",
		ResponseMode: "direct_post",
		ResponseURI:  responseURI,
		Nonce:        nonce,
		State:        state,
		PresentationDefinition: PresentationDefinition{
			ID: "deposit-pledge-attestation-pd",
			InputDescriptors: []InputDescriptor{
				{
					ID:     "deposit-pledge-attestation",
					Format: map[string]FormatAlgs{"vc+sd-jwt": {Alg: []string{"ES256"}}},
					Constraints: Constraints{
						LimitDisclosure: "required",
						Fields: []Field{
							{Path: []string{"$.vct"}, Filter: &FieldFilter{Type: "string", Const: &constVct}},
							{Path: []string{"$.deposit_id"}},
							{Path: []string{"$.pledge_date"}},
							{Path: []string{"$.statutory_basis"}},
							{Path: []string{"$.issuing_bank"}},
							{Path: []string{"$.deposit_amount"}, Optional: true},
							{Path: []string{"$.currency"}, Optional: true},
							{Path: []string{"$.property_address"}, Optional: true},
							{Path: []string{"$.tenant_first_name"}, Optional: true},
							{Path: []string{"$.tenant_last_name"}, Optional: true},
							{Path: []string{"$.pledged_until"}, Optional: true},
						},
					},
				},
			},
		},
	}

	return &VpRequestResult{Nonce: nonce, VpRequest: req}, nil
}

func (s *VpVerifierService) VerifyVpToken(_ context.Context, vpToken, expectedNonce, responseURI string) (*VerifiedClaims, error) {
	parts := strings.Split(vpToken, "~")
	if len(parts) == 0 || parts[0] == "" {
		return nil, errors.New("openid4vp: missing issuer JWT in vp_token")
	}

	issuerJWT := parts[0]

	// Last non-empty element after the issuer JWT is the KB-JWT; middle elements are disclosures.
	var kbJWT string
	var disclosureEncodings []string
	if len(parts) > 1 {
		last := parts[len(parts)-1]
		if last != "" {
			kbJWT = last
		}
		discEnd := len(parts) - 1
		if kbJWT == "" {
			discEnd = len(parts)
		}
		for _, d := range parts[1:discEnd] {
			if d != "" {
				disclosureEncodings = append(disclosureEncodings, d)
			}
		}
	}

	issuerClaims, err := s.verifyIssuerJWT(issuerJWT)
	if err != nil {
		return nil, fmt.Errorf("openid4vp: issuer JWT: %w", err)
	}

	if time.Now().Unix() >= issuerClaims.Exp {
		return nil, errors.New("openid4vp: issuer JWT has expired")
	}

	sdHashSet := make(map[string]struct{}, len(issuerClaims.SDHashes))
	for _, h := range issuerClaims.SDHashes {
		sdHashSet[h] = struct{}{}
	}

	disclosed := make(map[string]json.RawMessage)
	for _, enc := range disclosureEncodings {
		digest := sdJWTHash(enc)
		if _, ok := sdHashSet[digest]; !ok {
			return nil, fmt.Errorf("openid4vp: disclosure digest %s not in _sd array", digest)
		}
		raw, err := base64.RawURLEncoding.DecodeString(enc)
		if err != nil {
			return nil, fmt.Errorf("openid4vp: decode disclosure: %w", err)
		}
		var arr []json.RawMessage
		if err := json.Unmarshal(raw, &arr); err != nil || len(arr) < 3 {
			return nil, errors.New("openid4vp: disclosure must be a JSON array with at least 3 elements")
		}
		var name string
		if err := json.Unmarshal(arr[1], &name); err != nil {
			return nil, fmt.Errorf("openid4vp: disclosure name is not a string: %w", err)
		}
		disclosed[name] = arr[2]
	}

	if kbJWT != "" {
		if err := verifyKbJWT(kbJWT, issuerJWT, disclosureEncodings, expectedNonce, responseURI); err != nil {
			return nil, err
		}
	}

	claims := &VerifiedClaims{
		CredentialID:   issuerClaims.CredentialID,
		DepositID:      issuerClaims.DepositID,
		PledgeDate:     issuerClaims.PledgeDate,
		StatutoryBasis: issuerClaims.StatutoryBasis,
		IssuingBank:    issuerClaims.IssuingBank,
		VerifiedAt:     time.Now().UTC(),
	}

	if v, ok := disclosed["deposit_amount"]; ok {
		var f float64
		if json.Unmarshal(v, &f) == nil {
			claims.DepositAmount = &f
		}
	}
	claims.Currency = optString(disclosed, "currency")
	claims.PropertyAddress = optString(disclosed, "property_address")
	claims.TenantFirstName = optString(disclosed, "tenant_first_name")
	claims.TenantLastName = optString(disclosed, "tenant_last_name")
	claims.PledgedUntil = optString(disclosed, "pledged_until")

	return claims, nil
}

// verifyIssuerJWT parses the issuer JWT payload.
//
// Production note: verify the ES256 signature against self.JWKSUri before trusting any claims.
// Fetch the JWKS, match the key by `kid` from the JWT header, then verify with crypto/ecdsa.
func (s *VpVerifierService) verifyIssuerJWT(jwt string) (*IssuerClaims, error) {
	parts := strings.SplitN(jwt, ".", 3)
	if len(parts) < 3 {
		return nil, errors.New("malformed JWT: expected 3 dot-separated parts")
	}

	payloadRaw, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, fmt.Errorf("base64 decode payload: %w", err)
	}

	var p struct {
		Jti            string   `json:"jti"`
		DepositID      string   `json:"deposit_id"`
		PledgeDate     string   `json:"pledge_date"`
		StatutoryBasis string   `json:"statutory_basis"`
		IssuingBank    string   `json:"issuing_bank"`
		SD             []string `json:"_sd"`
		Exp            int64    `json:"exp"`
	}
	if err := json.Unmarshal(payloadRaw, &p); err != nil {
		return nil, fmt.Errorf("parse payload: %w", err)
	}

	return &IssuerClaims{
		CredentialID:   p.Jti,
		DepositID:      p.DepositID,
		PledgeDate:     p.PledgeDate,
		StatutoryBasis: p.StatutoryBasis,
		IssuingBank:    p.IssuingBank,
		SDHashes:       p.SD,
		Exp:            p.Exp,
	}, nil
}

func verifyKbJWT(kbJWT, issuerJWT string, disclosures []string, expectedNonce, responseURI string) error {
	parts := strings.SplitN(kbJWT, ".", 3)
	if len(parts) < 2 {
		return errors.New("openid4vp: malformed KB-JWT")
	}

	headerRaw, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return fmt.Errorf("openid4vp: decode KB-JWT header: %w", err)
	}
	var header struct {
		Typ string `json:"typ"`
	}
	if err := json.Unmarshal(headerRaw, &header); err != nil || header.Typ != "kb+jwt" {
		return fmt.Errorf("openid4vp: KB-JWT typ must be kb+jwt, got %q", header.Typ)
	}

	payloadRaw, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return fmt.Errorf("openid4vp: decode KB-JWT payload: %w", err)
	}
	var payload struct {
		Nonce  string `json:"nonce"`
		Aud    string `json:"aud"`
		SdHash string `json:"sd_hash"`
		Iat    int64  `json:"iat"`
	}
	if err := json.Unmarshal(payloadRaw, &payload); err != nil {
		return fmt.Errorf("openid4vp: parse KB-JWT payload: %w", err)
	}

	if payload.Nonce != expectedNonce {
		return fmt.Errorf("openid4vp: KB-JWT nonce mismatch: expected %q, got %q", expectedNonce, payload.Nonce)
	}
	if payload.Aud != responseURI {
		return fmt.Errorf("openid4vp: KB-JWT aud mismatch: expected %q, got %q", responseURI, payload.Aud)
	}

	// Recompute sd_hash over: issuerJWT~disc1~...~discN~
	var sdInput strings.Builder
	sdInput.WriteString(issuerJWT)
	sdInput.WriteByte('~')
	for _, d := range disclosures {
		sdInput.WriteString(d)
		sdInput.WriteByte('~')
	}
	expectedSdHash := sdJWTHash(sdInput.String())
	if payload.SdHash != expectedSdHash {
		return errors.New("openid4vp: KB-JWT sd_hash mismatch")
	}

	now := time.Now().Unix()
	if payload.Iat > now+30 || payload.Iat < now-300 {
		return errors.New("openid4vp: KB-JWT iat is stale or from the future")
	}

	// TODO: verify KB-JWT ES256 signature using the holder's public key (cnf.jwk from issuer claims).
	return nil
}

// sdJWTHash computes the base64url-unpadded SHA-256 digest of s, as required by the SD-JWT spec.
func sdJWTHash(s string) string {
	sum := sha256.Sum256([]byte(s))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

func optString(m map[string]json.RawMessage, key string) *string {
	v, ok := m[key]
	if !ok {
		return nil
	}
	var s string
	if err := json.Unmarshal(v, &s); err != nil {
		return nil
	}
	return &s
}

// newUUID generates a random UUID v4 using crypto/rand.
func newUUID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	b[6] = (b[6] & 0x0f) | 0x40 // version 4
	b[8] = (b[8] & 0x3f) | 0x80 // variant bits
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:16]), nil
}
