# XMiete Core

The open-source standard for digital rental deposits — BGB § 551 compliant, eIDAS 2.0 ready, pan-European.

## Overview

XMiete Core provides a unified JSON schema, OpenAPI specification, and reference server implementation to digitize the entire lifecycle of a rental deposit: from tenant identification and funding to legal pledge, QEAA credential issuance, and release. The standard covers all major EU rental markets under a single schema version.

## Features

- **Modular Schema** — Supports `CASH_EQUIVALENT`, `BANK_GUARANTEE`, `INSURANCE`, and `SURETY` (personal guarantee / Bürgschaft) deposit types.
- **Pan-European Jurisdictions** — Native support for DE, CH, AT, BE, NL, NO, GB, FR, ES; runtime-selected via `meta.jurisdiction`. Each jurisdiction carries its statutory basis and deposit-type constraints automatically.
- **Personal Surety** — `guarantor` array models third-party guarantee arrangements (Elternbürgschaft, caution solidaire, GB deed guarantors). Jurisdiction-specific compliance flags enforced at schema level (FR Garantie Visale / ALUR Art. 22-1; BGB § 551 cap; GB deed execution).
- **eID Integration** — Built-in eID verification status and EUDI Wallet credential presentation (PID, EAA, QEAA, MDL).
- **QEAA Issuance Flow** — Banks issue a legally-binding *DepositPledgeAttestation* QEAA into the tenant's EUDI Wallet via OpenID4VCI Pre-Authorized Code Flow. The SD-JWT credential replaces the paper pledge certificate and is legally recognized across the EU under eIDAS 2.0.
- **OpenID4VP Verification** — Verifiers can request selective disclosure of deposit credentials via OpenID for Verifiable Presentations; implemented across Go, Rust, and Java SDKs.
- **Selective Disclosure** — Tenants control which credential claims to share (e.g., deposit amount without revealing name or property address).
- **Credential Revocation** — Status endpoint for verifiers; credentials are automatically revoked on release or closure.
- **Settlement Flow** — Itemized deduction model with structured landlord claims and tenant dispute handling prior to deposit release.
- **Partial Release with Utility Reservation** — Landlords may release part of a deposit while reserving a utility cost buffer; the remainder follows a defined finalization path.
- **PDF Release Receipt** — On deposit release, the server generates a signed Kautionsfreigabe PDF for both parties.
- **EBICS Transport Profile** — Bank-grade batch processing via EBICS 3.0 (ISO 20022 `pain.001` / `camt.054`) for large banking institutions with existing SEPA infrastructure.
- **Installment Plans & Interest Tracking** — Schema supports phased deposit funding and accrued interest attribution.
- **Legal Compliance** — Designed for BGB § 551, eIDAS 2.0 / EUDI ARF, and national tenancy statutes across the EU.
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

## Jurisdiction Support

| Jurisdiction | Statutory basis | Deposit types |
|---|---|---|
| Germany (DE) | BGB § 551 · §§ 1204 ff. | Cash, Bank Guarantee, Surety (Bürgschaft) |
| Switzerland (CH) | OR Art. 257e | Gesperrtes Mietkautionskonto |
| Austria (AT) | MRG § 16b · ABGB § 1346 ff. | Kaution, Bankgarantie, Bürgschaft |
| Belgium (BE) | Garantie locative · e-DEPO | State escrow; CPAS/OCMW social guarantee |
| Netherlands (NL) | BW Art. 7:261 | Waarborgsom, Borgstelling |
| Norway (NO) | Husleieloven § 3-5 | Depositumskonto |
| United Kingdom (GB) | Housing Act 2004 (TDP) | Custodial, Insured, Guarantor agreements |
| France (FR) | Loi ALUR Art. 22 · Code civil Art. 2288 | Dépôt de garantie, Caution solidaire/simple |
| Spain (ES) | LAU Art. 36 | Fianza legal |

## Stakeholders

- **Fintechs & Brands:** heykaution, GetMomo, PlusForta, Smartmiete
- **Banks & Partners:** Aareal Bank, Volksbank, Instabank, Hausbank München eG, DKB, Sparkassen-Finanzgruppe, PSD Banken
- **Insurances:** Mietkautionsbürgschaften (e.g., Kautionsfrei)

## Getting Started

The core of this project is [`xmiete_schema.json`](xmiete_schema.json). The reference server lives in [`server/`](server/). SDK examples for Go, Rust, Java, and TypeScript/Node.js are in [`sdk-examples/`](sdk-examples/).

For a comprehensive overview of the standard's architecture, governance model, and adoption strategy see [`MANIFEST.md`](MANIFEST.md).

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
| High | **Persistent session store** — Replace in-memory issuance session store with Redis/DB-backed implementation for production deployments |
| High | **Conformance test suite** — Automated tests validating schema, lifecycle transitions, and credential issuance against all supported jurisdictions |
| Medium | **Credential status list (CSL)** — Batch revocation via W3C Bitstring Status List instead of per-credential polling |
| Medium | **Key management** — HSM / KMS integration for credential signing key (replace ephemeral ECDSA key) |
| Medium | **Wallet-initiated issuance** — Authorization Code Flow variant for cases where the tenant's wallet initiates issuance |
| Medium | **eIDAS Trust List integration** — Automated lookup of bank issuer trust anchors via EU Trust List |
| Low | **Batch credential issuance** — OID4VCI batch endpoint for multi-tenant scenarios |
| Low | **Python SDK** — Schema models + eID + issuance flow client |
| Low | **OpenAPI specification** — Formal OpenAPI 3.1 document for the REST transport profile |

## License

XMiete Core is dual-licensed:

- **Specification & Documentation** — [Creative Commons Attribution 4.0 International (CC BY 4.0)](LICENSE-SPECIFICATION). Covers JSON schemas (`.json`), API definitions (`.yaml`), and Markdown documentation.
- **Code & SDK Examples** — [Apache License, Version 2.0](LICENSE). Covers all source code in `sdk-examples/`, `server/`, `tests/`, and helper scripts.

Copyright © 2026 XMiete Core Contributors
