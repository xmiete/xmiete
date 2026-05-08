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

import org.junit.jupiter.api.Test;
import org.xmiete.sdk.eid.EidModels.*;

import javax.crypto.Mac;
import javax.crypto.spec.SecretKeySpec;
import java.nio.charset.StandardCharsets;
import java.util.HexFormat;
import java.util.concurrent.CompletableFuture;
import java.util.concurrent.CountDownLatch;
import java.util.concurrent.TimeUnit;
import java.util.concurrent.atomic.AtomicInteger;

import static org.junit.jupiter.api.Assertions.*;

class EidWebhookHandlerTest {

    // Manual mock — satisfies IdentityVerifier without any framework dependency.
    static class MockVerifier implements IdentityVerifier {
        final AtomicInteger kycCalled = new AtomicInteger(0);

        @Override
        public CompletableFuture<EidVerificationSession> initiateVerification(EidVerificationRequest req) {
            throw new UnsupportedOperationException("not exercised by webhook tests");
        }

        @Override
        public CompletableFuture<Void> updateDepositKycStatus(String depositId, KycUpdatePayload payload, String bearerToken) {
            kycCalled.incrementAndGet();
            return CompletableFuture.completedFuture(null);
        }
    }

    private static String sign(String body, String secret) throws Exception {
        Mac mac = Mac.getInstance("HmacSHA256");
        mac.init(new SecretKeySpec(secret.getBytes(StandardCharsets.UTF_8), "HmacSHA256"));
        return HexFormat.of().formatHex(mac.doFinal(body.getBytes(StandardCharsets.UTF_8)));
    }

    @Test
    void verifiedEvent_callsKycUpdateAndFiresCallback() throws Exception {
        MockVerifier verifier = new MockVerifier();
        CountDownLatch latch = new CountDownLatch(1);
        EidWebhookHandler handler = new EidWebhookHandler(verifier, "tok", e -> latch.countDown());

        String body = """
            {"deposit_id":"DEP-1","status":"VERIFIED","provider_reference":"ref-abc"}
            """;
        handler.handleWebhook(body, sign(body, "secret"), "secret");

        assertTrue(latch.await(1, TimeUnit.SECONDS), "onVerificationComplete not called within timeout");
        assertEquals(1, verifier.kycCalled.get(), "updateDepositKycStatus should be called once");
    }

    @Test
    void failedEvent_doesNotCallKyc_butFiresCallback() throws Exception {
        MockVerifier verifier = new MockVerifier();
        AtomicInteger completeCalled = new AtomicInteger(0);
        EidWebhookHandler handler = new EidWebhookHandler(verifier, "tok", e -> completeCalled.incrementAndGet());

        String body = """
            {"deposit_id":"DEP-2","status":"FAILED","error_code":"DOCUMENT_EXPIRED"}
            """;
        handler.handleWebhook(body, sign(body, "s"), "s");

        assertEquals(0, verifier.kycCalled.get(), "updateDepositKycStatus must not be called for FAILED");
        assertEquals(1, completeCalled.get());
    }

    @Test
    void expiredEvent_doesNotCallKyc_butFiresCallback() throws Exception {
        MockVerifier verifier = new MockVerifier();
        AtomicInteger completeCalled = new AtomicInteger(0);
        EidWebhookHandler handler = new EidWebhookHandler(verifier, "tok", e -> completeCalled.incrementAndGet());

        String body = """
            {"deposit_id":"DEP-3","status":"EXPIRED"}
            """;
        handler.handleWebhook(body, sign(body, "s"), "s");

        assertEquals(0, verifier.kycCalled.get());
        assertEquals(1, completeCalled.get());
    }

    @Test
    void nullSignature_throwsInvalidWebhookSignatureException() {
        EidWebhookHandler handler = new EidWebhookHandler(new MockVerifier(), "tok", e -> {});
        String body = "{\"deposit_id\":\"DEP-1\",\"status\":\"VERIFIED\"}";

        assertThrows(EidWebhookHandler.InvalidWebhookSignatureException.class,
            () -> handler.handleWebhook(body, null, "secret"));
    }

    @Test
    void blankSignature_throwsInvalidWebhookSignatureException() {
        EidWebhookHandler handler = new EidWebhookHandler(new MockVerifier(), "tok", e -> {});
        String body = "{\"deposit_id\":\"DEP-1\",\"status\":\"VERIFIED\"}";

        assertThrows(EidWebhookHandler.InvalidWebhookSignatureException.class,
            () -> handler.handleWebhook(body, "   ", "secret"));
    }

    @Test
    void wrongSecret_throwsInvalidWebhookSignatureException() throws Exception {
        EidWebhookHandler handler = new EidWebhookHandler(new MockVerifier(), "tok", e -> {});
        String body = "{\"deposit_id\":\"DEP-1\",\"status\":\"VERIFIED\"}";
        String badSig = sign(body, "wrong-secret");

        assertThrows(EidWebhookHandler.InvalidWebhookSignatureException.class,
            () -> handler.handleWebhook(body, badSig, "correct-secret"));
    }

    @Test
    void missingDepositId_throwsMalformedWebhookException() throws Exception {
        EidWebhookHandler handler = new EidWebhookHandler(new MockVerifier(), "tok", e -> {});
        String body = "{\"status\":\"VERIFIED\"}";
        assertThrows(EidWebhookHandler.MalformedWebhookException.class,
            () -> handler.handleWebhook(body, sign(body, "s"), "s"));
    }

    @Test
    void unknownStatus_throwsMalformedWebhookException() throws Exception {
        EidWebhookHandler handler = new EidWebhookHandler(new MockVerifier(), "tok", e -> {});
        String body = "{\"deposit_id\":\"DEP-1\",\"status\":\"BOGUS\"}";
        assertThrows(EidWebhookHandler.MalformedWebhookException.class,
            () -> handler.handleWebhook(body, sign(body, "s"), "s"));
    }

    @Test
    void customAdapter_plugsInWithoutSdkChanges() throws Exception {
        // Provider agnosticism: any IdentityVerifier implementation wires into
        // EidWebhookHandler without modifying any other SDK code.
        class CustomAdapter implements IdentityVerifier {
            final AtomicInteger kycCalled = new AtomicInteger(0);

            @Override
            public CompletableFuture<EidVerificationSession> initiateVerification(EidVerificationRequest req) {
                throw new UnsupportedOperationException();
            }

            @Override
            public CompletableFuture<Void> updateDepositKycStatus(String d, KycUpdatePayload p, String t) {
                kycCalled.incrementAndGet();
                return CompletableFuture.completedFuture(null);
            }
        }

        CustomAdapter adapter = new CustomAdapter();
        CountDownLatch latch = new CountDownLatch(1);
        EidWebhookHandler handler = new EidWebhookHandler(adapter, "tok", e -> latch.countDown());

        String body = "{\"deposit_id\":\"DEP-9\",\"status\":\"VERIFIED\"}";
        handler.handleWebhook(body, sign(body, "s"), "s");

        assertTrue(latch.await(1, TimeUnit.SECONDS), "onVerificationComplete not called");
        assertEquals(1, adapter.kycCalled.get());
    }
}
