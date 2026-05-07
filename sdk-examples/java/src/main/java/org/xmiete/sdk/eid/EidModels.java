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

import java.time.OffsetDateTime;

public final class EidModels {

    private EidModels() {}

    public enum EidStatus {
        NOT_STARTED, PENDING, VERIFIED, FAILED, EXPIRED
    }

    /** Input to start a new eID verification session for a tenant. */
    public record EidVerificationRequest(
        String depositId,
        String tenantEmail,
        String redirectUri,
        String clientId
    ) {}

    /** Response from the eID provider after creating a session. */
    public record EidVerificationSession(
        String sessionId,
        String authorizationUrl,
        OffsetDateTime expiresAt
    ) {}

    /** Payload for POST /deposits/{id}/kyc — updates eid_status in XMiete. */
    public record KycUpdatePayload(
        EidStatus eidStatus,
        OffsetDateTime verificationTimestamp,
        /** Opaque reference from the eID provider; stored for audit, not PII. */
        String providerReference
    ) {}

    /** Incoming webhook event from the eID provider after user completes verification. */
    public record EidWebhookEvent(
        String sessionId,
        String depositId,
        EidStatus status,
        String providerReference,
        OffsetDateTime completedAt,
        String errorCode
    ) {}
}
