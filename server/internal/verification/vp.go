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
package verification

import (
	"crypto/ecdsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// ── VP Session store ──────────────────────────────────────────────────────────

type VpSession struct {
	ID          string
	DepositID   string
	Nonce       string
	ResponseURI string
	CreatedAt   time.Time
	ExpiresAt   time.Time
}

type Store struct {
	mu   sync.RWMutex
	byID map[string]*VpSession
}

func NewStore() *Store {
	return &Store{byID: make(map[string]*VpSession)}
}

func (s *Store) Create(depositID, responseURI string) *VpSession {
	sess := &VpSession{
		ID:          uuid.NewString(),
		DepositID:   depositID,
		Nonce:       uuid.NewString(),
		ResponseURI: responseURI,
		CreatedAt:   time.Now().UTC(),
		ExpiresAt:   time.Now().UTC().Add(10 * time.Minute),
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.byID[sess.ID] = sess
	return sess
}

func (s *Store) GetByID(id string) (*VpSession, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	sess, ok := s.byID[id]
	return sess, ok
}

func (s *Store) Delete(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.byID, id)
}

// ── Presentation Exchange types ───────────────────────────────────────────────

type VpRequest struct {
	ClientID               string                 `json:"client_id"`
	ResponseType           string                 `json:"response_type"`
	ResponseMode           string                 `json:"response_mode"`
	ResponseURI            string                 `json:"response_uri"`
	Nonce                  string                 `json:"nonce"`
	State                  string                 `json:"state"`
	PresentationDefinition PresentationDefinition `json:"presentation_definition"`
}

type PresentationDefinition struct {
	ID               string            `json:"id"`
	InputDescriptors []InputDescriptor `json:"input_descriptors"`
}

type InputDescriptor struct {
	ID          string                `json:"id"`
	Format      map[string]FormatAlgs `json:"format"`
	Constraints Constraints           `json:"constraints"`
}

type FormatAlgs struct {
	Alg []string `json:"alg"`
}

type Constraints struct {
	Fields          []Field `json:"fields"`
	LimitDisclosure string  `json:"limit_disclosure"`
}

type Field struct {
	Path     []string     `json:"path"`
	Filter   *FieldFilter `json:"filter,omitempty"`
	Optional bool         `json:"optional,omitempty"`
}

type FieldFilter struct {
	Type  string `json:"type"`
	Const string `json:"const,omitempty"`
}

// BuildVpRequest returns the VpRequest for KautionsPfandNachweis.
// Required claims: vct, deposit_id, pledge_date, legal_reference, issuing_bank, deposit_amount.
// Optional claim: pledged_until.
func BuildVpRequest(clientID, responseURI, nonce, state string) VpRequest {
	return VpRequest{
		ClientID:     clientID,
		ResponseType: "vp_token",
		ResponseMode: "direct_post",
		ResponseURI:  responseURI,
		Nonce:        nonce,
		State:        state,
		PresentationDefinition: PresentationDefinition{
			ID: "kautionspfandnachweis-pd",
			InputDescriptors: []InputDescriptor{
				{
					ID: "kautionspfandnachweis",
					Format: map[string]FormatAlgs{
						"vc+sd-jwt": {Alg: []string{"ES256"}},
					},
					Constraints: Constraints{
						LimitDisclosure: "required",
						Fields: []Field{
							{
								Path:   []string{"$.vct"},
								Filter: &FieldFilter{Type: "string", Const: "KautionsPfandNachweis"},
							},
							{Path: []string{"$.deposit_id"}},
							{Path: []string{"$.pledge_date"}},
							{Path: []string{"$.legal_reference"}},
							{Path: []string{"$.issuing_bank"}},
							{Path: []string{"$.deposit_amount"}},
							{Path: []string{"$.pledged_until"}, Optional: true},
						},
					},
				},
			},
		},
	}
}

// ── JWT claim structs ─────────────────────────────────────────────────────────

type issuerJWTClaims struct {
	jwt.RegisteredClaims
	VCT            string   `json:"vct"`
	SDAlg          string   `json:"_sd_alg"`
	SD             []string `json:"_sd"`
	DepositID      string   `json:"deposit_id"`
	PledgeDate     string   `json:"pledge_date"`
	LegalReference string   `json:"legal_reference"`
	IssuingBank    string   `json:"issuing_bank"`
	IssuingBankID  string   `json:"issuing_bank_id,omitempty"`
	PropertyID     string   `json:"property_id,omitempty"`
}

type kbJWTClaims struct {
	jwt.RegisteredClaims
	Nonce  string `json:"nonce"`
	SDHash string `json:"sd_hash"`
}

// ── Verified claims ───────────────────────────────────────────────────────────

type VerifiedClaims struct {
	CredentialID    string    `json:"credential_id"`
	DepositID       string    `json:"deposit_id"`
	PledgeDate      string    `json:"pledge_date"`
	LegalReference  string    `json:"legal_reference"`
	IssuingBank     string    `json:"issuing_bank"`
	DepositAmount   float64   `json:"deposit_amount,omitempty"`
	Currency        string    `json:"currency,omitempty"`
	PropertyAddress string    `json:"property_address,omitempty"`
	TenantFirstName string    `json:"tenant_first_name,omitempty"`
	TenantLastName  string    `json:"tenant_last_name,omitempty"`
	PledgedUntil    string    `json:"pledged_until,omitempty"`
	VerifiedAt      time.Time `json:"verified_at"`
}

// ── VP Token verification ─────────────────────────────────────────────────────

// VerifyVpToken validates an SD-JWT VP token.
// issuerPublicKey is the ECDSA P-256 public key of the credential issuer.
//
// Note on KB-JWT signature: NOT verified (no cnf.jwk stored during issuance).
// TODO: store holder public key from proof during /v1/credential, use it here.
func VerifyVpToken(vpToken string, issuerPublicKey *ecdsa.PublicKey, expectedNonce, responseURI string) (*VerifiedClaims, error) {
	// Step 1: split on "~"
	parts := strings.Split(vpToken, "~")
	if len(parts) < 1 || parts[0] == "" {
		return nil, errors.New("verification: malformed vp_token: missing issuer JWT")
	}

	issuerJWT := parts[0]

	// Identify KB-JWT: last non-empty part after the issuer JWT
	var kbJWTRaw string
	var disclosures []string

	for _, p := range parts[1:] {
		if p == "" {
			continue
		}
		// A KB-JWT header contains "kb+jwt" — peek at the first segment to detect it.
		// Disclosures are base64url-encoded JSON arrays; KB-JWTs are dotted JWT strings.
		if strings.Contains(p, ".") {
			// Could be a KB-JWT; disclosures never contain dots (they're base64url of JSON).
			kbJWTRaw = p
		} else {
			disclosures = append(disclosures, p)
		}
	}

	// Step 2: parse and verify issuer JWT signature
	var claims issuerJWTClaims
	_, err := jwt.ParseWithClaims(issuerJWT, &claims, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodECDSA); !ok {
			return nil, fmt.Errorf("verification: unexpected signing method: %v", t.Header["alg"])
		}
		return issuerPublicKey, nil
	})
	if err != nil {
		return nil, fmt.Errorf("verification: issuer JWT invalid: %w", err)
	}

	// Step 3: check vct and expiry
	if claims.VCT != "KautionsPfandNachweis" {
		return nil, fmt.Errorf("verification: unexpected vct %q", claims.VCT)
	}
	if claims.ExpiresAt != nil && time.Now().After(claims.ExpiresAt.Time) {
		return nil, errors.New("verification: credential has expired")
	}

	// Build the issuer SD hash set for disclosure verification
	sdSet := make(map[string]struct{}, len(claims.SD))
	for _, h := range claims.SD {
		sdSet[h] = struct{}{}
	}

	// Step 4: verify each disclosure
	disclosed := make(map[string]any)
	for _, enc := range disclosures {
		h := sha256.Sum256([]byte(enc))
		hash := base64.RawURLEncoding.EncodeToString(h[:])
		if _, ok := sdSet[hash]; !ok {
			return nil, fmt.Errorf("verification: disclosure hash %q not found in _sd", hash)
		}

		raw, err := base64.RawURLEncoding.DecodeString(enc)
		if err != nil {
			return nil, fmt.Errorf("verification: disclosure base64 decode: %w", err)
		}
		var arr []any
		if err := json.Unmarshal(raw, &arr); err != nil || len(arr) != 3 {
			return nil, errors.New("verification: disclosure must be [salt, name, value]")
		}
		name, ok := arr[1].(string)
		if !ok {
			return nil, errors.New("verification: disclosure name is not a string")
		}
		disclosed[name] = arr[2]
	}

	// Step 5: verify KB-JWT if present
	if kbJWTRaw != "" {
		// 5a: parse without signature verification
		parser := jwt.NewParser()
		var kbClaims kbJWTClaims
		kbToken, _, err := parser.ParseUnverified(kbJWTRaw, &kbClaims)
		if err != nil {
			return nil, fmt.Errorf("verification: KB-JWT parse: %w", err)
		}

		// 5b: check header typ
		if typ, _ := kbToken.Header["typ"].(string); typ != "kb+jwt" {
			return nil, fmt.Errorf("verification: KB-JWT typ must be kb+jwt, got %q", typ)
		}

		// 5c: check nonce and aud
		if kbClaims.Nonce != expectedNonce {
			return nil, errors.New("verification: KB-JWT nonce mismatch")
		}
		auds, err := kbClaims.GetAudience()
		if err != nil || len(auds) == 0 || auds[0] != responseURI {
			return nil, errors.New("verification: KB-JWT aud does not match response_uri")
		}

		// 5d: verify sd_hash
		expectedSDHash := computeSDHash(issuerJWT, disclosures)
		if kbClaims.SDHash != expectedSDHash {
			return nil, errors.New("verification: KB-JWT sd_hash mismatch")
		}

		// 5e: check iat within last 5 minutes
		if kbClaims.IssuedAt == nil || time.Since(kbClaims.IssuedAt.Time) > 5*time.Minute {
			return nil, errors.New("verification: KB-JWT iat too old or missing")
		}
	}

	// Step 6: assemble VerifiedClaims from always-revealed + disclosed map
	vc := &VerifiedClaims{
		CredentialID:   claims.ID,
		DepositID:      claims.DepositID,
		PledgeDate:     claims.PledgeDate,
		LegalReference: claims.LegalReference,
		IssuingBank:    claims.IssuingBank,
		VerifiedAt:     time.Now().UTC(),
	}

	if v, ok := disclosed["deposit_amount"]; ok {
		if f, ok := v.(float64); ok {
			vc.DepositAmount = f
		}
	}
	if v, ok := disclosed["currency"]; ok {
		if s, ok := v.(string); ok {
			vc.Currency = s
		}
	}
	if v, ok := disclosed["property_address"]; ok {
		if s, ok := v.(string); ok {
			vc.PropertyAddress = s
		}
	}
	if v, ok := disclosed["tenant_first_name"]; ok {
		if s, ok := v.(string); ok {
			vc.TenantFirstName = s
		}
	}
	if v, ok := disclosed["tenant_last_name"]; ok {
		if s, ok := v.(string); ok {
			vc.TenantLastName = s
		}
	}
	if v, ok := disclosed["pledged_until"]; ok {
		if s, ok := v.(string); ok {
			vc.PledgedUntil = s
		}
	}

	return vc, nil
}

// computeSDHash computes base64url(sha256(issuerJWT~disc1~...~discN~)) as required by SD-JWT KB.
func computeSDHash(issuerJWT string, disclosures []string) string {
	var sb strings.Builder
	sb.WriteString(issuerJWT)
	sb.WriteByte('~')
	for _, d := range disclosures {
		sb.WriteString(d)
		sb.WriteByte('~')
	}
	h := sha256.Sum256([]byte(sb.String()))
	return base64.RawURLEncoding.EncodeToString(h[:])
}
