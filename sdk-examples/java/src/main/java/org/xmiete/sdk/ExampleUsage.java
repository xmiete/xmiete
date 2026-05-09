/*
 * Copyright 2026 XMiete Core Contributors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 */

package org.xmiete.sdk;

import org.xmiete.sdk.auth.OidcTokenValidator;
import org.xmiete.sdk.auth.TokenClaims;
import org.xmiete.sdk.eid.EidModels.*;
import org.xmiete.sdk.eid.EidVerificationService;
import org.xmiete.sdk.eid.EidWebhookHandler;
import org.xmiete.sdk.models.Deposit;
import org.xmiete.sdk.openid4vp.OpenId4VpModels.*;
import org.xmiete.sdk.openid4vp.OpenId4VpService;

import java.time.OffsetDateTime;
import java.util.concurrent.CompletableFuture;

/**
 * Demonstrates the full XMiete flow including OIDC token validation, eID verification,
 * webhook handling, and OpenID4VP wallet credential presentation.
 *
 * Scenario: a PropTech platform onboards a tenant and lets a landlord verify the pledge
 * via an eIDAS 2.0 EUDI Wallet.
 */
public class ExampleUsage {

    public static void main(String[] args) {

        // ── 1. OIDC Token Validation ──────────────────────────────────────────
        // docs:start:auth-java
        var validator = new OidcTokenValidator(
            "https://auth.xmiete.example/.well-known/openid-configuration"
        );

        // In production, pass request.getHeader("Authorization").
        String incomingAuthHeader = "Bearer <token>";

        try {
            TokenClaims claims = validator.validateToken(
                incomingAuthHeader, "deposit:read", "deposit:create"
            );
            System.out.println("Token valid — sub: " + claims.subject()
                + ", scopes: " + claims.scopes());
        } catch (OidcTokenValidator.TokenValidationException e) {
            // HTTP 401 — malformed JWT, expired, or wrong issuer
            System.err.println("Token rejected: " + e.getMessage());
            return;
        } catch (OidcTokenValidator.InsufficientScopeException e) {
            // HTTP 403 — valid token, scope not granted
            System.err.println("Access denied: " + e.getMessage());
            return;
        }
        // docs:end:auth-java

        // ── 2. eID Verification Session ───────────────────────────────────────
        // docs:start:eid-java
        var eidService = new EidVerificationService(
            "https://eid-provider.example",  // e.g., Authada, SkIDentity
            "https://api.xmiete.org/v1"
        );

        var verificationRequest = new EidVerificationRequest(
            "DEP-123",
            "max.mustermann@example.de",
            "https://app.xmiete.example/eid-callback",
            "xmiete-fintech-client"
        );

        EidVerificationSession session = eidService
            .initiateVerification(verificationRequest)
            .exceptionally(ex -> new EidVerificationSession(
                "SESSION-STUB-456",
                "https://eid-provider.example/authorize?session_id=SESSION-STUB-456",
                OffsetDateTime.now().plusMinutes(15)
            ))
            .join();

        System.out.println("Redirect tenant to: " + session.authorizationUrl());
        // The eID provider will POST the result to your /webhook/eid endpoint.
        // docs:end:eid-java

        // ── 3. Webhook from eID Provider ──────────────────────────────────────
        // docs:start:webhook-java
        String webhookSecret = System.getenv().getOrDefault("EID_WEBHOOK_SECRET", "dev-secret-only");

        var webhookHandler = new EidWebhookHandler(
            eidService,
            "Bearer <xmiete-service-token>",
            event -> System.out.println("eID done — status: " + event.status()
                + ", providerRef: " + event.providerReference())
        );

        // In your servlet / Spring @PostMapping:
        //   byte[] body = request.getInputStream().readAllBytes();
        //   String sig  = request.getHeader("X-Signature");
        //   webhookHandler.handleWebhook(body, sig, webhookSecret);
        // docs:end:webhook-java

        // ── 4. OpenID4VP — Wallet Credential Presentation ────────────────────
        // docs:start:openid4vp-java
        var vpVerifier = new OpenId4VpService(
            "https://verifier.yourapp.example.com",
            "https://auth.example.com/.well-known/jwks.json"
        );

        // Step 1 — build a VP request and deliver it to the wallet (QR or deep-link).
        String responseUri = "https://yourapp.example.com/vp-response";
        VpRequestResult req = vpVerifier.buildVpRequest("DEP-123", responseUri).join();
        String nonce = req.nonce();
        // Persist nonce; serialize req.vpRequest() as JSON and embed in QR code.

        // Step 3 — wallet POSTs vp_token to responseUri; verify it here.
        VerifiedClaims vp = vpVerifier.verifyVpToken("<vp_token>", nonce, responseUri).join();
        System.out.println("Deposit: " + vp.depositId());
        System.out.println("Pledge date: " + vp.pledgeDate());
        System.out.println("Issuing bank: " + vp.issuingBank());
        // docs:end:openid4vp-java

        System.out.println("Flow complete.");
    }

    // ── Mock Client ───────────────────────────────────────────────────────────

    static class MockXMieteClient implements XMieteClient {

        @Override
        public CompletableFuture<Deposit> createDeposit(Deposit r) {
            return CompletableFuture.completedFuture(r);
        }

        @Override
        public CompletableFuture<Deposit> getDeposit(String id) {
            return CompletableFuture.completedFuture(null);
        }

        @Override
        public CompletableFuture<Void> confirmPledge(String id, String date) {
            return CompletableFuture.completedFuture(null);
        }

        @Override
        public CompletableFuture<Void> releaseDeposit(String id, String tok) {
            return CompletableFuture.completedFuture(null);
        }

        @Override
        public CompletableFuture<Void> updateKycStatus(String depositId,
                org.xmiete.sdk.eid.EidModels.KycUpdatePayload payload) {
            System.out.println("(Mock) KYC updated for " + depositId + " → " + payload.eidStatus());
            return CompletableFuture.completedFuture(null);
        }
    }
}
