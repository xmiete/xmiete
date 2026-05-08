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
package issuance

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/xmiete/server/internal/models"
)

// Signer holds the ECDSA P-256 key used to sign DepositPledgeAttestation credentials.
// In production, replace key generation with an HSM or KMS-backed key loader.
type Signer struct {
	once  sync.Once
	key   *ecdsa.PrivateKey
	KeyID string
}

var DefaultSigner = &Signer{}

func (s *Signer) init() {
	s.once.Do(func() {
		key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		if err != nil {
			panic("issuance: failed to generate signing key: " + err.Error())
		}
		s.key = key
		s.KeyID = uuid.NewString()
	})
}

// PublicKey returns the public key for JWKS publication.
func (s *Signer) PublicKey() *ecdsa.PublicKey {
	s.init()
	return &s.key.PublicKey
}

// sdDisclosure encodes a single SD-JWT disclosure: base64url(["salt","name",value]).
type sdDisclosure struct {
	salt  string
	name  string
	value any
}

func newSDDisclosure(name string, value any) (enc string, hash string, err error) {
	saltBytes := make([]byte, 16)
	if _, err = rand.Read(saltBytes); err != nil {
		return
	}
	d := sdDisclosure{
		salt:  base64.RawURLEncoding.EncodeToString(saltBytes),
		name:  name,
		value: value,
	}
	arr := []any{d.salt, d.name, d.value}
	b, err := json.Marshal(arr)
	if err != nil {
		return
	}
	enc = base64.RawURLEncoding.EncodeToString(b)
	h := sha256.Sum256([]byte(enc))
	hash = base64.RawURLEncoding.EncodeToString(h[:])
	return
}

// sdJWTClaims is the JWT payload for a DepositPledgeAttestation SD-JWT.
// Fields under _sd are selectively disclosed; all other fields are always revealed.
type sdJWTClaims struct {
	jwt.RegisteredClaims

	// Credential type per OpenID4VCI / EUDI ARF vct claim
	VCT string `json:"vct"`

	// SD-JWT algorithm indicator
	SDAlg string `json:"_sd_alg"`

	// Hashes of selectively-disclosable claims
	SD []string `json:"_sd"`

	// Non-selectively-disclosed claims (always revealed in every presentation)
	DepositID      string `json:"deposit_id"`
	PledgeDate     string `json:"pledge_date"`
	StatutoryBasis string `json:"statutory_basis"`
	IssuingBank    string `json:"issuing_bank"`
	IssuingBankID  string `json:"issuing_bank_id,omitempty"`
	PropertyID     string `json:"property_id,omitempty"`
}

// BuildSDJWT constructs and signs a DepositPledgeAttestation SD-JWT credential.
//
// Selectively disclosed claims (tenant can choose which to present):
//   - deposit_amount, currency
//   - property_address
//   - tenant_first_name, tenant_last_name
//   - pledged_until
//
// Always-revealed claims: deposit_id, pledge_date, statutory_basis, issuing_bank.
//
// Returns (sdJWTToken, credentialID, error).
// The token format is: header.payload.signature~disclosure_1~...~disclosure_n~
func BuildSDJWT(issuerURL string, deposit *models.Deposit, validUntil string) (string, string, error) {
	DefaultSigner.init()

	credentialID := "urn:xmiete:credential:" + uuid.NewString()
	now := time.Now().UTC()

	expiry := now.AddDate(1, 0, 0)
	if validUntil != "" {
		if t, err := time.Parse("2006-01-02", validUntil); err == nil {
			expiry = t.Add(30 * 24 * time.Hour) // 30-day grace after pledge end
		}
	}

	pledgedUntil := validUntil
	if pledgedUntil == "" {
		pledgedUntil = now.AddDate(1, 0, 0).Format("2006-01-02")
	}

	propertyAddr := ""
	if p := deposit.Property.Address; p.Street != "" {
		propertyAddr = fmt.Sprintf("%s, %s %s, %s", p.Street, p.ZIP, p.City, p.Country)
	}

	pledgeDate := now.Format("2006-01-02")
	if deposit.Pledge != nil && deposit.Pledge.PledgeDate != "" {
		pledgeDate = deposit.Pledge.PledgeDate
	}

	bankName := ""
	bankID := ""
	if deposit.Provider != nil {
		bankName = deposit.Provider.ExecutingEntity
		if bankName != "" {
			bankID = "https://" + strings.ToLower(strings.ReplaceAll(bankName, " ", "."))
		}
	}

	// Build selectively-disclosable claims
	sdSpecs := []struct {
		name  string
		value any
	}{
		{"deposit_amount", deposit.Deposit.Amount},
		{"currency", deposit.Deposit.Currency},
		{"property_address", propertyAddr},
		{"tenant_first_name", deposit.Tenant.FirstName},
		{"tenant_last_name", deposit.Tenant.LastName},
		{"pledged_until", pledgedUntil},
	}

	var disclosureEncodings []string
	var sdHashes []string
	for _, spec := range sdSpecs {
		enc, hash, err := newSDDisclosure(spec.name, spec.value)
		if err != nil {
			return "", "", fmt.Errorf("issuance: disclosure for %q: %w", spec.name, err)
		}
		disclosureEncodings = append(disclosureEncodings, enc)
		sdHashes = append(sdHashes, hash)
	}

	claims := sdJWTClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    issuerURL,
			Subject:   deposit.Tenant.Email,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(expiry),
			ID:        credentialID,
		},
		VCT:            "DepositPledgeAttestation",
		SDAlg:          "sha-256",
		SD:             sdHashes,
		DepositID:      deposit.ID,
		PledgeDate:     pledgeDate,
		StatutoryBasis: "BGB § 551",
		IssuingBank:    bankName,
		IssuingBankID:  bankID,
		PropertyID:     deposit.Property.UnitID,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodES256, claims)
	token.Header["typ"] = "vc+sd-jwt"
	token.Header["kid"] = DefaultSigner.KeyID

	signed, err := token.SignedString(DefaultSigner.key)
	if err != nil {
		return "", "", fmt.Errorf("issuance: signing credential: %w", err)
	}

	// SD-JWT wire format: <signed-jwt>~<disclosure_1>~...~<disclosure_n>~
	sdJWT := signed + "~" + strings.Join(disclosureEncodings, "~") + "~"
	return sdJWT, credentialID, nil
}
