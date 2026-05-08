# XMiete Security Protocols

This document defines the mandatory and recommended security protocols for implementing the XMiete standard. Adherence to these protocols ensures financial-grade security and trust between all stakeholders.

## 1. Transport Layer Security

### Mandatory: TLS 1.3
All communication between endpoints MUST use **TLS 1.3**. Legacy versions (TLS 1.2 and below) SHOULD be disabled to prevent downgrade attacks.

### Recommended: Mutual TLS (mTLS)
For service-to-service communication between Banks and Property Management Systems, **mTLS** is strongly recommended.
- Both parties must present a valid certificate issued by a trusted Certificate Authority (CA) or a mutually agreed-upon private CA.
- This ensures that only authorized systems can establish a connection at the network level.

## 2. Authentication & Authorization

### OAuth 2.0 / OpenID Connect (OIDC)
All API requests must be authenticated using **OAuth 2.0 Access Tokens**.

#### Scopes
Access should be restricted using granular scopes:
- `deposit:read`: View deposit status and history.
- `deposit:create`: Initiate a new deposit request (Tenant/Fintech).
- `deposit:pledge`: Confirm the legal pledge (Bank only).
- `deposit:release`: Authorize the release of funds (Landlord/Manager only).
- `deposit:claim`: Initiate a claim (Landlord/Manager only).

## 3. Data Integrity & Non-Repudiation

### JSON Web Signatures (JWS)
For critical lifecycle transitions, the standard supports **JWS (RFC 7515)** to ensure the payload has not been tampered with and to provide non-repudiation.
- **Mandatory for**: `PLEDGED`, `RELEASED`, and `CLAIMED` transitions.
- The `history` entries for these states SHOULD contain a `signature` field holding the JWS.

## 4. Identity Verification (KYC)

### eID (Online-Ausweisfunktion)
As per the XMiete standard, identity verification for tenants SHOULD prioritize the German **eID**.
- The `eid_status` field must be updated only after a successful cryptographic verification of the eID chip.
- Implementers MUST store a reference to the verification transaction (not the PII itself) for auditing purposes.

## 5. Security Checklist for Implementers

- [ ] TLS 1.3 is enforced.
- [ ] OAuth 2.0 tokens are validated on every request.
- [ ] Granular scopes are checked (RBAC/ABAC).
- [ ] mTLS is used for bank-to-manager interfaces.
- [ ] PII (Personally Identifiable Information) is encrypted at rest.
- [ ] Logging does not include sensitive data (Secrets, Tax IDs, IBANs).
- [ ] Critical state changes are signed via JWS.

---
**License:** Licensed under [Creative Commons Attribution 4.0 International (CC BY 4.0)](./LICENSE-SPECIFICATION).
