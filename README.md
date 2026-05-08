# XMiete Core

The open-source standard for digital rental deposits — BGB § 551 compliant, eIDAS 2.0 ready.

## Overview

XMiete Core provides a unified JSON schema, OpenAPI specification, and reference server implementation to digitize the entire lifecycle of a rental deposit: from tenant identification and funding to legal pledge, QEAA credential issuance, and release.

## Features

- **Modular Schema** — Supports `CASH_EQUIVALENT`, `BANK_GUARANTEE`, and `INSURANCE` deposit types.
- **eID Integration** — Built-in eID verification status and EUDI Wallet credential presentation (PID, EAA, QEAA, MDL).
- **QEAA Issuance Flow** — Banks can issue a legally-binding *KautionsPfandNachweis* QEAA directly into the tenant's EUDI Wallet via OpenID4VCI Pre-Authorized Code Flow. The SD-JWT credential replaces the physical pledge certificate and is legally recognized across the EU under eIDAS 2.0.
- **Selective Disclosure** — Tenants control which credential claims to share (e.g., deposit amount without revealing their name).
- **Credential Revocation** — Status endpoint for verifiers; credentials are automatically revoked when the deposit is released or closed.
- **Legal Compliance** — Designed for BGB § 551 and eIDAS 2.0 / EUDI ARF requirements.
- **Tax Compliance** — Steuer-ID (11-digit) validation support.

## Credential Issuance Flow (QEAA)

```
Bank  →  POST /v1/deposits/{id}/issue-credential
      ←  { credential_offer_url: "openid-credential-offer://..." }

Wallet →  GET  /v1/credential-offers/{sessionId}
       →  POST /v1/token   (pre-authorized_code grant)
       →  POST /v1/credential
       ←  { credential: "<KautionsPfandNachweis SD-JWT>" }

Verifier →  GET /v1/credentials/{id}/status
         ←  { status: "active" | "revoked" }
```

See [`examples/qeaa_kautions_nachweis.json`](examples/qeaa_kautions_nachweis.json) for a decoded credential example.

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
