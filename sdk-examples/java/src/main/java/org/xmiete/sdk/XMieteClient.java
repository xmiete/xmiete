/*
 * Copyright 2024 XMiete Core Contributors
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

package org.xmiete.sdk;

import org.xmiete.sdk.eid.EidModels.KycUpdatePayload;
import org.xmiete.sdk.models.Deposit;
import java.util.concurrent.CompletableFuture;

/**
 * Robust Java Client for the XMiete API.
 * In a real-world scenario, this would be implemented using
 * Spring RestTemplate, Feign, or Java 11 HttpClient.
 */
public interface XMieteClient {

    /**
     * Creates a new deposit request.
     */
    CompletableFuture<Deposit> createDeposit(Deposit request);

    /**
     * Retrieves the current state of a deposit.
     */
    CompletableFuture<Deposit> getDeposit(String depositId);

    /**
     * Confirms the legal pledge (BGB § 551).
     */
    CompletableFuture<Void> confirmPledge(String depositId, String pledgeDate);

    /**
     * Authorizes the release of a deposit.
     */
    CompletableFuture<Void> releaseDeposit(String depositId, String signatureToken);

    /**
     * Updates the eID/KYC status after successful identity verification.
     * Requires scope: deposit:create or deposit:read (tenant/fintech role).
     * Moves lifecycle state to IDENTIFIED when eid_status becomes VERIFIED.
     */
    CompletableFuture<Void> updateKycStatus(String depositId, KycUpdatePayload payload);
}
