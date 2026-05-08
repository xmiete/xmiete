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

## Roadmap

- **QEAA Issuance Flow client** — Port the OID4VCI Pre-Authorized Code Flow to Rust: session management, SD-JWT credential building with ECDSA P-256 signing (`p256` crate), token exchange, and credential endpoint. Will mirror the reference implementation in `server/internal/issuance/`.

## How to Run

```bash
cargo run
```
