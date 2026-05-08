# XMiete Developer Guide

The XMiete Core API digitises the full lifecycle of a rental deposit — from tenant identity verification and legal pledge to QEAA credential issuance and eventual release. This guide covers the REST API and all official SDK integrations.

**Base URL:** `https://api.xmiete.org/v1`

**Supported SDK languages:** Java · Go · Rust · Python *(coming soon)* · TypeScript *(coming soon)*

---

## Table of Contents

1. [Authentication](#authentication)
2. [REST API Reference](#rest-api-reference)
3. [Deposit Lifecycle](#deposit-lifecycle)
4. [SDK Installation](#sdk-installation)
5. [Auth — Token Validation](#auth--token-validation)
6. [eID Verification](#eid-verification)
7. [Webhook Handling](#webhook-handling)
8. [OpenID4VP — Wallet Credential Presentation](#openid4vp--wallet-credential-presentation)
9. [Error Handling](#error-handling)

---

## Authentication

All requests require an **OAuth2 Bearer token**.

```
Authorization: Bearer <access_token>
```

### Required scopes

| Endpoint | Scope |
|---|---|
| `POST /deposits` | `deposit:create` |
| `GET /deposits/{id}` | `deposit:read` |
| `PATCH /deposits/{id}/identity` | `deposit:write` |
| `POST /deposits/{id}/pledge` | `deposit:pledge` |
| `POST /deposits/{id}/release` | `deposit:release` |
| `POST /deposits/{id}/claim` | `deposit:claim` |

For mTLS and JWS signing requirements on critical state transitions, see [SECURITY.md](../SECURITY.md).

---

## REST API Reference

### Application Management

#### `POST /deposits`
Creates a new deposit request. The deposit starts in state `REQUESTED`.

**Request body:** Full `xmiete_schema.json` object (omit `history` and `pledge`).

**Response:** `201 Created` — the created deposit object with server-assigned `id`.

---

#### `GET /deposits/{id}`
Returns the current state and full history of a deposit.

**Response:** `200 OK` — full deposit object including `history[]`.

---

### Identity & Verification

#### `PATCH /deposits/{id}/identity`
Reports the eID verification outcome. Moves deposit to `IDENTIFIED` on success.

```json
{
  "eid_status": "VERIFIED",
  "verification_timestamp": "2026-05-07T10:00:00Z",
  "provider_reference": "EID-ABC-123"
}
```

> Only the provider reference is stored — never raw PII from the eID chip.

---

### Funding & Pledging

#### `POST /deposits/{id}/pledge`
The bank or insurer confirms the legal pledge (BGB § 551). Moves deposit to `PLEDGED`.

```json
{
  "pledge_date": "2026-05-07",
  "is_confirmed_by_bank": true,
  "provider_reference": "PLEDGE-XYZ-789"
}
```

---

### Release & Claims

#### `POST /deposits/{id}/release`
Landlord authorises release of the deposit. Moves deposit to `RELEASED`.

```json
{
  "release_type": "FULL",
  "release_amount": 1500.00,
  "landlord_signature_token": "SIGN-998877"
}
```

#### `POST /deposits/{id}/claim`
Landlord initiates a claim against the deposit. Moves deposit to `CLAIMED`.

```json
{
  "claim_amount": 450.00,
  "reason": "Damages to flooring",
  "evidence_urls": ["https://storage.example.com/photo1.jpg"]
}
```

---

### Webhooks

Register a webhook URL with the XMiete platform to receive asynchronous state change events.

**Event payload:**
```json
{
  "event_type": "deposit.status_changed",
  "deposit_id": "550e8400-e29b-41d4-a716-446655440000",
  "new_state": "PLEDGED",
  "timestamp": "2026-05-07T10:05:00Z"
}
```

All webhook deliveries include an `X-Signature` header — an HMAC-SHA256 hex digest of the raw request body using your shared secret. Always verify this before processing. See [Webhook Handling](#webhook-handling) for per-language examples.

---

## Deposit Lifecycle

```
REQUESTED → IDENTIFIED → FUNDED → PLEDGED → ACTIVE → RELEASED
                                                  └──→ CLAIMED
```

| State | Trigger |
|---|---|
| `REQUESTED` | `POST /deposits` |
| `IDENTIFIED` | `PATCH /deposits/{id}/identity` with `VERIFIED` |
| `FUNDED` | Bank confirms receipt of deposit funds |
| `PLEDGED` | `POST /deposits/{id}/pledge` |
| `ACTIVE` | Lease begins |
| `RELEASED` | `POST /deposits/{id}/release` |
| `CLAIMED` | `POST /deposits/{id}/claim` |

---

## SDK Installation

### Java

Requires Java 17+. No external dependencies beyond the Java standard library — the SDK uses `java.net.http.HttpClient` throughout.

```bash
# Copy the sdk-examples/java directory into your project, or package it as a JAR:
mvn package -f sdk-examples/java/pom.xml
```

### Go

Requires Go 1.21+. Uses only the standard library — no external dependencies.

```bash
go get github.com/xmiete/xmiete-go-sdk
```

Import paths:
```go
import (
    sdk      "github.com/xmiete/xmiete-go-sdk"
    "github.com/xmiete/xmiete-go-sdk/auth"
    "github.com/xmiete/xmiete-go-sdk/eid"
    "github.com/xmiete/xmiete-go-sdk/models"
    "github.com/xmiete/xmiete-go-sdk/openid4vp"
)
```

### Rust

Requires Rust 2021 edition. Add to `Cargo.toml`:

```toml
[dependencies]
xmiete-sdk = { path = "sdk-examples/rust" }
```

Modules:
```rust
use xmiete_sdk::auth;
use xmiete_sdk::eid;
use xmiete_sdk::models;
use xmiete_sdk::openid4vp;
```

### Python *(coming soon)*

```python
# pip install xmiete-sdk
from xmiete import XMieteClient, auth, eid, openid4vp
```

### TypeScript *(coming soon)*

```typescript
// npm install @xmiete/sdk
import { XMieteClient, auth, eid, openid4vp } from '@xmiete/sdk';
```

---

## Auth — Token Validation

Use the `auth` module to validate incoming Bearer tokens from tenants, landlords, or internal services before processing any request.

### Java

```java
var validator = new OidcTokenValidator(
    "https://auth.example.com/.well-known/openid-configuration"
);

// Throws TokenValidationException or InsufficientScopeException on failure.
TokenClaims claims = validator.validateToken(
    request.getHeader("Authorization"),
    "deposit:read"
);

System.out.println("Subject: " + claims.subject());
System.out.println("Expires: " + claims.expiresAt());
```

**Async:**
```java
validator.validateTokenAsync(bearerHeader, "deposit:write")
    .thenAccept(claims -> processRequest(claims));
```

### Go

```go
validator := auth.NewOidcTokenValidator(
    "https://auth.example.com/.well-known/openid-configuration",
)

claims, err := validator.ValidateToken(r.Header.Get("Authorization"), "deposit:read")
if err != nil {
    if auth.IsInsufficientScopeError(err) {
        http.Error(w, err.Error(), http.StatusForbidden)
    } else {
        http.Error(w, err.Error(), http.StatusUnauthorized)
    }
    return
}

fmt.Println("Subject:", claims.Subject)
fmt.Println("Expires:", claims.ExpiresAt)
```

### Rust

```rust
use xmiete_sdk::auth::OidcTokenValidator;

let validator = OidcTokenValidator::new(
    "https://auth.example.com/.well-known/openid-configuration",
);

match validator.validate_token(bearer_header, &["deposit:read"]) {
    Ok(claims) => println!("Subject: {}", claims.subject),
    Err(e) => eprintln!("Auth failed: {}", e),
}
```

### Python *(coming soon)*

```python
validator = OidcTokenValidator(
    "https://auth.example.com/.well-known/openid-configuration"
)
claims = validator.validate_token(request.headers["Authorization"], "deposit:read")
print(claims.subject, claims.expires_at)
```

### TypeScript *(coming soon)*

```typescript
const validator = new OidcTokenValidator(
    'https://auth.example.com/.well-known/openid-configuration'
);
const claims = await validator.validateToken(req.headers.authorization, 'deposit:read');
console.log(claims.subject, claims.expiresAt);
```

> **Production note:** The signature verification in all SDKs is currently stubbed.
> Replace it with a full RS256/ES256 JWKS verification before going to production.
> See the stub comments in each SDK for the recommended library per language.

---

## eID Verification

XMiete supports BSI TR-03130 compatible eID providers (AusweisApp2, Authada, SkIDentity, Bundesdruckerei). All SDKs expose the same three-step flow behind a pluggable interface so you can swap providers without touching application code.

### Flow

```
1. InitiateVerification  →  receive AuthorizationURL
2. Redirect tenant to AuthorizationURL
3. Provider POSTs signed webhook to your endpoint
4. WebhookHandler validates HMAC and calls PATCH /deposits/{id}/identity
```

### Java

```java
var service = new EidVerificationService(
    "https://eid-provider.example.com",
    "https://api.xmiete.org/v1"
);

// Step 1: start session
var req = new EidVerificationRequest(
    depositId, tenantEmail, "https://yourapp.example.com/eid-callback", clientId
);
service.initiateVerification(req)
    .thenAccept(session -> {
        // Step 2: redirect tenant
        response.sendRedirect(session.authorizationUrl());
    });
```

### Go

```go
service := eid.NewVerificationService(
    "https://eid-provider.example.com",
    "https://api.xmiete.org/v1",
)

session, err := service.InitiateVerification(ctx, eid.VerificationRequest{
    DepositID:   depositID,
    TenantEmail: tenantEmail,
    RedirectURI: "https://yourapp.example.com/eid-callback",
    ClientID:    clientID,
})
if err != nil {
    return err
}
http.Redirect(w, r, session.AuthorizationURL, http.StatusFound)
```

### Rust

```rust
use xmiete_sdk::eid::{EidVerificationService, EidVerifier, VerificationRequest};

let service = EidVerificationService::new(
    "https://eid-provider.example.com",
    "https://api.xmiete.org/v1",
);

let session = service.initiate_verification(&VerificationRequest {
    deposit_id: deposit_id.to_string(),
    tenant_email: tenant_email.to_string(),
    redirect_uri: "https://yourapp.example.com/eid-callback".to_string(),
    client_id: client_id.to_string(),
}).await?;

// redirect tenant to session.authorization_url
```

### Python *(coming soon)*

```python
service = EidVerificationService(
    eid_provider_base_url="https://eid-provider.example.com",
    xmiete_api_base_url="https://api.xmiete.org/v1",
)
session = await service.initiate_verification(VerificationRequest(
    deposit_id=deposit_id,
    tenant_email=tenant_email,
    redirect_uri="https://yourapp.example.com/eid-callback",
    client_id=client_id,
))
# redirect tenant to session.authorization_url
```

### TypeScript *(coming soon)*

```typescript
const service = new EidVerificationService({
    eidProviderBaseUrl: 'https://eid-provider.example.com',
    xmieteApiBaseUrl: 'https://api.xmiete.org/v1',
});
const session = await service.initiateVerification({
    depositId, tenantEmail,
    redirectUri: 'https://yourapp.example.com/eid-callback',
    clientId,
});
// redirect tenant to session.authorizationUrl
```

### Custom provider adapter

All SDKs define the identity verifier as an interface/trait. Swap the concrete provider by supplying your own adapter — no other SDK code changes required.

**Go:**
```go
type MyProviderAdapter struct{ /* bank-specific config */ }

func (a *MyProviderAdapter) InitiateVerification(ctx context.Context, req eid.VerificationRequest) (*eid.VerificationSession, error) {
    // call your eID SDK or internal REST service
}

func (a *MyProviderAdapter) UpdateDepositKYCStatus(ctx context.Context, depositID string, payload eid.KYCUpdatePayload, bearerToken string) error {
    // push result to XMiete API
}

handler := eid.NewWebhookHandler(&MyProviderAdapter{}, bearerToken, nil)
```

**Rust:**
```rust
#[async_trait]
impl EidVerifier for MyProviderAdapter {
    async fn initiate_verification(&self, req: &VerificationRequest) -> Result<VerificationSession, EidError> { /* ... */ }
    async fn update_deposit_kyc_status(&self, deposit_id: &str, payload: &KycUpdatePayload, bearer_token: &str) -> Result<(), EidError> { /* ... */ }
}
```

**Java:**
```java
public class MyProviderAdapter implements IdentityVerifier {
    public CompletableFuture<EidVerificationSession> initiateVerification(EidVerificationRequest req) { /* ... */ }
    public CompletableFuture<Void> updateDepositKycStatus(String depositId, KycUpdatePayload payload, String bearerToken) { /* ... */ }
}
```

---

## Webhook Handling

Webhook payloads from the eID provider are signed with **HMAC-SHA256**. The hex digest is sent in the `X-Signature` header. All SDKs verify this with a constant-time comparison to prevent timing attacks.

### Java

```java
var handler = new EidWebhookHandler(service, bearerToken, event -> {
    System.out.println("Verification complete for deposit: " + event.depositId());
});

// In your servlet / Spring @PostMapping:
byte[] body = request.getInputStream().readAllBytes();
String sig = request.getHeader("X-Signature");
handler.handleWebhook(body, sig, webhookSecret);
```

### Go

```go
handler := eid.NewWebhookHandler(service, bearerToken, func(event eid.WebhookEvent) {
    log.Printf("verification complete for deposit %s", event.DepositID)
})

http.HandleFunc("/webhook/eid", func(w http.ResponseWriter, r *http.Request) {
    body, _ := io.ReadAll(r.Body)
    if err := handler.HandleWebhook(body, r.Header.Get("X-Signature"), webhookSecret); err != nil {
        http.Error(w, err.Error(), http.StatusBadRequest)
        return
    }
    w.WriteHeader(http.StatusOK)
})
```

### Rust

```rust
use std::sync::Arc;
use xmiete_sdk::eid::webhook::WebhookHandler;

let handler = WebhookHandler::new(
    Arc::new(service),
    bearer_token.to_string(),
    Some(Arc::new(|event| {
        println!("complete for deposit {}", event.deposit_id);
    })),
);

// In your Axum / Actix handler:
handler.dispatch(raw_body, &signature_header, &webhook_secret).await?;
```

### Python *(coming soon)*

```python
handler = EidWebhookHandler(service, bearer_token, on_complete=lambda evt: print(evt.deposit_id))

@app.post("/webhook/eid")
async def eid_webhook(request: Request):
    body = await request.body()
    sig = request.headers.get("X-Signature", "")
    await handler.handle_webhook(body, sig, WEBHOOK_SECRET)
```

### TypeScript *(coming soon)*

```typescript
const handler = new EidWebhookHandler(service, bearerToken, (event) => {
    console.log('complete for deposit', event.depositId);
});

app.post('/webhook/eid', async (req, res) => {
    await handler.handleWebhook(req.rawBody, req.headers['x-signature'], WEBHOOK_SECRET);
    res.sendStatus(200);
});
```

---

## OpenID4VP — Wallet Credential Presentation

Landlords and property managers can request that tenants present their *DepositPledgeAttestation* QEAA directly from an eIDAS 2.0 EUDI Wallet (SD-JWT VC format).

### Flow

```
1. BuildVpRequest   →  send VpRequest JSON to wallet (via QR or deep-link)
2. Wallet presents  →  wallet POSTs vp_token to your response_uri
3. VerifyVpToken    →  verify SD-JWT VP, KB-JWT, disclosures → VerifiedClaims
```

### Java

```java
var verifier = new OpenId4VpService(
    "https://verifier.yourapp.example.com",
    "https://auth.example.com/.well-known/jwks.json"
);

// Step 1: build request
VpRequestResult result = verifier.buildVpRequest(depositId, responseUri).join();
String nonce = result.nonce();
// Serialize result.vpRequest() to JSON and send to wallet

// Step 3: verify response
VerifiedClaims claims = verifier.verifyVpToken(vpToken, nonce, responseUri).join();
System.out.println("Deposit ID: " + claims.depositId());
System.out.println("Pledge date: " + claims.pledgeDate());
System.out.println("Amount: " + claims.depositAmount().orElse(null));
```

### Go

```go
verifier := openid4vp.NewVpVerifierService(
    "https://verifier.yourapp.example.com",
    "https://auth.example.com/.well-known/jwks.json",
)

// Step 1: build request
result, err := verifier.BuildVpRequest(ctx, depositID, responseURI)
if err != nil {
    return err
}
// persist result.Nonce; send result.VpRequest as JSON to wallet

// Step 3: verify response
claims, err := verifier.VerifyVpToken(ctx, vpToken, result.Nonce, responseURI)
if err != nil {
    return err
}
fmt.Println("Deposit ID:", claims.DepositID)
fmt.Println("Pledge date:", claims.PledgeDate)
```

### Rust

```rust
use xmiete_sdk::openid4vp::service::VpVerifierService;
use xmiete_sdk::openid4vp::VpVerifier;

let verifier = VpVerifierService::new(
    "https://verifier.yourapp.example.com".to_string(),
    "https://auth.example.com/.well-known/jwks.json".to_string(),
);

// Step 1: build request
let (nonce, vp_request) = verifier.build_vp_request(deposit_id, response_uri).await?;
// persist nonce; serialize vp_request and send to wallet

// Step 3: verify response
let claims = verifier.verify_vp_token(vp_token, &nonce, response_uri).await?;
println!("Deposit ID: {}", claims.deposit_id);
println!("Pledge date: {}", claims.pledge_date);
```

### Python *(coming soon)*

```python
verifier = VpVerifierService(
    client_id="https://verifier.yourapp.example.com",
    jwks_uri="https://auth.example.com/.well-known/jwks.json",
)
result = await verifier.build_vp_request(deposit_id, response_uri)
# persist result.nonce; send result.vp_request to wallet

claims = await verifier.verify_vp_token(vp_token, result.nonce, response_uri)
print(claims.deposit_id, claims.pledge_date)
```

### TypeScript *(coming soon)*

```typescript
const verifier = new VpVerifierService({
    clientId: 'https://verifier.yourapp.example.com',
    jwksUri: 'https://auth.example.com/.well-known/jwks.json',
});
const { nonce, vpRequest } = await verifier.buildVpRequest(depositId, responseUri);
// persist nonce; send vpRequest JSON to wallet

const claims = await verifier.verifyVpToken(vpToken, nonce, responseUri);
console.log(claims.depositId, claims.pledgeDate);
```

### What `VerifyVpToken` checks

| Check | Detail |
|---|---|
| Issuer JWT structure | 3-part JWT; base64url payload |
| Credential expiry | `exp` claim in issuer JWT |
| Disclosure integrity | SHA-256 digest of each disclosure must appear in `_sd` array |
| KB-JWT `typ` | Must be `kb+jwt` |
| KB-JWT `nonce` | Must match the nonce from `BuildVpRequest` |
| KB-JWT `aud` | Must match `response_uri` |
| KB-JWT `sd_hash` | SHA-256 over `issuerJWT~disc1~...~discN~` |
| KB-JWT `iat` | Must be within the last 5 minutes |

> **Production note:** ES256 signature verification over the issuer JWT is currently stubbed in all SDKs. Implement it using the p256 / ECDSA library for your language before production use. The KB-JWT holder signature verification is also noted as a TODO.

---

## Error Handling

### HTTP status codes

| Code | Meaning |
|---|---|
| `400 Bad Request` | Malformed JSON or illegal state transition |
| `401 Unauthorized` | Missing or invalid Bearer token |
| `403 Forbidden` | Valid token but insufficient scope for this deposit |
| `404 Not Found` | Deposit ID does not exist |
| `409 Conflict` | Transition not permitted from the current state |

### SDK error types

**Go** — all errors are typed and can be inspected with `errors.As`:
```go
if auth.IsInsufficientScopeError(err) { /* 403 */ }
if auth.IsTokenValidationError(err)   { /* 401 */ }
```

**Rust** — errors are `thiserror` enums with `Display` and `Error` implementations:
```rust
match err {
    AuthError::TokenExpired            => /* 401 */,
    AuthError::InsufficientScope {..}  => /* 403 */,
    VpError::NonceMismatch             => /* replay attack */,
    EidError::ProviderError(status)    => /* eID provider returned {status} */,
}
```

**Java** — unchecked exceptions with descriptive messages:
```java
try {
    claims = validator.validateToken(header, "deposit:read");
} catch (OidcTokenValidator.InsufficientScopeException e) {
    // 403
} catch (OidcTokenValidator.TokenValidationException e) {
    // 401
}
```

---

## Supported eID Providers

| Provider | Notes |
|---|---|
| Generic BSI TR-03130 HTTP | Built-in — compatible with any compliant REST service |
| AusweisApp2 SDK | Bundesdruckerei — local SDK, no external network hop |
| Authada | SaaS, used by several German Sparkassen |
| SkIDentity | OpenID Connect front-end for eID |
| Bundesdruckerei / D-Trust | Direct BSI TR-03130 integration |

---

*Licensed under [Apache 2.0](../LICENSE). API specification licensed under [CC BY 4.0](../LICENSE-SPECIFICATION).*
