package org.xmiete.sdk;

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
}
