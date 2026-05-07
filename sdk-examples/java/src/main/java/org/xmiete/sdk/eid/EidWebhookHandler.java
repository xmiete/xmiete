/*
 * Copyright 2024 XMiete Core Contributors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 */

package org.xmiete.sdk.eid;

import org.xmiete.sdk.eid.EidModels.*;

import javax.crypto.Mac;
import javax.crypto.spec.SecretKeySpec;
import java.nio.charset.StandardCharsets;
import java.security.InvalidKeyException;
import java.security.MessageDigest;
import java.security.NoSuchAlgorithmException;
import java.time.OffsetDateTime;
import java.util.HexFormat;
import java.util.function.Consumer;
import java.util.logging.Logger;

/**
 * Processes signed webhook events from the eID provider.
 *
 * Mount on your HTTP server at the endpoint registered as redirect_uri.
 * The provider sends a POST with:
 *   - Body: JSON EidWebhookEvent
 *   - Header X-Signature: HMAC-SHA256(body, sharedSecret) as lowercase hex
 *
 * Example (Spring Boot):
 *   @PostMapping("/webhook/eid")
 *   public ResponseEntity<Void> eidCallback(
 *       @RequestBody String body,
 *       @RequestHeader("X-Signature") String sig
 *   ) {
 *       handler.handleWebhook(body, sig, webhookSecret);
 *       return ResponseEntity.ok().build();
 *   }
 */
public class EidWebhookHandler {

    private static final Logger log = Logger.getLogger(EidWebhookHandler.class.getName());

    private final EidVerificationService verificationService;
    private final String xmieteBearerToken;
    private final Consumer<EidWebhookEvent> onVerificationComplete;

    public EidWebhookHandler(
        EidVerificationService verificationService,
        String xmieteBearerToken,
        Consumer<EidWebhookEvent> onVerificationComplete
    ) {
        this.verificationService = verificationService;
        this.xmieteBearerToken = xmieteBearerToken;
        this.onVerificationComplete = onVerificationComplete;
    }

    /**
     * Validates the webhook signature, parses the event, and dispatches the result.
     *
     * @param rawBody      raw UTF-8 request body from the eID provider
     * @param signature    X-Signature header value (HMAC-SHA256 hex)
     * @param webhookSecret shared secret configured with the eID provider
     */
    public void handleWebhook(String rawBody, String signature, String webhookSecret) {
        verifyHmacSignature(rawBody, signature, webhookSecret);
        EidWebhookEvent event = parseWebhookEvent(rawBody);
        dispatchEvent(event);
    }

    private void dispatchEvent(EidWebhookEvent event) {
        switch (event.status()) {
            case VERIFIED -> {
                KycUpdatePayload kycPayload = new KycUpdatePayload(
                    EidStatus.VERIFIED,
                    event.completedAt() != null ? event.completedAt() : OffsetDateTime.now(),
                    event.providerReference()
                );
                verificationService
                    .updateDepositKycStatus(event.depositId(), kycPayload, xmieteBearerToken)
                    .thenRun(() -> {
                        log.info("KYC updated for deposit %s (ref: %s)"
                            .formatted(event.depositId(), event.providerReference()));
                        onVerificationComplete.accept(event);
                    })
                    .exceptionally(ex -> {
                        log.severe("Failed to update KYC for deposit %s: %s"
                            .formatted(event.depositId(), ex.getMessage()));
                        return null;
                    });
            }
            case FAILED, EXPIRED -> {
                log.warning("eID verification %s for deposit %s — error: %s"
                    .formatted(event.status(), event.depositId(), event.errorCode()));
                onVerificationComplete.accept(event);
            }
            default -> log.fine("Unhandled eID event status: " + event.status());
        }
    }

    private void verifyHmacSignature(String body, String signature, String secret) {
        if (signature == null || signature.isBlank()) {
            throw new InvalidWebhookSignatureException("Missing X-Signature header");
        }
        try {
            Mac mac = Mac.getInstance("HmacSHA256");
            mac.init(new SecretKeySpec(secret.getBytes(StandardCharsets.UTF_8), "HmacSHA256"));
            byte[] expected = mac.doFinal(body.getBytes(StandardCharsets.UTF_8));
            String expectedHex = HexFormat.of().formatHex(expected);
            // Constant-time comparison to prevent timing attacks
            if (!MessageDigest.isEqual(expectedHex.getBytes(), signature.getBytes())) {
                throw new InvalidWebhookSignatureException("Signature mismatch");
            }
        } catch (NoSuchAlgorithmException | InvalidKeyException e) {
            throw new InvalidWebhookSignatureException("Signature verification error: " + e.getMessage());
        }
    }

    /** Minimal JSON parsing — production: use Jackson or Gson. */
    private EidWebhookEvent parseWebhookEvent(String json) {
        String sessionId = extractJsonString(json, "session_id");
        String depositId = extractJsonString(json, "deposit_id");
        String statusStr = extractJsonString(json, "status");
        String providerRef = extractJsonString(json, "provider_reference");
        String errorCode = extractJsonString(json, "error_code");

        if (depositId == null || statusStr == null) {
            throw new MalformedWebhookException("Webhook missing required fields: deposit_id, status");
        }

        EidStatus status;
        try {
            status = EidStatus.valueOf(statusStr);
        } catch (IllegalArgumentException e) {
            throw new MalformedWebhookException("Unknown eID status: " + statusStr);
        }

        return new EidWebhookEvent(sessionId, depositId, status, providerRef, OffsetDateTime.now(), errorCode);
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

    public static class InvalidWebhookSignatureException extends RuntimeException {
        public InvalidWebhookSignatureException(String msg) { super(msg); }
    }

    public static class MalformedWebhookException extends RuntimeException {
        public MalformedWebhookException(String msg) { super(msg); }
    }
}
