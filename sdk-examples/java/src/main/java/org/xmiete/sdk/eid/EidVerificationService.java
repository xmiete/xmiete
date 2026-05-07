/*
 * Copyright 2026 XMiete Core Contributors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 */

package org.xmiete.sdk.eid;

import org.xmiete.sdk.eid.EidModels.*;

import java.net.URI;
import java.net.http.HttpClient;
import java.net.http.HttpRequest;
import java.net.http.HttpResponse;
import java.time.OffsetDateTime;
import java.util.UUID;
import java.util.concurrent.CompletableFuture;

/**
 * Manages eID verification sessions against a BSI TR-03130 compatible eID provider
 * (e.g., Bundesdruckerei AusweisApp2 SDK, Authada, or SkIDentity).
 *
 * Flow:
 *   1. Call initiateVerification() → get EidVerificationSession with an authorizationUrl
 *   2. Redirect the tenant's browser to authorizationUrl
 *   3. The eID provider calls your webhook → handled by EidWebhookHandler
 *   4. EidWebhookHandler calls updateDepositKycStatus() to finalize the XMiete deposit state
 */
public class EidVerificationService {

    private final HttpClient httpClient;
    private final String eidProviderBaseUrl;
    private final String xmieteApiBaseUrl;

    public EidVerificationService(String eidProviderBaseUrl, String xmieteApiBaseUrl) {
        this.httpClient = HttpClient.newHttpClient();
        this.eidProviderBaseUrl = eidProviderBaseUrl;
        this.xmieteApiBaseUrl = xmieteApiBaseUrl;
    }

    /**
     * Creates an eID verification session. Redirect the tenant to
     * {@link EidVerificationSession#authorizationUrl()} within the session's validity window.
     */
    public CompletableFuture<EidVerificationSession> initiateVerification(EidVerificationRequest request) {
        String body = """
            {
              "client_id": "%s",
              "deposit_id": "%s",
              "tenant_email": "%s",
              "redirect_uri": "%s",
              "scope": "openid eid"
            }
            """.formatted(
                request.clientId(),
                request.depositId(),
                request.tenantEmail(),
                request.redirectUri()
            );

        HttpRequest httpRequest = HttpRequest.newBuilder()
            .uri(URI.create(eidProviderBaseUrl + "/sessions"))
            .header("Content-Type", "application/json")
            .POST(HttpRequest.BodyPublishers.ofString(body))
            .build();

        return httpClient.sendAsync(httpRequest, HttpResponse.BodyHandlers.ofString())
            .thenApply(response -> {
                if (response.statusCode() != 201) {
                    throw new EidProviderException(
                        "Failed to create eID session: HTTP " + response.statusCode());
                }
                // Production: parse JSON response body for sessionId and authorizationUrl.
                String sessionId = extractJsonString(response.body(), "session_id");
                if (sessionId == null) sessionId = UUID.randomUUID().toString();
                String authUrl = eidProviderBaseUrl + "/authorize?session_id=" + sessionId
                    + "&deposit_id=" + request.depositId();
                return new EidVerificationSession(sessionId, authUrl, OffsetDateTime.now().plusMinutes(15));
            });
    }

    /**
     * Pushes the verified eID result to the XMiete API (POST /deposits/{id}/kyc).
     * Only the {@code providerReference} is stored — never raw PII from the eID chip.
     */
    public CompletableFuture<Void> updateDepositKycStatus(
        String depositId,
        KycUpdatePayload payload,
        String bearerToken
    ) {
        String body = """
            {
              "eid_status": "%s",
              "verification_timestamp": "%s",
              "provider_reference": "%s"
            }
            """.formatted(
                payload.eidStatus().name(),
                payload.verificationTimestamp().toString(),
                payload.providerReference()
            );

        HttpRequest request = HttpRequest.newBuilder()
            .uri(URI.create(xmieteApiBaseUrl + "/deposits/" + depositId + "/kyc"))
            .header("Content-Type", "application/json")
            .header("Authorization", "Bearer " + bearerToken)
            .POST(HttpRequest.BodyPublishers.ofString(body))
            .build();

        return httpClient.sendAsync(request, HttpResponse.BodyHandlers.discarding())
            .thenApply(response -> {
                if (response.statusCode() != 200 && response.statusCode() != 204) {
                    throw new EidProviderException("KYC update failed: HTTP " + response.statusCode());
                }
                return null;
            });
    }

    private static String extractJsonString(String json, String key) {
        if (json == null) return null;
        int idx = json.indexOf("\"" + key + "\"");
        if (idx < 0) return null;
        int colon = json.indexOf(':', idx);
        int start = json.indexOf('"', colon + 1);
        if (start < 0) return null;
        int end = json.indexOf('"', start + 1);
        return end > start ? json.substring(start + 1, end) : null;
    }

    public static class EidProviderException extends RuntimeException {
        public EidProviderException(String msg) { super(msg); }
    }
}
