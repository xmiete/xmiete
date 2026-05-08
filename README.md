# XMiete Core

The open-source standard for digital rental deposits — BGB § 551 compliant, eIDAS 2.0 ready.

## Overview

XMiete Core provides a unified JSON schema, OpenAPI specification, and reference server implementation to digitize the entire lifecycle of a rental deposit: from tenant identification and funding to legal pledge, QEAA credential issuance, and release.

## Features

- **Modular Schema** — Supports `CASH_EQUIVALENT`, `BANK_GUARANTEE`, and `INSURANCE` deposit types.
- **eID Integration** — Built-in eID verification status and EUDI Wallet credential presentation (PID, EAA, QEAA, MDL).
- **QEAA Issuance Flow** — Banks can issue a legally-binding *DepositPledgeAttestation* QEAA directly into the tenant's EUDI Wallet via OpenID4VCI Pre-Authorized Code Flow. The SD-JWT credential replaces the physical pledge certificate and is legally recognized across the EU under eIDAS 2.0.
- **Selective Disclosure** — Tenants control which credential claims to share (e.g., deposit amount without revealing their name).
- **Credential Revocation** — Status endpoint for verifiers; credentials are automatically revoked when the deposit is released or closed.
- **Legal Compliance** — Designed for BGB § 551 and eIDAS 2.0 / EUDI ARF requirements.
- **Tax Compliance** — Steuer-ID (11-digit) validation support.

## Provider Agnosticism

XMiete Core is built to avoid middleware lock-in — a hard requirement for banks that must retain control over their own trust infrastructure.

Every identity verification module is defined behind a narrow interface (`IdentityVerifier` in Go, `EidVerifier` in Rust). The SDK ships one concrete adapter for a generic BSI TR-03130 HTTP provider, but any bank can replace it:

| Provider | Type | Notes |
|---|---|---|
| Generic HTTP | Built-in | Compatible with any BSI TR-03130 REST service |
| AusweisApp2 SDK | Custom adapter | Bundesdruckerei — local SDK, no external network hop |
| Authada | Custom adapter | SaaS, used by several German Sparkassen |
| SkIDentity | Custom adapter | OpenID Connect front-end for eID |
| Bundesdruckerei / D-Trust | Custom adapter | Government-grade HSM signing flow |

To plug in your own provider, implement a single interface — no core SDK changes required:

**Go**
```go
type MyProviderAdapter struct{ /* your config */ }

func (a *MyProviderAdapter) InitiateVerification(ctx context.Context, req eid.VerificationRequest) (*eid.VerificationSession, error) { … }
func (a *MyProviderAdapter) UpdateDepositKYCStatus(ctx context.Context, depositID string, payload eid.KYCUpdatePayload, bearerToken string) error { … }

handler := eid.NewWebhookHandler(&MyProviderAdapter{…}, bearerToken, onComplete)
```

**Rust**
```rust
struct MyProviderAdapter { /* config */ }

#[async_trait]
impl EidVerifier for MyProviderAdapter {
    async fn initiate_verification(&self, req: &VerificationRequest) -> Result<VerificationSession, EidError> { … }
    async fn update_deposit_kyc_status(&self, deposit_id: &str, payload: &KycUpdatePayload, bearer_token: &str) -> Result<(), EidError> { … }
}

let handler = WebhookHandler::new(Arc::new(MyProviderAdapter { … }), bearer_token, None);
```

Swap providers, run your test suite, ship.

## Credential Issuance Flow (QEAA)

```
Bank  →  POST /v1/deposits/{id}/issue-credential
      ←  { credential_offer_url: "openid-credential-offer://..." }

Wallet →  GET  /v1/credential-offers/{sessionId}
       →  POST /v1/token   (pre-authorized_code grant)
       →  POST /v1/credential
       ←  { credential: "<DepositPledgeAttestation SD-JWT>" }

Verifier →  GET /v1/credentials/{id}/status
         ←  { status: "active" | "revoked" }
```

See [`examples/qeaa_deposit_pledge_attestation.json`](examples/qeaa_deposit_pledge_attestation.json) for a decoded credential example.

## Stakeholders

- **Fintechs & Brands:** heykaution, GetMomo, PlusForta, Smartmiete
- **Banks & Partners:** Aareal Bank, Volksbank, Instabank, Hausbank München eG, DKB, Sparkassen-Finanzgruppe, PSD Banken
- **Insurances:** Mietkautionsbürgschaften (e.g., Kautionsfrei)

## Getting Started

The core of this project is [`xmiete_schema.json`](xmiete_schema.json). The reference server lives in [`server/`](server/). SDK examples for Go, Rust, and Java are in [`sdk-examples/`](sdk-examples/).

### Environment Variables (reference server)

| Variable | Required | Description |
|---|---|---|
| `DATABASE_URL` | yes | PostgreSQL DSN |
| `JWT_SECRET` | yes | Secret for deposit lifecycle endpoint auth |
| `ISSUER_URL` | no | OID4VCI base URL (default: `https://api.xmiete.org`) |
| `WEBHOOK_URL` | no | URL to receive deposit state-change events |

## Roadmap

| Priority | Item |
|---|---|
| High | **QEAA Issuance Flow — SDK implementations**: Port the server-side issuance flow (OpenID4VCI Pre-Authorized Code Flow, SD-JWT credential building, session management) to client SDK libraries in Go, Rust, Java, Python, and TypeScript. |
| High | **Python SDK** — Schema models + eID + issuance flow client |
| High | **TypeScript/Node.js SDK** — Schema models + eID + issuance flow client |
| Medium | **Persistent session store** — Replace in-memory issuance session store with Redis/DB-backed implementation for production deployments |
| Medium | **Credential status list (CSL)** — Batch revocation via W3C Bitstring Status List instead of per-credential polling |
| Medium | **Key management** — HSM / KMS integration for credential signing key (replace ephemeral ECDSA key) |
| Medium | **Wallet-initiated issuance** — Authorization Code Flow variant for cases where the tenant's wallet initiates the issuance |
| Low | **Batch credential issuance** — OID4VCI batch endpoint for multi-tenant scenarios |
| Low | **eIDAS Trust List integration** — Automated lookup of bank issuer trust anchors via EU Trust List |

## License

XMiete Core is dual-licensed:

- **Specification & Documentation** — [Creative Commons Attribution 4.0 International (CC BY 4.0)](LICENSE-SPECIFICATION). Covers JSON schemas (`.json`), API definitions (`.yaml`), and Markdown documentation.
- **Code & SDK Examples** — [Apache License, Version 2.0](LICENSE). Covers all source code in `sdk-examples/`, `server/`, `tests/`, and helper scripts.

Copyright © 2026 XMiete Core Contributors
