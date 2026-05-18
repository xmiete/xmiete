# XMiete Technical Manifest
**Version 0.3 — May 2026**
**AG XMiete · xmiete.org**

---

## 1. Preamble: Why a Standard, Not a Product

Across the European Union, roughly 1.5 million new rental deposits are created every month — an estimated €180 billion in tenant capital locked in workflows that are still predominantly paper-based. The failure is not technological. The technology to digitise this process has existed for years. The failure is structural: no single actor in the market has the mandate, the neutrality, or the incentive to define the shared protocol that all others would need to adopt.

This is exactly the class of problem that standards solve.

XMiete is not a product. It is not a platform. It is not a marketplace. It is an open, vendor-neutral, governance-backed specification — a digital infrastructure layer for rental deposit management, designed from the ground up as a pan-European standard and grounded in eIDAS 2.0 as the shared legal framework.

The initiative draws directly from the governance tradition of comparable German digital infrastructure standards:

| Standard | Domain | Governance | Legal anchor |
|---|---|---|---|
| **DENIC** | Domain names (.de) | Registered cooperative, multi-stakeholder | Telekommunikationsgesetz, ICANN delegation |
| **xRechnung** | Electronic invoices (B2G) | KoSIT working group, federal mandate | EU Directive 2014/55/EU, E-Rechnungsverordnung |
| **xBau** | Building permit data exchange | KoSIT / IT-Planungsrat working group | Onlinezugangsgesetz (OZG), Baurecht |
| **XMiete** | Rental deposit lifecycle | AG XMiete, open working group | eIDAS 2.0 Regulation (EU) 2024/1183 · national tenancy statutes via `meta.jurisdiction` |

Each of these exists because a market or public administration process had a coordination problem that no commercial actor could or should solve unilaterally. XMiete is initiated from the same conviction.

---

## 2. The Coordination Problem

The rental deposit process involves at least four distinct actors — tenant, landlord (or property manager), bank, and in the case of deposit insurance or guarantee products, a fourth financial institution — none of whom share a common data format, identity layer, or communication protocol. The consequences are predictable:

- **Media breaks at every interface.** A tenant who opens a deposit account at a bank cannot transmit the pledge confirmation to the landlord in a machine-readable format. The landlord's property management software cannot receive it programmatically. The bank has no standardised outbound channel.
- **KYC costs are repeated at every transition.** Without a shared identity anchor, each new deposit requires a full identity verification cycle. This is the single largest cost driver in the digital deposit products already on the market.
- **No interoperability between deposit types.** Cash deposits, bank guarantees, and deposit insurance products operate on entirely separate rails. There is no lifecycle-level abstraction that spans them.
- **Proprietary lock-in masquerading as innovation.** Existing fintech platforms in this space have each defined their own internal formats. They have no incentive to interoperate. The market fragments into islands.

This is a textbook commons problem. The optimal outcome — a shared standard that reduces costs for all participants — cannot emerge from individual actors' self-interest alone. It requires a neutral coordination layer: a standard.

---

## 3. Governance Principles

XMiete is governed by the **AG XMiete** (Arbeitsgemeinschaft XMiete), an open working group in the tradition of KoSIT and the IT-Planungsrat working groups that produce xRechnung, xBau, and the XÖV standard family.

The governance model rests on four principles:

**3.1 Vendor neutrality.** No single commercial entity controls the specification. The schema, the credential definitions, and the lifecycle state machine are governed by the working group and published under open licence (CC BY 4.0). Any bank, platform, or software vendor may implement XMiete without licensing fees and without seeking permission from any commercial counterparty.

**3.2 Separation of specification and implementation.** XMiete defines *what* is exchanged — the schema, the lifecycle, the credential format — not *how* the market is organised around it. Banks remain independent. Platforms remain competitive. XMiete is the protocol layer below them, not the commercial layer above them. This is precisely the DENIC model: DENIC operates the registry; the domain market is served by hundreds of registrars.

**3.3 Normative versioning.** The specification is semantically versioned. Minor versions are backwards-compatible. Major versions go through a working group review process with a defined migration window. No version is deprecated without a successor and a migration path. This stability guarantee is what makes the standard safe to build on — the same guarantee that xRechnung's normative publication cycle provides to ERP vendors.

**3.4 Public interest mandate.** The working group operates without profit motive. Its mandate is the quality and stability of the standard, not the commercial success of any participant. Membership is open to banks, housing associations, property management software vendors, legal experts, and public administration.

---

## 4. Technical Architecture

XMiete is a layered architecture. Each layer has a defined scope and may be replaced or extended without affecting the others.

### 4.1 Schema Layer — the Core Protocol

The XMiete Core Schema (`xmiete_schema.json`, JSON Schema Draft 07) defines the canonical data structure for a rental deposit object. It is the normative centre of the standard — the equivalent of xRechnung's UBL/CII XML binding or xBau's XSD.

Key design properties:
- **Jurisdiction-aware field semantics.** The `meta.jurisdiction` field selects the applicable national legal framework; field constraints adapt accordingly — BGB § 551 (DE), Art. 257e CO (CH), Housing Act 2004 (UK), Garantie locative (BE), Husleieloven § 3-5 (NO), and so on. XMiete carries the law of the deposit, not just its data.
- **Provider-agnostic.** The `provider` object supports all four deposit types (cash equivalent, bank guarantee, insurance, personal surety) and all four provider archetypes (direct bank, fintech platform, insurance broker, bank system partner) under a single schema.
- **Lifecycle state machine.** The `deposit.lifecycle_state` field defines a normative seven-state machine (`REQUESTED → IDENTIFIED → FUNDED → PLEDGED → RELEASED → CLAIMED → CLOSED`) with non-repudiable JWS-signed state transitions in the audit history.
- **Modular identity integration.** The `tenant.eid_status` field and `wallet_metadata` object support any national eID scheme compliant with eIDAS LoA "High" today and EUDI Wallet QEAA credentials (PID, EAA) as they become available under eIDAS 2.0.
- **Personal surety support.** The `guarantor` array models third-party guarantee arrangements alongside or in place of a cash deposit. It captures the legal form of the guarantee (DE `SELBSTSCHULDNERISCH` / `AUSFALLBUERGSCHAFT`; FR `CAUTION_SOLIDAIRE` / `CAUTION_SIMPLE`; GB/NL/IE `STANDARD`), the guarantor's relationship to the tenant (parent, employer, third party, state body), the scope and cap of liability, and jurisdiction-specific compliance flags (FR Garantie Visale / ALUR Art. 22-1; GB deed execution). A schema-level conditional enforces that any deposit of type `SURETY` carries at least one guarantor entry.

### 4.2 Identity Layer — eID and EUDI Wallet

XMiete does not define an identity protocol. It profiles existing standards.

For domestic use: the German eID (Online-Ausweisfunktion, BSI TR-03130) provides the identity anchor. The `tenant.eid_status` field records the outcome of eID verification. Provider selection (AusweisApp2, Authada, SkIDentity, Bundesdruckerei) is outside the standard's scope.

For European scale: the EUDI Wallet (eIDAS 2.0, Regulation (EU) 2024/1183) provides the interoperability layer. XMiete profiles the `wallet_metadata` object against the EUDI Architecture Reference Framework (ARF), supporting `PID`, `EAA`, and `QEAA` credential types in `vc+sd-jwt` and `mso_mdoc` formats.

### 4.3 Credential Layer — DepositPledgeAttestation

The **DepositPledgeAttestation** is a QEAA (Qualified Electronic Attestation of Attributes) credential issued by a bank into the tenant's EUDI Wallet upon deposit pledge. It is the machine-readable, legally binding proof that replaces the paper pledge confirmation letter.

The credential is defined within the XMiete schema (`definitions/deposit_pledge_attestation`) and follows the OpenID for Verifiable Credential Issuance (OID4VCI) specification, Pre-Authorized Code Flow. Key properties:

- Format: `vc+sd-jwt`
- Assurance level: `high` (eIDAS Article 8, LoA High)
- Selective disclosure: amount and property address may be withheld from verifiers who do not require them (privacy by design)
- Non-repudiation: credential maps to a JWS-signed lifecycle event in the deposit history
- Status: revocable via status URI (deposit released or closed triggers revocation)

This credential is to the rental deposit what the `rechnungsnummer` and `leitweg-id` are to xRechnung: the normative identifier that makes the transaction machine-processable end-to-end.

### 4.4 Transport Layer — REST and EBICS

XMiete is transport-agnostic at the schema level. Two transport profiles are currently specified:

**REST profile** (default): Standard HTTPS/JSON. Appropriate for fintech platforms, property management software, and any modern API integration. OpenAPI specification TBD in a future working group deliverable.

**EBICS profile**: Bank-grade batch/bulk processing via EBICS 3.0. The `transport.ebics_metadata` object carries all parameters required for an EBICS connection (host ID, partner ID, user ID, bank URL, signature class). BTF descriptors for ISO 20022 messages are defined: `MCT/pain.001` for credit transfer initiation, `STM/camt.054` for credit notification. This profile is critical for large banking institutions with existing EBICS infrastructure — the same infrastructure used for SEPA credit transfers and direct debits.

---

## 5. Legal Grounding

XMiete does not create new law. It operationalises existing law — at two levels simultaneously.

**eIDAS Regulation (EU) No 910/2014 as amended by (EU) 2024/1183** is the primary pan-European anchor. It provides the legal basis for EUDI Wallet credentials at LoA "High" across all 27 EU member states. A DepositPledgeAttestation issued by a regulated bank as a QEAA has the same legal standing across the EU as a bank-issued paper document — this is the legal mechanism that makes the pan-European standard possible without harmonising national tenancy law.

**National tenancy statutes** form the second layer, selected at runtime by the `meta.jurisdiction` field. The schema ships with normative mappings for the largest EU rental markets:

| Jurisdiction | Statutory basis | Deposit type |
|---|---|---|
| Germany (DE) | BGB § 551 · §§ 1204 ff. (Pfandrecht) | Treuhandkonto / Verpfändung |
| Germany (DE) — personal surety | BGB § 765 (Bürgschaft) | Elternbürgschaft / selbstschuldnerische Bürgschaft |
| Switzerland (CH) | OR Art. 257e | Gesperrtes Mietkautionskonto |
| Austria (AT) | MRG § 16b · ABGB § 1346 ff. | Kaution / Bankgarantie / Bürgschaft |
| Belgium (BE) | Garantie locative · e-DEPO | State-held escrow; CPAS/OCMW social guarantee via `STATE_BODY` |
| Netherlands (NL) | BW Art. 7:261 | Waarborgsom / borgstelling |
| Norway (NO) | Husleieloven § 3-5 | Depositumskonto |
| United Kingdom (GB) | Housing Act 2004 (TDP) | Custodial / insured; guarantor agreements (contractual) |
| France (FR) | Loi ALUR Art. 22 · Art. 2288 Code civil | Dépôt de garantie / caution solidaire / caution simple |
| Spain (ES) | LAU Art. 36 | Fianza legal |

Implementers outside this list may supply a custom `statutory_basis` string; conformance is validated against the jurisdiction-specific JSON Schema extension module.

**Personal surety cross-jurisdiction note.** Where a landlord accepts a personal guarantee in lieu of or alongside a cash deposit, the `deposit.type` is set to `SURETY` and the `guarantor` array carries the full guarantee terms. The schema enforces the most critical jurisdiction-specific constraint automatically: in France, if the landlord holds a Garantie Visale, requiring a personal guarantor is prohibited under ALUR Art. 22-1 unless the tenant is a student — both conditions are captured in `fr_visale_held` and `fr_visale_student_exception`. In Germany, courts frequently apply the BGB § 551 three-month cold-rent cap to Bürgschaften by analogy when they substitute for a Mietkaution; `cap_amount` is the normative field for recording this limit. In the United Kingdom, guarantor agreements must explicitly bind the guarantor to lease renewals (`validity_follows_tenancy`) and are often — but not always — required to be executed as deeds (`executed_as_deed`) to be enforceable beyond the fixed term.

**ISO 20022** (financial messaging) ensures that the EBICS transport profile is interoperable with existing bank payment infrastructure across Europe. `pain.001` and `camt.054` are already implemented by every major EU bank for SEPA processing.

---

## 6. Adoption Strategy

XMiete follows the adoption model that has proven effective for xRechnung and xBau: establish the standard first, pilot with early adopters, then seek regulatory endorsement.

**Phase 1 — Specification (2025–2026):** Define the normative schema, the lifecycle state machine, the credential format, and the transport profiles. Publish the reference schema under CC BY 4.0. Establish the AG XMiete working group. *This phase is underway.*

**Phase 2 — Pilot (2026):** Onboard a minimum of two banks, two property management software vendors, and one deposit insurance provider to a joint pilot. Validate the schema against real deposit workflows. Produce a conformance test suite. Publish a pilot report.

**Phase 3 — Industry recommendation (2027):** Seek endorsement from pan-European banking associations (EBF, EACB, ESBG) and national equivalents (Bundesverband deutscher Banken, GdW, DSGV). Position XMiete as the recommended data exchange standard for digital rental deposits across EU member states.

**Phase 4 — Regulatory pathway (2027+):** Engage the European Banking Authority (EBA) and the European Commission (DG FISMA) to propose XMiete as a reference protocol under eIDAS 2.0 for the cross-border use of EUDI Wallet credentials in rental finance. At the national level, seek alignment with digital acceptance mandates — equivalent to the E-Rechnungsverordnung for public bodies — in at least three EU member states.

---

## 7. Stakeholder Map

| Stakeholder | Role | Value proposition |
|---|---|---|
| **Banks** (Sparkassen, Volksbanken, private banks) | Primary implementers; issue DepositPledgeAttestation | Single API contract for all markets; reduced KYC costs via EUDI Wallet re-use; EBICS-native integration |
| **Property management software** (Hausverwaltungssoftware) | Consume XMiete objects; trigger pledge workflows | Machine-readable confirmation replaces manual paper filing; audit-ready lifecycle history |
| **Fintech deposit platforms** (Smartmiete, GetMomo, PlusForta et al.) | Implement XMiete as output format | Interoperability with landlord systems; regulatory alignment; portability of tenant identity |
| **Housing associations & landlords** | Receive pledge confirmations | Digital-first onboarding; no paper; verifiable via EUDI Wallet |
| **Tenants** | Subject of the deposit process | Single EUDI Wallet credential usable across EU; selective disclosure of personal data |
| **Federal government** (BMWSB, BMJ, BMF) | Policy and regulatory anchor | OZG-compliant digital process; eIDAS 2.0 implementation case; potential regulatory mandate |
| **European Banking Authority / ECB** | EU-level institutional stakeholder | Reference implementation for pan-EU deposit standardisation under DORA and eIDAS 2.0 |

---

## 8. What XMiete Is Not

To be precise about scope is to be precise about governance. XMiete explicitly does not:

- **Operate a registry.** XMiete defines the protocol; it does not run a central deposit register. Data stays at the bank and in the tenant's wallet.
- **Set prices.** XMiete has no commercial model. It takes no transaction fee, no licensing fee, and no membership fee. While the specification itself is free and open, the AG XMiete may offer optional certification and support services to ensure industry-wide interoperability and quality control.
- **Choose deposit types.** XMiete supports cash, bank guarantee, insurance, and personal surety deposits equally. It does not advocate for any one product category.
- **Replace national tenancy law.** XMiete profiles national statutes — it does not override them. The `meta.jurisdiction` field carries the applicable legal framework for each deposit object. No harmonisation of substantive tenancy law is required or intended.
- **Control the identity market.** XMiete profiles eID and EUDI Wallet. Provider selection remains with the implementing party.

---

## 9. Founding Conviction

The German internet gained its critical domain infrastructure when DENIC was founded in 1996 — not as a product, but as a neutral cooperative. German public administration gained a common e-invoicing standard when xRechnung was published under KoSIT — not by a vendor, but by a working group with a public mandate. German building permit processes gained a common data standard through xBau — not because a software vendor defined it, but because the public interest required it.

The rental deposit market across Europe has reached the same inflection point. €180 billion in annual deposit volume — spread across 32 jurisdictions, each with its own statutory framework — is locked in analogue workflows because no neutral party has stepped forward to define the shared protocol.

AG XMiete steps forward.

---

*XMiete Core Schema is published under Creative Commons Attribution 4.0 International (CC BY 4.0).*
*This manifest is a living document. Comments and contributions are welcome via the AG XMiete working group.*
*GitHub: github.com/xmiete/xmiete · Web: xmiete.org*
