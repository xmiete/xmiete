# XMiete Core

The open-source standard for digital rental deposits — BGB § 551 compliant, eIDAS 2.0 ready, pan-European.

## Overview

XMiete Core provides a unified JSON schema, OpenAPI specification, and reference server implementation to digitize the entire lifecycle of a rental deposit: from tenant identification and funding to legal pledge, QEAA credential issuance, and release. The standard covers all major EU rental markets under a single schema version.

## Features

- **Modular Schema** — Supports `CASH_EQUIVALENT`, `BANK_GUARANTEE`, `INSURANCE`, and `SURETY` (personal guarantee / Bürgschaft) deposit types.
- **Pan-European Jurisdictions** — Native support for DE, CH, AT, BE, NL, NO, GB, FR, ES. Each jurisdiction enforces its statutory basis, deposit-type constraints, and currency automatically.
- **Personal Surety** — `guarantor` array models third-party guarantee arrangements (Elternbürgschaft, caution solidaire, GB deed guarantors). Jurisdiction-specific compliance flags enforced at schema level.
- **eID Integration** — Built-in eID verification status and EUDI Wallet credential presentation (PID, EAA, QEAA, MDL).
- **QEAA Issuance Flow** — Banks issue a legally-binding *DepositPledgeAttestation* QEAA into the tenant's EUDI Wallet via OpenID4VCI Pre-Authorized Code Flow. The SD-JWT credential replaces the paper pledge certificate and is legally recognized across the EU under eIDAS 2.0.
- **W3C Bitstring Status List** — Credentials carry a `credentialStatus` entry pointing to a signed `BitstringStatusListCredential`. Verifiers fetch the list once and check bits locally — no per-credential polling required.
- **OpenID4VP Verification** — Verifiers can request selective disclosure of deposit credentials via OpenID for Verifiable Presentations; implemented across Go, Rust, and Java SDKs.
- **Selective Disclosure** — Tenants control which credential claims to share (e.g., deposit amount without revealing name or property address).
- **Conformance Validator** — `POST /validate` accepts any deposit document and returns a structured conformance report: schema checks plus jurisdiction-specific statutory rules (currency, cap, deposit type, statutory basis, interest obligation, trusteeship plausibility) for all 9 supported countries.
- **Settlement Flow** — Itemized deduction model with structured landlord claims and tenant dispute handling prior to deposit release.
- **Partial Release with Utility Reservation** — Landlords may release part of a deposit while reserving a utility cost buffer; the remainder follows a defined finalization path.
- **PDF Release Receipt** — On deposit release, the server generates a signed Kautionsfreigabe PDF for both parties.
- **EBICS Transport Profile** — Bank-grade batch processing via EBICS 3.0 (ISO 20022 `pain.001` / `camt.054`) for large banking institutions with existing SEPA infrastructure.
- **Installment Plans & Interest Tracking** — Schema supports phased deposit funding and accrued interest attribution.
- **Legal Compliance** — Designed for BGB § 551, eIDAS 2.0 / EUDI ARF, and national tenancy statutes across the EU.

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

Verifier →  GET /v1/status-list/revocation
         ←  { credential: "<BitstringStatusListCredential JWT>" }
```

The issued SD-JWT embeds a `credentialStatus` claim:
```json
{
  "credentialStatus": {
    "type": "BitstringStatusListEntry",
    "statusPurpose": "revocation",
    "statusListIndex": "42",
    "statusListCredential": "https://api.xmiete.org/v1/status-list/revocation"
  }
}
```

See [`examples/qeaa_deposit_pledge_attestation.json`](examples/qeaa_deposit_pledge_attestation.json) for a full decoded credential example.

## Conformance Validator

`POST /validate` is a public, stateless endpoint that checks any deposit document against XMiete schema rules and the statutory constraints of its jurisdiction. It requires no authentication and makes no database calls.

```bash
curl -X POST https://api.xmiete.org/validate \
  -H "Content-Type: application/json" \
  -d '{
    "deposit": {
      "meta": {"version": "2.2.0"},
      "tenant": {"first_name": "Max", "last_name": "Müller", "email": "m@example.com"},
      "landlord": {"name": "Haus GmbH"},
      "property": {"address": {"street": "Teststr. 1", "zip": "10115", "city": "Berlin", "country": "DE"}},
      "deposit": {"amount": 4500, "currency": "EUR", "type": "CASH_EQUIVALENT"}
    },
    "monthly_rent": 1000
  }'
```

```json
{
  "valid": false,
  "jurisdiction": "DE",
  "schema_version": "2.2.0",
  "findings": [
    {
      "code": "CAP_EXCEEDED",
      "severity": "ERROR",
      "field": "deposit.amount",
      "message": "DE statutory cap is 3 months rent — deposit 4500.00 exceeds max 3000.00 EUR",
      "rule": "BGB § 551"
    }
  ],
  "checked_at": "2026-06-26T09:00:00Z"
}
```

The response is always `200 OK`. `valid: false` means at least one `ERROR`-severity finding is present. `WARNING` findings indicate likely problems; `INFO` findings are informational.

### Finding codes

| Code | Severity | Description |
|---|---|---|
| `MISSING_TENANT_FIELDS` | ERROR | `tenant.first_name`, `last_name`, or `email` absent |
| `MISSING_LANDLORD_NAME` | ERROR | `landlord.name` absent |
| `INVALID_AMOUNT` | ERROR | `deposit.amount` ≤ 0 |
| `MISSING_CURRENCY` | ERROR | `deposit.currency` absent |
| `MISSING_TYPE` | ERROR | `deposit.type` absent or unknown |
| `UNKNOWN_TYPE` | ERROR | Unrecognized deposit type |
| `WRONG_CURRENCY` | ERROR | Currency does not match jurisdiction requirement |
| `DISALLOWED_TYPE` | ERROR | Deposit type not permitted in this jurisdiction |
| `CAP_EXCEEDED` | ERROR | Deposit exceeds statutory months-of-rent cap |
| `UNEXPECTED_STATUTORY_BASIS` | WARNING | `pledge.statutory_basis` does not match jurisdiction |
| `INTEREST_REQUIRED` | WARNING | Jurisdiction mandates interest accrual (CH, NO) |
| `MISSING_TRUSTEESHIP` | WARNING | `CASH_EQUIVALENT` deposit has no trusteeship declaration |
| `INSOLVENCY_PROTECTION_UNCONFIRMED` | WARNING | Trusteeship present but not marked insolvency-protected |
| `MISSING_VERSION` | WARNING | `meta.version` not set |
| `MISSING_COUNTRY` | WARNING | `property.address.country` absent — jurisdiction checks skipped |
| `UNKNOWN_JURISDICTION` | INFO | No statutory rules defined for this country |

## Jurisdiction Support

| Jurisdiction | Statutory basis | Cap | Deposit types |
|---|---|---|---|
| Germany (DE) | BGB § 551 · §§ 1204 ff. | 3 months | Cash, Bank Guarantee, Surety |
| Switzerland (CH) | OR Art. 257e | 3 months | Cash (blocked savings account) |
| Austria (AT) | MRG § 16b · ABGB § 1346 ff. | 3 months | Cash, Bank Guarantee |
| Belgium (BE) | Woninghuurwet · Code civil belge | 2 months | Cash, Bank Guarantee |
| Netherlands (NL) | BW Art. 7:261 | 2 months | Cash only |
| Norway (NO) | Husleieloven § 3-5 | 6 months | Cash (Depositumskonto) |
| United Kingdom (GB) | Housing Act 2004 · Tenant Fees Act 2019 | 5 weeks | Cash, Bank Guarantee |
| France (FR) | Loi ALUR · Loi 89-462 | 1 month | Cash, Surety |
| Spain (ES) | LAU Art. 36 | 1 month | Cash, Bank Guarantee |

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
| `PORT` | no | HTTP listen port (default: `8080`) |
| `WEBHOOK_URL` | no | URL to receive deposit state-change events |
| `SMTP_HOST` | no | SMTP host for receipt emails (disabled if absent) |
| `SMTP_PORT` | no | SMTP port (default: `587`) |
| `SMTP_USERNAME` | no | SMTP credentials |
| `SMTP_PASSWORD` | no | SMTP credentials |
| `SMTP_FROM` | no | Sender address (default: `noreply@xmiete.org`) |

## Roadmap

| Priority | Item |
|---|---|
| High | **Key management** — HSM / KMS integration for credential signing key (replace ephemeral ECDSA key) |
| High | **Example infrastructure** — Docker Compose + deployment templates for banks and property management software to use as a starting point |
| Medium | **Wallet-initiated issuance** — Authorization Code Flow variant for cases where the tenant's wallet initiates issuance |
| Medium | **eIDAS Trust List integration** — Automated lookup of bank issuer trust anchors via EU Trust List |
| Medium | **Conformance seal** — Paid certification tier: partners submit to `POST /validate` and receive a signed "XMiete-konform" attestation for their implementation |
| Low | **Batch credential issuance** — OID4VCI batch endpoint for multi-tenant scenarios |
| Low | **Python SDK** — Schema models + eID + issuance flow client |
| Low | **OpenAPI specification** — Formal OpenAPI 3.1 document for the REST transport profile |

## License

XMiete Core is dual-licensed:

- **Specification & Documentation** — [Creative Commons Attribution 4.0 International (CC BY 4.0)](LICENSE-SPECIFICATION). Covers JSON schemas (`.json`), API definitions (`.yaml`), and Markdown documentation.
- **Code & SDK Examples** — [Apache License, Version 2.0](LICENSE). Covers all source code in `sdk-examples/`, `server/`, `tests/`, and helper scripts.

Copyright © 2026 XMiete Core Contributors
