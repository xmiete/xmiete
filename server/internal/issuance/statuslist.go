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
	"bytes"
	"compress/gzip"
	"context"
	"encoding/base64"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// StatusListSize is the W3C minimum: 16 KB = 131 072 entries.
// Each bit corresponds to one credential slot; 0 = valid, 1 = revoked.
const StatusListSize = 131_072

// IndexAllocator assigns unique sequential indices for the W3C Bitstring Status List.
type IndexAllocator interface {
	AllocateIndex(ctx context.Context) (int, error)
}

// MemIndexAllocator is a thread-safe, in-memory allocator (dev/tests).
type MemIndexAllocator struct {
	counter atomic.Int64
}

func (a *MemIndexAllocator) AllocateIndex(_ context.Context) (int, error) {
	return int(a.counter.Add(1) - 1), nil
}

// BuildEncodedList compresses a set of revoked indices into the W3C base64url(gzip(bitstring)) format.
func BuildEncodedList(revokedIndices []int) (string, error) {
	bitstring := make([]byte, StatusListSize/8) // 16 384 bytes
	for _, idx := range revokedIndices {
		if idx < 0 || idx >= StatusListSize {
			continue
		}
		// MSB-first: bit 0 is the most significant bit of byte 0.
		bitstring[idx/8] |= 1 << uint(7-idx%8)
	}

	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	if _, err := gz.Write(bitstring); err != nil {
		return "", err
	}
	if err := gz.Close(); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf.Bytes()), nil
}

// statusListVCClaims is the JWT payload for a BitstringStatusListCredential.
type statusListVCClaims struct {
	jwt.RegisteredClaims
	VC statusListVC `json:"vc"`
}

type statusListVC struct {
	Context           []string          `json:"@context"`
	ID                string            `json:"id"`
	Type              []string          `json:"type"`
	Issuer            string            `json:"issuer"`
	ValidFrom         string            `json:"validFrom"`
	CredentialSubject statusListSubject `json:"credentialSubject"`
}

type statusListSubject struct {
	ID            string `json:"id"`
	Type          string `json:"type"`
	StatusPurpose string `json:"statusPurpose"`
	EncodedList   string `json:"encodedList"`
}

// BuildStatusListJWT builds and signs a W3C BitstringStatusListCredential as a JWT.
// The returned JWT is signed with the same ECDSA P-256 key as SD-JWT credentials.
func BuildStatusListJWT(issuerURL string, revokedIndices []int) (string, error) {
	DefaultSigner.init()

	encodedList, err := BuildEncodedList(revokedIndices)
	if err != nil {
		return "", fmt.Errorf("status list: encode: %w", err)
	}

	statusListURL := issuerURL + "/v1/status-list/revocation"
	now := time.Now().UTC()

	claims := statusListVCClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    issuerURL,
			Subject:   statusListURL,
			ID:        statusListURL,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(24 * time.Hour)),
		},
		VC: statusListVC{
			Context:   []string{"https://www.w3.org/ns/credentials/v2"},
			ID:        statusListURL,
			Type:      []string{"VerifiableCredential", "BitstringStatusListCredential"},
			Issuer:    issuerURL,
			ValidFrom: now.Format(time.RFC3339),
			CredentialSubject: statusListSubject{
				ID:            statusListURL + "#list",
				Type:          "BitstringStatusList",
				StatusPurpose: "revocation",
				EncodedList:   encodedList,
			},
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodES256, claims)
	token.Header["typ"] = "vc+ld+jwt"
	token.Header["kid"] = DefaultSigner.KeyID

	signed, err := token.SignedString(DefaultSigner.key)
	if err != nil {
		return "", fmt.Errorf("status list: sign: %w", err)
	}
	return signed, nil
}
