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

import java.util.concurrent.CompletableFuture;

/**
 * Abstraction over a BSI TR-03130 compatible eID provider.
 *
 * Implement this interface to plug in any eID provider — AusweisApp2 SDK,
 * Authada, SkIDentity, Bundesdruckerei / D-Trust — without changing any
 * other part of the SDK. The built-in {@link EidVerificationService} provides
 * a generic HTTP adapter as a reference implementation.
 */
public interface IdentityVerifier {

    /**
     * Creates an eID verification session. Redirect the tenant's browser to
     * {@link EidVerificationSession#authorizationUrl()} within the validity window.
     */
    CompletableFuture<EidVerificationSession> initiateVerification(EidVerificationRequest request);

    /**
     * Pushes the verified eID result to the XMiete API.
     * Only {@code providerReference} is forwarded — never raw PII from the eID chip.
     */
    CompletableFuture<Void> updateDepositKycStatus(
        String depositId,
        KycUpdatePayload payload,
        String bearerToken
    );
}
