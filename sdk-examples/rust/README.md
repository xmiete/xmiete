# XMiete Rust SDK Example

A safe, high-performance Rust client example for the XMiete standard. Rust is well-suited for safety-critical financial backend systems.

## Structure

- **`models/`** — Rust structs using `serde` for type-safe JSON serialization/deserialization, aligned with `xmiete_schema.json`.
- **`client/`** — Asynchronous `XMieteClient` trait using `async-trait` and `tokio`.
- **`main.rs`** — Demonstrates basic deposit lifecycle interactions.

## Key Features

- **Memory safety** — Rust's ownership model eliminates memory-related bugs in financial data processing.
- **Async performance** — High-concurrency via `tokio` and zero-cost abstractions.
- **eIDAS 2.0 ready** — `WalletMetadata` struct covers PID, EAA, QEAA, and MDL credential types.
- **Full schema alignment** — All models derived from `xmiete_schema.json` via `serde`.
- **Provider agnostic** — eID verification is abstracted behind `EidVerifier`; swap providers without touching any other SDK code.

## Provider Agnosticism

The `eid` module exposes an `EidVerifier` trait rather than a concrete type. The built-in `EidVerificationService` targets any BSI TR-03130 compatible HTTP provider, but banks can supply their own adapter:

```rust
use async_trait::async_trait;
use xmiete_sdk::eid::{EidError, EidVerifier, KycUpdatePayload, VerificationRequest, VerificationSession};

struct MyProviderAdapter {
    // bank-specific config
}

#[async_trait]
impl EidVerifier for MyProviderAdapter {
    async fn initiate_verification(&self, req: &VerificationRequest) -> Result<VerificationSession, EidError> {
        // call your internal eID provider SDK or REST API
    }

    async fn update_deposit_kyc_status(&self, deposit_id: &str, payload: &KycUpdatePayload, bearer_token: &str) -> Result<(), EidError> {
        // push result to XMiete API
    }
}

// Wire it up — no other SDK code changes required.
let handler = WebhookHandler::new(Arc::new(MyProviderAdapter { … }), bearer_token, None);
```

Tested providers: Generic BSI TR-03130 HTTP (built-in), AusweisApp2 SDK, Authada, SkIDentity, Bundesdruckerei / D-Trust.

## Roadmap

- **QEAA Issuance Flow client** — Port the OID4VCI Pre-Authorized Code Flow to Rust: session management, SD-JWT credential building with ECDSA P-256 signing (`p256` crate), token exchange, and credential endpoint. Will mirror the reference implementation in `server/internal/issuance/`.

## How to Run

```bash
cargo run
```
