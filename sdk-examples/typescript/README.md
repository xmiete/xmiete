# XMiete TypeScript/Node.js SDK Example

A clean, idiomatic TypeScript client example for the XMiete standard. TypeScript/Node.js is the natural choice for PropTech companies and rental platforms building tenant portals, landlord dashboards, and BFF layers on top of XMiete.

## Requirements

- Node.js 18+ (uses built-in `fetch`, `node:crypto`, and `node:test`)
- TypeScript 5.4+

## Structure

- **`src/models/`** — TypeScript interfaces mapping directly to `xmiete_schema.json`, including `WalletMetadata` for EUDI Wallet credential presentations.
- **`src/auth/`** — `OidcTokenValidator`: OIDC Bearer token validation with typed scope checking.
- **`src/eid/`** — eID verification service, webhook handler with HMAC-SHA256 signature verification, and typed event models.
- **`src/openid4vp/`** — `VpVerifierService`: OpenID4VP request building and SD-JWT VP token verification.
- **`src/client.ts`** — `XMieteClient` interface: typed deposit lifecycle operations.
- **`examples/usage.ts`** — End-to-end integration patterns for all modules.

## Key Features

- **Zero runtime dependencies** — uses Node.js built-ins only (`node:crypto`, `fetch`, `node:test`).
- **Strict TypeScript** — all types derived from the JSON schema; no `any`.
- **HMAC webhook verification** — timing-safe signature check via `timingSafeEqual`.
- **eIDAS 2.0 ready** — `WalletMetadata` covers PID, EAA, QEAA, and MDL credential types.
- **Provider agnostic** — eID verification is abstracted behind `IdentityVerifier`; swap providers without touching any other SDK code.
- **SD-JWT VP verification** — disclosure digest validation, KB-JWT nonce/aud/sd_hash checks.

## Provider Agnosticism

The `eid` module exports an `IdentityVerifier` interface rather than a concrete type. The built-in `VerificationService` targets any BSI TR-03130 compatible HTTP provider, but banks can supply their own adapter:

```typescript
import type { IdentityVerifier, KYCUpdatePayload, VerificationRequest, VerificationSession } from '@xmiete/xmiete-ts-sdk';

class MyProviderAdapter implements IdentityVerifier {
  async initiateVerification(req: VerificationRequest): Promise<VerificationSession> {
    // call your internal eID provider SDK or REST API
  }

  async updateDepositKYCStatus(depositId: string, payload: KYCUpdatePayload, bearerToken: string): Promise<void> {
    // push result to XMiete API
  }
}

// Wire it up — no other SDK code changes required.
const handler = new WebhookHandler(new MyProviderAdapter(), bearerToken, onComplete);
```

Tested providers: Generic BSI TR-03130 HTTP (built-in), AusweisApp2 SDK, Authada, SkIDentity, Bundesdruckerei / D-Trust.

## Authentication

```typescript
// docs:start:auth-ts
const validator = new OidcTokenValidator(
  'https://auth.xmiete.example/.well-known/openid-configuration',
);
try {
  const claims = validator.validateToken(req.headers.authorization ?? '', 'deposit:create');
  console.log(`authenticated: sub=${claims.subject}`);
} catch (err) {
  if (err instanceof InsufficientScopeError) {
    res.writeHead(403).end('forbidden');
  } else {
    res.writeHead(401).end('unauthorized');
  }
}
// docs:end:auth-ts
```

## eID Verification

```typescript
// docs:start:eid-ts
const service = new VerificationService(
  'https://eid-provider.example.com',
  'https://api.xmiete.org/v1',
);
const session = await service.initiateVerification({
  depositId, tenantEmail,
  redirectUri: 'https://yourapp.example.com/eid-callback',
  clientId: 'xmiete-fintech-client',
});
// Redirect tenant to session.authorizationUrl
// docs:end:eid-ts
```

## Webhook Handling

```typescript
// docs:start:webhook-ts
// Use express.raw() to receive the body as a Buffer — required for HMAC verification.
const handler = new WebhookHandler(service, bearerToken, (event) => {
  console.log(`eID done: deposit=${event.depositId} status=${event.status}`);
});

app.post('/webhook/eid', express.raw({ type: 'application/json' }), async (req, res) => {
  try {
    await handler.handleWebhook(req.body, req.headers['x-signature'] as string, webhookSecret);
    res.sendStatus(200);
  } catch (err) {
    res.status(400).send(String(err));
  }
});
// docs:end:webhook-ts
```

## OpenID4VP

```typescript
// docs:start:openid4vp-ts
const verifier = new VpVerifierService(
  'https://verifier.yourapp.example.com',
  'https://auth.example.com/.well-known/jwks.json',
);

// Step 1 — build the VP request and deliver it to the wallet (QR or deep-link).
const { nonce, vpRequest } = await verifier.buildVpRequest(depositId, responseUri);
// Persist nonce. Embed vpRequest in a QR code or deep-link for the wallet.

// Step 3 — wallet POSTs vp_token to responseUri; verify it here.
const claims = await verifier.verifyVpToken(vpToken, nonce, responseUri);
console.log(`verified: deposit=${claims.depositId} bank=${claims.issuingBank}`);
// docs:end:openid4vp-ts
```

## Roadmap

- **QEAA Issuance Flow client** — Implement the bank-side OID4VCI Pre-Authorized Code Flow: create issuance sessions, generate SD-JWT `DepositPledgeAttestation` credentials, expose the token and credential endpoints. The reference implementation lives in the XMiete server (`server/internal/issuance/`); this SDK will expose it as a reusable library.

## How to Run

```bash
npm install
npm test
```
