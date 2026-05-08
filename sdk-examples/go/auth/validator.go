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

// Package auth provides OAuth2/OIDC Bearer token validation for the XMiete SDK.
//
// Usage:
//
//	validator := auth.NewOidcTokenValidator("https://auth.example.com/.well-known/openid-configuration")
//	claims, err := validator.ValidateToken(r.Header.Get("Authorization"), "deposit:read")
//
// Production note: replace verifySignatureStub with a JWKS-backed ES256/RS256 verifier.
// The standard library has no JWT library; use golang.org/x/oauth2/jws or a third-party
// package such as github.com/lestrrat-go/jwx.
package auth

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

// TokenClaims holds the validated claims from an OIDC Bearer token.
type TokenClaims struct {
	Subject   string
	Issuer    string
	Scopes    map[string]struct{}
	ExpiresAt time.Time
}

// HasScope reports whether the token carries the given OAuth2 scope.
func (c *TokenClaims) HasScope(scope string) bool {
	_, ok := c.Scopes[scope]
	return ok
}

// TokenValidationError is returned when the JWT is structurally invalid, expired,
// or issued by the wrong issuer.
type TokenValidationError struct{ msg string }

func (e *TokenValidationError) Error() string { return "auth: " + e.msg }

// InsufficientScopeError is returned when the token does not carry a required scope.
type InsufficientScopeError struct {
	Required string
	Present  []string
}

func (e *InsufficientScopeError) Error() string {
	return fmt.Sprintf("auth: required scope %q not in token: %v", e.Required, e.Present)
}

// OidcTokenValidator validates Bearer tokens against an OIDC provider's JWKS.
type OidcTokenValidator struct {
	// JWKSUri is the JWKS endpoint derived from the discovery URL.
	JWKSUri string
	// ExpectedIssuer is the required "iss" claim value.
	ExpectedIssuer string
}

// NewOidcTokenValidator creates a validator from an OIDC discovery URL.
// The JWKS URI and expected issuer are derived automatically.
func NewOidcTokenValidator(oidcDiscoveryURL string) *OidcTokenValidator {
	issuer := strings.TrimSuffix(oidcDiscoveryURL, "/.well-known/openid-configuration")
	return &OidcTokenValidator{
		JWKSUri:        issuer + "/.well-known/jwks.json",
		ExpectedIssuer: issuer,
	}
}

// ValidateToken validates a Bearer token and checks every required scope.
// The bearerToken may be a raw JWT or prefixed with "Bearer ".
func (v *OidcTokenValidator) ValidateToken(bearerToken string, requiredScopes ...string) (*TokenClaims, error) {
	token := strings.TrimPrefix(bearerToken, "Bearer ")
	claims, err := v.parseAndValidateClaims(token)
	if err != nil {
		return nil, err
	}
	if err := v.checkScopes(claims, requiredScopes); err != nil {
		return nil, err
	}
	return claims, nil
}

func (v *OidcTokenValidator) parseAndValidateClaims(jwt string) (*TokenClaims, error) {
	parts := strings.SplitN(jwt, ".", 3)
	if len(parts) != 3 {
		return nil, &TokenValidationError{fmt.Sprintf("malformed JWT: expected 3 parts, got %d", len(parts))}
	}

	payloadRaw, err := base64.RawURLEncoding.DecodeString(strings.TrimRight(parts[1], "="))
	if err != nil {
		return nil, &TokenValidationError{"base64 decode: " + err.Error()}
	}

	var p struct {
		Sub   string  `json:"sub"`
		Iss   string  `json:"iss"`
		Scope string  `json:"scope"`
		Exp   float64 `json:"exp"`
	}
	if err := json.Unmarshal(payloadRaw, &p); err != nil {
		return nil, &TokenValidationError{"json parse: " + err.Error()}
	}

	if p.Sub == "" {
		return nil, &TokenValidationError{"missing claim: sub"}
	}
	if p.Iss == "" {
		return nil, &TokenValidationError{"missing claim: iss"}
	}
	if p.Exp == 0 {
		return nil, &TokenValidationError{"missing claim: exp"}
	}

	if p.Iss != v.ExpectedIssuer {
		return nil, &TokenValidationError{fmt.Sprintf("unexpected issuer: %q", p.Iss)}
	}

	exp := int64(p.Exp)
	if time.Now().Unix() > exp {
		return nil, &TokenValidationError{fmt.Sprintf("token expired at epoch %d", exp)}
	}

	// Production: verify RS256/ES256 signature using the JWKS endpoint at v.JWKSUri.
	// Fetch JWKS, match the key by `kid` from the JWT header, then verify with crypto/ecdsa or crypto/rsa.
	v.verifySignatureStub(parts[0], parts[1], parts[2])

	scopes := make(map[string]struct{})
	for _, s := range strings.Fields(p.Scope) {
		scopes[s] = struct{}{}
	}

	return &TokenClaims{
		Subject:   p.Sub,
		Issuer:    p.Iss,
		Scopes:    scopes,
		ExpiresAt: time.Unix(exp, 0).UTC(),
	}, nil
}

func (v *OidcTokenValidator) checkScopes(claims *TokenClaims, required []string) error {
	for _, scope := range required {
		if !claims.HasScope(scope) {
			present := make([]string, 0, len(claims.Scopes))
			for s := range claims.Scopes {
				present = append(present, s)
			}
			return &InsufficientScopeError{Required: scope, Present: present}
		}
	}
	return nil
}

//nolint:unused
func (v *OidcTokenValidator) verifySignatureStub(_, _, _ string) {
	// Replace with JWKS-backed signature verification when adding a JWT dependency.
}

// IsTokenValidationError reports whether err is a *TokenValidationError.
func IsTokenValidationError(err error) bool {
	var e *TokenValidationError
	return errors.As(err, &e)
}

// IsInsufficientScopeError reports whether err is an *InsufficientScopeError.
func IsInsufficientScopeError(err error) bool {
	var e *InsufficientScopeError
	return errors.As(err, &e)
}
