package org.xmiete.sdk;

import org.xmiete.sdk.models.Deposit;

/**
 * Demonstrates how a bank or property manager would use the SDK.
 */
public class ExampleUsage {

    public static void main(String[] args) {
        // In a real app, this would be an actual HTTP implementation
        XMieteClient client = new MockXMieteClient();

        System.out.println("Initiating XMiete Deposit Process...");

        // 1. Fetch a deposit by ID
        client.getDeposit("DEP-123")
            .thenAccept(deposit -> {
                System.out.println("Found Deposit for: " + deposit.tenant().lastName());
                System.out.println("Current State: " + deposit.depositDetails().lifecycleState());
            })
            .join();

        // 2. Confirm a Pledge (Bank Side)
        System.out.println("Bank confirming legal pledge...");
        client.confirmPledge("DEP-123", "2026-05-07")
            .thenRun(() -> System.out.println("Pledge successfully registered in XMiete System."))
            .join();
    }

    /**
     * Minimal mock implementation for demonstration.
     */
    static class MockXMieteClient implements XMieteClient {
        @Override
        public java.util.concurrent.CompletableFuture<Deposit> createDeposit(Deposit r) { 
            return java.util.concurrent.CompletableFuture.completedFuture(r); 
        }

        @Override
        public java.util.concurrent.CompletableFuture<Deposit> getDeposit(String id) {
            // Return dummy data for demo
            return java.util.concurrent.CompletableFuture.completedFuture(null); 
        }

        @Override
        public java.util.concurrent.CompletableFuture<Void> confirmPledge(String id, String date) {
            return java.util.concurrent.CompletableFuture.completedFuture(null);
        }

        @Override
        public java.util.concurrent.CompletableFuture<Void> releaseDeposit(String id, String tok) {
            return java.util.concurrent.CompletableFuture.completedFuture(null);
        }
    }
}
