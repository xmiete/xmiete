# XMiete Core — Release Announcements

This file contains release announcements for XMiete Core. Each release section follows the template defined below.

---

## Announcement Template

Use this structure for every release. Omit sections that have no content; never leave a section empty.

```
## XMiete Core vX.Y.Z — <one-line release title>

**Released:** YYYY-MM-DD
**Schema version:** X.Y.Z
**Manifest version:** X.Y (if updated)

### What's New

Short narrative paragraph (2–4 sentences) explaining the theme of this release and its significance.

#### Breaking Changes
- List each breaking change. If none: omit this subsection entirely.

#### New Features
- <Feature name> — one-sentence description and the schema field or API endpoint it introduces.

#### Bug Fixes & Improvements
- <Fix or improvement> — one-sentence description.

### Migration Notes

Describe any schema field renames, removed fields, or state-machine changes implementers must handle.
If this is a minor/patch release with no migration work: omit this section.

### SDK Updates

| Language | Status | Notes |
|---|---|---|
| Go | Updated / No change | |
| Rust | Updated / No change | |
| Java | Updated / No change | |
| TypeScript | Updated / No change | |

### Stakeholder Impact

Which actor categories are affected and what action (if any) they need to take.

### Get Involved

Standard closing — link to repo and working group.
```

---

## XMiete Core v2.2.0 — Personal Surety and International Compliance

**Released:** 2026-05-18
**Schema version:** 2.2.0
**Manifest version:** 0.3

### What's New

XMiete Core v2.2.0 completes the deposit-type matrix by adding full support for personal surety arrangements — Bürgschaft, caution solidaire, and GB deed guarantors — alongside the existing cash, bank guarantee, and insurance types. This release also resolves the remaining international compliance gaps identified in the v2.1.0 Belgium and UK pilot, and upgrades the manifest to reflect the updated governance model.

#### New Features
- **`SURETY` deposit type** — new `deposit.type` enum value; schema enforces presence of at least one `guarantor` entry when type is `SURETY`.
- **`guarantor` array** — captures legal form of the guarantee (`SELBSTSCHULDNERISCH`, `AUSFALLBUERGSCHAFT`, `CAUTION_SOLIDAIRE`, `CAUTION_SIMPLE`, `STANDARD`), guarantor relationship, scope and cap of liability, and jurisdiction-specific compliance flags.
- **FR Garantie Visale enforcement** — `fr_visale_held` and `fr_visale_student_exception` flags enforce ALUR Art. 22-1 prohibition at schema validation time.
- **GB deed execution flag** — `executed_as_deed` field for guarantor agreements requiring deed execution for fixed-term enforceability.
- **BGB § 551 cap field** — `cap_amount` records the three-month cold-rent cap applied by German courts to Bürgschaften substituting a Mietkaution.
- **`validity_follows_tenancy` flag** — binds GB guarantors to lease renewals automatically.

#### Bug Fixes & Improvements
- Resolved schema validation edge cases for BE `STATE_BODY` guarantors introduced in v2.1.0.
- Corrected `statutory_basis` enum for Norwegian (`NO`) deposits to `Husleieloven § 3-5`.

### Migration Notes

v2.2.0 is backwards-compatible with v2.1.0. No existing `deposit.type` values were renamed or removed. Implementers who handle all `deposit.type` values exhaustively (switch/match statements) should add a `SURETY` case; unrecognized types should be treated as pass-through if strict validation is not required. The `guarantor` array is absent on all existing deposit objects and will not appear unless `deposit.type` is `SURETY`.

### SDK Updates

| Language | Status | Notes |
|---|---|---|
| Go | Updated | `guarantor` types added; eID and OpenID4VP modules unchanged |
| Rust | Updated | `Guarantor` struct and builder added |
| Java | Updated | `GuarantorBuilder` and validation helpers |
| TypeScript | Updated | Full type definitions for `Guarantor` and all flag fields |

### Stakeholder Impact

**Banks** implementing deposit pledge flows must handle the new `SURETY` type in their lifecycle state machines. The `guarantor` array is required for pledge and issuance; banks should surface guarantor details in their KYC review step.

**Property management software** consuming XMiete objects should render guarantor details alongside the deposit summary; the `cap_amount` and `executed_as_deed` fields are relevant for legal document generation.

**Fintech platforms** operating in France must check `fr_visale_held` before presenting the personal surety option to landlords; presenting it when prohibited is a schema validation error.

### Get Involved

XMiete Core is open for contributions. Schema, documentation, and SDK examples are at [github.com/xmiete/xmiete-core](https://github.com/xmiete/xmiete-core). Comments and proposals are welcome via the AG XMiete working group at [xmiete.org](https://xmiete.org).

---

## XMiete Core v2.1.0 — International Compliance for Belgium and United Kingdom

**Released:** 2026-05 (exact date not recorded)
**Schema version:** 2.1.0

### What's New

v2.1.0 extends the international jurisdiction coverage introduced in v2.0.0 with full compliance mappings for Belgium (Garantie locative, e-DEPO, CPAS/OCMW social guarantee via `STATE_BODY`) and the United Kingdom (Housing Act 2004 TDP — custodial and insured schemes). These were partially supported in v2.0.0 but lacked jurisdiction-specific field constraints.

#### New Features
- **BE `STATE_BODY` guarantor type** — supports CPAS/OCMW social guarantee in Belgium.
- **GB TDP scheme fields** — `tdp_scheme` and `tdp_registration_reference` for custodial and insured scheme compliance.
- **`meta.jurisdiction` constraint modules** — per-jurisdiction JSON Schema extension modules now enforced for BE and GB.

### Migration Notes

Deposits created under v2.0.0 with `meta.jurisdiction: "BE"` or `meta.jurisdiction: "GB"` may fail validation against v2.1.0 if they relied on relaxed BE/GB constraints. Verify BE and GB deposits against the v2.1.0 schema before upgrading in production.

### SDK Updates

| Language | Status | Notes |
|---|---|---|
| Go | Updated | BE/GB jurisdiction helpers |
| Rust | Updated | |
| Java | Updated | |
| TypeScript | No change | |

---

## XMiete Core v2.0.0 — Pan-European Jurisdiction Support

**Released:** 2026-04 (exact date not recorded)
**Schema version:** 2.0.0

### What's New

v2.0.0 is a major release that transforms XMiete from a German-market standard into a pan-European protocol. The `meta.jurisdiction` field now selects from nine national legal frameworks; field constraints, deposit types, and statutory basis values adapt accordingly at runtime. No harmonisation of substantive tenancy law is required or implied — XMiete carries the law of the deposit, not a unified European tenancy law.

#### Breaking Changes
- `meta.jurisdiction` is now required (was optional in v1.x). Existing deposits without a jurisdiction field must be backfilled with `"DE"` before validation against v2.0.0.
- `is_treuhand` field removed; replaced by `trusteeship.is_treuhand` and `trusteeship.type` within the new `trusteeship` object.
- `statutory_basis` is now an enum constrained by `meta.jurisdiction`; free-text values from v1.x are not valid.

#### New Features
- Jurisdiction-aware field semantics for DE, CH, AT, BE, NL, NO, GB, FR, ES.
- `trusteeship` object for BGB § 551 Abs. 3 Treuhandkonto arrangements.
- EBICS 3.0 transport profile with ISO 20022 `pain.001` / `camt.054` BTF descriptors.
- Installment plan support and accrued interest tracking.
- Settlement flow with itemized deductions and tenant dispute handling.
- Partial release with utility cost reservation.
- PDF release receipt (Kautionsfreigabe) generated on deposit release.

### Migration Notes

Breaking changes are listed above. The v2.0.0 migration guide is in [`developer-docs/migration-v2.md`](developer-docs/migration-v2.md) (if present). Deposits with `schema_version: "1.x"` continue to be accepted by the reference server under a compatibility shim until the v3.0 deprecation window.

### SDK Updates

| Language | Status | Notes |
|---|---|---|
| Go | Updated | Full v2.0.0 schema support; auth module added |
| Rust | Updated | Full v2.0.0 schema support; auth module added |
| Java | Updated | OpenID4VP verification added |
| TypeScript | New | Initial release — eID, OpenID4VP, auth modules |

---

## XMiete Core v1.0.0 — Initial Release

**Released:** 2025 (exact date not recorded)
**Schema version:** 1.0.0

### What's New

Initial public release of XMiete Core. Establishes the open-source standard for digital rental deposits under BGB § 551 with support for cash deposits, bank guarantees, and insurance products. Introduces the seven-state deposit lifecycle, eID integration, QEAA issuance via OpenID4VCI, and SDK examples in Go, Rust, and Java.

#### New Features
- JSON Schema (`xmiete_schema.json`) for `CASH_EQUIVALENT`, `BANK_GUARANTEE`, and `INSURANCE` deposit types.
- Seven-state lifecycle state machine: `REQUESTED → IDENTIFIED → FUNDED → PLEDGED → RELEASED → CLAIMED → CLOSED`.
- JWS-signed audit history for non-repudiable state transitions.
- eID verification status (`tenant.eid_status`) with BSI TR-03130 provider interface.
- EUDI Wallet metadata (`wallet_metadata`) for PID, EAA, and QEAA credentials.
- DepositPledgeAttestation QEAA credential definition (SD-JWT, `vc+sd-jwt`).
- OpenID4VCI Pre-Authorized Code Flow for credential issuance.
- Credential revocation status endpoint.
- mTLS, OAuth2 scope, and JWS signing for financial-grade security.
- Steuer-ID (11-digit) validation.
- SDK examples: Go, Rust, Java.
- Reference server implementation.

### Stakeholder Impact

Initial release — all implementers start fresh. No migration required.

### Get Involved

XMiete Core is open for contributions at [github.com/xmiete/xmiete-core](https://github.com/xmiete/xmiete-core).
