/*
 * Copyright 2026 XMiete Core Contributors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 */

package org.xmiete.sdk.openid4vp;

import java.util.concurrent.CompletableFuture;

/**
 * Implement this interface to add OpenID4VP presentation request/verify capability
 * to any XMiete verifier (landlord app, property management system, etc.).
 *
 * The included {@link OpenId4VpService} provides a complete implementation.
 * Custom implementations can integrate with W3C Verifiable Credential frameworks
 * (e.g., Spruce DIDKit, Walt.id) without touching any other part of the SDK.
 */
public interface VpVerifier {

    /**
     * Builds a VP request for a KautionsPfandNachweis presentation.
     * Returns a nonce (first element) and the serialized VpRequest JSON (second element).
     * The caller must store the nonce to pass to {@link #verifyVpToken} later.
     */
    CompletableFuture<VpRequestResult> buildVpRequest(String depositId, String responseUri);

    /**
     * Verifies a vp_token received from the wallet.
     *
     * @param vpToken       SD-JWT VP wire format: {@code issuerJWT~disc1~...~discN~kbJWT}
     * @param expectedNonce nonce from the corresponding VP request
     * @param responseUri   the response_uri from the VP request (used as KB-JWT aud check)
     */
    CompletableFuture<OpenId4VpModels.VerifiedClaims> verifyVpToken(
        String vpToken,
        String expectedNonce,
        String responseUri
    );

    record VpRequestResult(String nonce, OpenId4VpModels.VpRequest vpRequest) {}
}
