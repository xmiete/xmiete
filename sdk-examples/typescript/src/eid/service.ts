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

/**
 * EidVerificationService manages eID verification sessions against a BSI TR-03130
 * compatible eID provider (e.g., AusweisApp2 SDK, Authada, SkIDentity).
 *
 * Flow:
 *  1. Call initiateVerification → receive VerificationSession with authorizationUrl
 *  2. Redirect the tenant's browser to authorizationUrl
 *  3. eID provider POSTs to your webhook → handled by WebhookHandler
 *  4. WebhookHandler calls updateDepositKYCStatus to finalize the deposit state
 */

import type {
  IdentityVerifier,
  KYCUpdatePayload,
  VerificationRequest,
  VerificationSession,
} from './models';

export class VerificationService implements IdentityVerifier {
  private readonly eidProviderBaseUrl: string;
  private readonly xmieteApiBaseUrl: string;

  constructor(eidProviderBaseUrl: string, xmieteApiBaseUrl: string) {
    this.eidProviderBaseUrl = eidProviderBaseUrl.replace(/\/$/, '');
    this.xmieteApiBaseUrl = xmieteApiBaseUrl.replace(/\/$/, '');
  }

  /** Creates an eID session and returns the authorization redirect URL. */
  async initiateVerification(req: VerificationRequest): Promise<VerificationSession> {
    const res = await fetch(`${this.eidProviderBaseUrl}/sessions`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        client_id: req.clientId,
        deposit_id: req.depositId,
        tenant_email: req.tenantEmail,
        redirect_uri: req.redirectUri,
        scope: 'openid eid',
      }),
      signal: AbortSignal.timeout(10_000),
    });

    if (res.status !== 201) {
      throw new Error(`eid: provider returned HTTP ${res.status}`);
    }

    const result = await res.json() as { session_id?: string; authorization_url?: string };
    return {
      sessionId: result.session_id ?? `${req.depositId}-session`,
      authorizationUrl:
        result.authorization_url ??
        `${this.eidProviderBaseUrl}/authorize?deposit_id=${encodeURIComponent(req.depositId)}`,
      expiresAt: new Date(Date.now() + 15 * 60 * 1000),
    };
  }

  /**
   * Pushes the verified eID result to the XMiete API.
   * Only providerReference is forwarded — never raw PII from the eID chip.
   */
  async updateDepositKYCStatus(
    depositId: string,
    payload: KYCUpdatePayload,
    bearerToken: string,
  ): Promise<void> {
    const res = await fetch(`${this.xmieteApiBaseUrl}/deposits/${encodeURIComponent(depositId)}/identity`, {
      method: 'PATCH',
      headers: {
        'Content-Type': 'application/json',
        Authorization: `Bearer ${bearerToken}`,
      },
      body: JSON.stringify({
        eid_status: payload.eidStatus,
        verification_timestamp: payload.verificationTimestamp.toISOString(),
        provider_reference: payload.providerReference,
      }),
      signal: AbortSignal.timeout(10_000),
    });

    if (res.status !== 200 && res.status !== 204) {
      throw new Error(`eid: KYC update failed: HTTP ${res.status}`);
    }
  }
}
