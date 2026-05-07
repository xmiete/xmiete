/*
 * Copyright 2024 XMiete Core Contributors
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

import java.time.OffsetDateTime;
import java.util.concurrent.CompletableFuture;

/**
 * Demonstrates the full XMiete flow including OIDC token validation and eID verification.
 *
 * Scenario: a PropTech platform (Fintech role) onboards a new tenant.
 *   1. Validate the API caller's OAuth2 token and scopes
 *   2. Initiate an eID verification session for the tenant
 *   3. Process the eID provider's webhook callback
 *   4. KYC status is updated → deposit transitions to IDENTIFIED
 */
public class ExampleUsage {

    public static void main(String[] args) {

        // --- 1. OIDC Token Validation ----------------------------------------

        var validator = new OidcTokenValidator(
            "https://auth.xmiete.example/.well-known/openid-configuration"
        );

        // Simulate a Bearer token arriving in an HTTP Authorization header.
        // In production this comes from the incoming HTTP request.
        String incomingAuthHeader = "Bearer eyJhbGciOiJSUzI1NiIsImtpZCI6ImtleS0xIn0"
            + ".eyJzdWIiOiJmaW50ZWNoLWFwcC0xIiwiaXNzIjoiaHR0cHM6Ly9hdXRoLnhtaWV0ZS5leGFtcGxlIiwic2NvcGUiOiJkZXBvc2l0OnJlYWQgZGVwb3NpdDpjcmVhdGUiLCJleHAiOjk5OTk5OTk5OTl9"
            + ".signature-stub";

        try {
            TokenClaims claims = validator.validateToken(incomingAuthHeader, "deposit:read", "deposit:create");
            System.out.println("Token valid — subject: " + claims.subject()
                + ", scopes: " + claims.scopes());
        } catch (OidcTokenValidator.TokenValidationException e) {
            System.err.println("Token rejected: " + e.getMessage());
            return;
        } catch (OidcTokenValidator.InsufficientScopeException e) {
            System.err.println("Access denied: " + e.getMessage());
            return;
        }

        // --- 2. eID Verification Session -------------------------------------

        var eidService = new EidVerificationService(
            "https://eid-provider.example",   // e.g., Authada or SkIDentity
            "https://api.xmiete.example/v1"
        );

        var verificationRequest = new EidVerificationRequest(
            "DEP-123",
            "max.mustermann@example.de",
            "https://app.xmiete.example/eid-callback",
            "xmiete-fintech-client"
        );

        System.out.println("Initiating eID verification for deposit DEP-123...");
        EidVerificationSession session = eidService
            .initiateVerification(verificationRequest)
            .exceptionally(ex -> {
                // Demo fallback — eID provider not reachable in this example
                System.out.println("(Demo) eID provider unavailable, using stub session");
                return new EidVerificationSession(
                    "SESSION-STUB-456",
                    "https://eid-provider.example/authorize?session_id=SESSION-STUB-456",
                    OffsetDateTime.now().plusMinutes(15)
                );
            })
            .join();

        System.out.println("eID session created: " + session.sessionId());
        System.out.println("Redirect tenant to: " + session.authorizationUrl());
        System.out.println("Session valid until: " + session.expiresAt());

        // --- 3. Webhook from eID Provider ------------------------------------

        // In production this arrives as an HTTP POST to your webhook endpoint.
        // Here we simulate the provider calling back after the tenant verified their eID.
        String simulatedWebhookBody = """
            {
              "session_id": "SESSION-STUB-456",
              "deposit_id": "DEP-123",
              "status": "VERIFIED",
              "provider_reference": "EID-AUTHADA-789XYZ",
              "error_code": null
            }
            """;

        // Shared secret registered with the eID provider (store in env/secrets manager)
        String webhookSecret = System.getenv().getOrDefault("EID_WEBHOOK_SECRET", "dev-secret-only");
        String simulatedSignature = computeHmacStub(simulatedWebhookBody, webhookSecret);

        var webhookHandler = new EidWebhookHandler(
            eidService,
            "Bearer <xmiete-service-token>",  // service account token with deposit:create scope
            event -> System.out.println("Verification complete — status: " + event.status()
                + ", providerRef: " + event.providerReference())
        );

        System.out.println("Processing eID webhook...");
        try {
            webhookHandler.handleWebhook(simulatedWebhookBody, simulatedSignature, webhookSecret);
        } catch (EidWebhookHandler.InvalidWebhookSignatureException e) {
            System.err.println("Webhook rejected — invalid signature: " + e.getMessage());
        }

        // --- 4. Fetch Updated Deposit ----------------------------------------

        XMieteClient client = new MockXMieteClient();
        client.getDeposit("DEP-123")
            .thenAccept(deposit -> {
                if (deposit != null) {
                    System.out.println("Deposit state: " + deposit.depositDetails().lifecycleState());
                    System.out.println("eID status: " + deposit.tenant().eidStatus());
                }
            })
            .join();

        System.out.println("Flow complete.");
    }

    /** Stub: real implementation uses javax.crypto.Mac as in EidWebhookHandler. */
    private static String computeHmacStub(String body, String secret) {
        return "stub-hmac-for-demo";
    }


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
        public CompletableFuture<Void> updateKycStatus(String depositId, org.xmiete.sdk.eid.EidModels.KycUpdatePayload payload) {
            System.out.println("(Mock) KYC updated for " + depositId + " → " + payload.eidStatus());
            return CompletableFuture.completedFuture(null);
        }
    }
}
