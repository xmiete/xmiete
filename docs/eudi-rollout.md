# EUDI Wallet Rollout — Impact on XMiete Receipt Strategy

Research date: 2026-05-08

## Key Dates

| Date | Milestone |
|---|---|
| Dec 2024 | eIDAS 2.0 Implementing Acts enter into force — 24-month clock starts |
| Q1 2026 | Germany SPRIND Sandbox live; QEAA testing now available |
| **Dec 2026** | EU legal deadline: all member states must offer a certified wallet to citizens |
| **Jan 2027** | Germany state wallet (SPRIND + Bundesdruckerei) planned citizen launch — PID focus initially |
| Dec 2027 | Mandatory private-sector acceptance deadline — banks and regulated entities only |
| ~2028 | Private wallet providers (Lissi etc.) permitted to certify and enter market |
| ~2028+ | QEAA flows like DepositPledgeAttestation realistically production-ready for broad use |

**Note:** The EU Commission itself has expressed doubt that all member states will meet the Dec 2026 deadline. Expect a staggered rollout. Germany is among the best-prepared states but its state wallet still slips to Jan 2027.

## Germany-Specific Status (as of May 2026)

- **SPRIND Sandbox**: live, accepting relying parties for testing; both PID and QEAA testing now available (QEAA added Q1 2026).
- **State wallet app**: planned for 2 January 2027 by SPRIND + Bundesdruckerei. Core launch features: digital identity (PID) and qualified electronic signatures (QES). QEAA delivery to third-party relying parties is a later roadmap item.
- **Private wallets** (Lissi, others): blocked from citizen-facing production until ~12 months after the state wallet launches (~Jan 2028).
- **Lissi EUDI Wallet Connector**: can already issue QEAAs in the sandbox today and is the closest near-term bridge for early adopters, but not yet a production citizen wallet.

## Mandatory Acceptance Scope

The Dec 2027 mandate covers only a defined subset of private-sector parties:

- Banks and regulated financial institutions (SCA-required).
- Very Large Online Platforms and designated gatekeepers.
- Telecom providers and other Commission-designated categories.

**Landlords and proptech platforms are NOT in any designated category.** There is no mandatory EUDI acceptance deadline for XMiete — acceptance remains voluntary indefinitely under current law.

## Implications for XMiete

The existing QEAA issuance flow (OID4VCI Pre-Authorized Code) is the right long-term architecture. However, no tenant going live in 2026 or early 2027 will realistically have a wallet capable of receiving a DepositPledgeAttestation via the production path.

**Recommended strategy:**

1. **PDF receipt as default delivery** — generated automatically when a deposit reaches `PLEDGED` with `is_confirmed_by_bank = true`. Sent to the tenant by email or available via `GET /v1/deposits/{id}/receipt`.
2. **QEAA as opt-in enhanced receipt** — the existing issuance flow remains in place; tenants with a compatible wallet can scan the credential offer QR code to receive the QEAA in addition to the PDF.
3. **Both in parallel, not either/or** — the PDF receipt and the QEAA cover the same information. Having both does not create a second source of truth: the QEAA is the tamper-evident binding artifact; the PDF is the human-readable fallback.

## Sources

- [EU Commission EUDI Regulation Overview](https://digital-strategy.ec.europa.eu/en/policies/eudi-regulation)
- [EU Digital Identity Wallet — Wikipedia](https://en.wikipedia.org/wiki/EU_Digital_Identity_Wallet)
- [Will the EUDI Wallet be ready in 2026? — Biometric Update](https://www.biometricupdate.com/202512/will-the-eudi-wallet-be-ready-in-2026-experts-say-probably-not)
- [Germany launches EUDI Wallet sandbox — Biometric Update](https://www.biometricupdate.com/202601/germany-launches-eudi-wallet-sandbox-to-test-key-functions-apply-specific-use-cases)
- [Germany lays out plans for national EUDI Wallet — Biometric Update](https://www.biometricupdate.com/202410/germany-lays-out-plans-for-national-eudi-wallet)
- [Official German EUDI Wallet portal — eudi-wallet.gov.de](https://eudi-wallet.gov.de/en)
- [Mandatory EUDI Wallet Acceptance: Who Must Accept — Dock.io](https://www.dock.io/post/mandatory-eudi-wallet-acceptance-heres-who-must-accept-and-whos-exempt)
- [Lissi EUDI Wallet Connector and QEAA](https://www.lissi.id/qeaa)
- [EUDI Wallets — Only One Year to Launch — Signicat](https://www.signicat.com/blog/eudi-wallets-only-one-year-to-launch)
