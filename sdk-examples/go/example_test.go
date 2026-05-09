//go:build ignore

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

// Usage examples for the XMiete Go SDK.
// This file carries //go:build ignore and is excluded from normal builds and go test.
// Each function demonstrates one integration pattern end-to-end.
package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"

	"github.com/xmiete/xmiete-go-sdk/auth"
	"github.com/xmiete/xmiete-go-sdk/eid"
	"github.com/xmiete/xmiete-go-sdk/openid4vp"
)

// ── Authentication ────────────────────────────────────────────────────────────

// docs:start:auth-go
func handleRequest(w http.ResponseWriter, r *http.Request) {
	validator := auth.NewOidcTokenValidator(
		"https://auth.xmiete.example/.well-known/openid-configuration",
	)
	claims, err := validator.ValidateToken(r.Header.Get("Authorization"), "deposit:create")
	if err != nil {
		if auth.IsInsufficientScopeError(err) {
			http.Error(w, "forbidden", http.StatusForbidden)
		} else {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
		}
		return
	}
	log.Printf("authenticated: sub=%s expires=%s", claims.Subject, claims.ExpiresAt)
	// ... process the request
}
// docs:end:auth-go

// ── eID Verification ──────────────────────────────────────────────────────────

// docs:start:eid-go
func initiateEIDVerification(ctx context.Context, depositID, tenantEmail string) (string, error) {
	service := eid.NewVerificationService(
		"https://eid-provider.example.com", // e.g., Authada, SkIDentity
		"https://api.xmiete.org/v1",
	)
	session, err := service.InitiateVerification(ctx, eid.VerificationRequest{
		DepositID:   depositID,
		TenantEmail: tenantEmail,
		RedirectURI: "https://yourapp.example.com/eid-callback",
		ClientID:    "xmiete-fintech-client",
	})
	if err != nil {
		return "", fmt.Errorf("eid: initiate session: %w", err)
	}
	// Redirect the tenant's browser to session.AuthorizationURL.
	// The provider POSTs the result to your /webhook/eid endpoint.
	return session.AuthorizationURL, nil
}
// docs:end:eid-go

// ── Webhook Handling ──────────────────────────────────────────────────────────

// docs:start:webhook-go
func registerEIDWebhook(service eid.IdentityVerifier, bearerToken, webhookSecret string) {
	handler := eid.NewWebhookHandler(service, bearerToken, func(event eid.WebhookEvent) {
		log.Printf("eID done: deposit=%s status=%s", event.DepositID, event.Status)
	})

	http.HandleFunc("/webhook/eid", func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if err := handler.HandleWebhook(body, r.Header.Get("X-Signature"), webhookSecret); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusOK)
	})
}
// docs:end:webhook-go

// ── OpenID4VP ─────────────────────────────────────────────────────────────────

// docs:start:openid4vp-go
func requestWalletPresentation(ctx context.Context, depositID, responseURI string) error {
	verifier := openid4vp.NewVpVerifierService(
		"https://verifier.yourapp.example.com",
		"https://auth.example.com/.well-known/jwks.json",
	)

	// Step 1 — build the VP request and deliver it to the wallet (QR or deep-link).
	result, err := verifier.BuildVpRequest(ctx, depositID, responseURI)
	if err != nil {
		return fmt.Errorf("openid4vp: build request: %w", err)
	}
	// Persist result.Nonce in your session store — required for step 3.
	// Serialize result.VpRequest as JSON and embed in QR code or deep-link.

	// Step 3 — wallet POSTs vp_token to responseURI; verify it here.
	claims, err := verifier.VerifyVpToken(ctx, "<vp_token from wallet>", result.Nonce, responseURI)
	if err != nil {
		return fmt.Errorf("openid4vp: invalid presentation: %w", err)
	}
	log.Printf("verified: deposit=%s pledged=%s bank=%s",
		claims.DepositID, claims.PledgeDate, claims.IssuingBank)
	return nil
}
// docs:end:openid4vp-go

// ── Error Handling ────────────────────────────────────────────────────────────

// docs:start:error-go
func handleAuthError(w http.ResponseWriter, err error) {
	switch {
	case auth.IsInsufficientScopeError(err):
		// HTTP 403 — valid token, but the required scope is not granted
		http.Error(w, "insufficient scope", http.StatusForbidden)
	case auth.IsTokenValidationError(err):
		// HTTP 401 — malformed JWT, expired, or wrong issuer
		http.Error(w, "unauthorized", http.StatusUnauthorized)
	default:
		http.Error(w, "internal error", http.StatusInternalServerError)
	}
}
// docs:end:error-go

func main() {}
