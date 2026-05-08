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
- **Provider agnostic** — eID verification is abstracted behind `IdentityVerifier`; swap providers without touching any other SDK code.

## Provider Agnosticism

The `eid` package exposes an `IdentityVerifier` interface rather than a concrete type. The built-in `VerificationService` targets any BSI TR-03130 compatible HTTP provider, but banks can supply their own adapter:

```go
type MyProviderAdapter struct {
    // bank-specific config
}

func (a *MyProviderAdapter) InitiateVerification(ctx context.Context, req eid.VerificationRequest) (*eid.VerificationSession, error) {
    // call your internal eID provider SDK or REST API
}

func (a *MyProviderAdapter) UpdateDepositKYCStatus(ctx context.Context, depositID string, payload eid.KYCUpdatePayload, bearerToken string) error {
    // push result to XMiete API
}

// Wire it up — no other SDK code changes required.
handler := eid.NewWebhookHandler(&MyProviderAdapter{}, bearerToken, onComplete)
```

Tested providers: Generic BSI TR-03130 HTTP (built-in), AusweisApp2 SDK, Authada, SkIDentity, Bundesdruckerei / D-Trust.

## Roadmap

- **QEAA Issuance Flow client** — Implement the bank-side OID4VCI Pre-Authorized Code Flow: create issuance sessions, generate SD-JWT `KautionsPfandNachweis` credentials, expose the token and credential endpoints. The reference implementation lives in the XMiete server (`server/internal/issuance/`); this SDK will expose it as a reusable library.

## How to Run

```bash
go test ./...
```
