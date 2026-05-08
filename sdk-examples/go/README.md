# XMiete Go SDK Example

A clean, idiomatic Go client example for the XMiete standard. Go is a popular choice for cloud-native microservices in the fintech sector.

## Structure

- **`models/`** — Go structs with `json` tags that map directly to `xmiete_schema.json`, including `WalletMetadata` for EUDI Wallet credential presentations.
- **`eid/`** — eID verification service: JWKS-authenticated identity update flow.
- **`client/`** — `XMieteClient` interface using `context.Context` for cancellation and timeouts.
- **`example_test.go`** — Demonstrates basic deposit lifecycle interactions.

## Key Features

- **Context support** — Request cancellation and deadlines throughout, essential for robust financial systems.
- **JWKS authentication** — Validates eID provider JWTs against a published JWKS endpoint.
- **eIDAS 2.0 ready** — `WalletMetadata` struct covers PID, EAA, QEAA, and MDL credential types.
- **Cloud-native** — Designed for containerized deployments (Kubernetes / Docker).

## Roadmap

- **QEAA Issuance Flow client** — Implement the bank-side OID4VCI Pre-Authorized Code Flow: create issuance sessions, generate SD-JWT `KautionsPfandNachweis` credentials, expose the token and credential endpoints. The reference implementation lives in the XMiete server (`server/internal/issuance/`); this SDK will expose it as a reusable library.

## How to Run

```bash
go test ./...
```
