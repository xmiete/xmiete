/*
 * Copyright 2026 XMiete Core Contributors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

// Usage examples for the XMiete TypeScript/Node.js SDK.
// This file is excluded from the build (not under src/) and is for illustration only.
// Each section demonstrates one integration pattern end-to-end.

import type { IncomingMessage, ServerResponse } from 'node:http';
import {
  OidcTokenValidator,
  TokenValidationError,
  InsufficientScopeError,
} from '../src/auth/index';
import { VerificationService, WebhookHandler } from '../src/eid/index';
import type { IdentityVerifier } from '../src/eid/index';
import { VpVerifierService } from '../src/openid4vp/index';

// ── Authentication ────────────────────────────────────────────────────────────

// docs:start:auth-ts
function handleRequest(req: IncomingMessage, res: ServerResponse): void {
  const validator = new OidcTokenValidator(
    'https://auth.xmiete.example/.well-known/openid-configuration',
  );
  try {
    const claims = validator.validateToken(
      req.headers.authorization ?? '',
      'deposit:create',
    );
    console.log(`authenticated: sub=${claims.subject} expires=${claims.expiresAt.toISOString()}`);
    // ... process the request
  } catch (err) {
    if (err instanceof InsufficientScopeError) {
      res.writeHead(403).end('forbidden');
    } else {
      res.writeHead(401).end('unauthorized');
    }
  }
}
// docs:end:auth-ts

// ── eID Verification ──────────────────────────────────────────────────────────

// docs:start:eid-ts
async function initiateEIDVerification(depositId: string, tenantEmail: string): Promise<string> {
  const service = new VerificationService(
    'https://eid-provider.example.com', // e.g., Authada, SkIDentity
    'https://api.xmiete.org/v1',
  );
  const session = await service.initiateVerification({
    depositId,
    tenantEmail,
    redirectUri: 'https://yourapp.example.com/eid-callback',
    clientId: 'xmiete-fintech-client',
  });
  // Redirect the tenant's browser to session.authorizationUrl.
  // The provider POSTs the result to your /webhook/eid endpoint.
  return session.authorizationUrl;
}
// docs:end:eid-ts

// ── Webhook Handling ──────────────────────────────────────────────────────────

// docs:start:webhook-ts
function registerEIDWebhook(
  service: IdentityVerifier,
  bearerToken: string,
  webhookSecret: string,
): (req: IncomingMessage, res: ServerResponse) => void {
  const handler = new WebhookHandler(service, bearerToken, (event) => {
    console.log(`eID done: deposit=${event.depositId} status=${event.status}`);
  });

  return async (req, res) => {
    const chunks: Buffer[] = [];
    for await (const chunk of req) chunks.push(chunk as Buffer);
    const body = Buffer.concat(chunks);

    try {
      await handler.handleWebhook(
        body,
        (req.headers['x-signature'] as string) ?? '',
        webhookSecret,
      );
      res.writeHead(200).end();
    } catch (err) {
      res.writeHead(400).end(String(err));
    }
  };
}
// docs:end:webhook-ts

// ── OpenID4VP ─────────────────────────────────────────────────────────────────

// docs:start:openid4vp-ts
async function requestWalletPresentation(depositId: string, responseUri: string): Promise<void> {
  const verifier = new VpVerifierService(
    'https://verifier.yourapp.example.com',
    'https://auth.example.com/.well-known/jwks.json',
  );

  // Step 1 — build the VP request and deliver it to the wallet (QR or deep-link).
  const { nonce, vpRequest } = await verifier.buildVpRequest(depositId, responseUri);
  // Persist nonce in your session store — required for step 3.
  // Serialize vpRequest as JSON and embed in QR code or deep-link.
  console.log('VP request built, nonce:', nonce, 'request:', JSON.stringify(vpRequest));

  // Step 3 — wallet POSTs vp_token to responseUri; verify it here.
  const claims = await verifier.verifyVpToken('<vp_token from wallet>', nonce, responseUri);
  console.log(
    `verified: deposit=${claims.depositId} pledged=${claims.pledgeDate} bank=${claims.issuingBank}`,
  );
}
// docs:end:openid4vp-ts

// ── Error Handling ────────────────────────────────────────────────────────────

// docs:start:error-ts
function handleAuthError(res: ServerResponse, err: unknown): void {
  if (err instanceof InsufficientScopeError) {
    // HTTP 403 — valid token, but the required scope is not granted
    res.writeHead(403).end('insufficient scope');
  } else if (err instanceof TokenValidationError) {
    // HTTP 401 — malformed JWT, expired, or wrong issuer
    res.writeHead(401).end('unauthorized');
  } else {
    res.writeHead(500).end('internal error');
  }
}
// docs:end:error-ts

// Prevent unused-variable warnings in this illustration file.
void handleRequest;
void initiateEIDVerification;
void registerEIDWebhook;
void requestWalletPresentation;
void handleAuthError;
