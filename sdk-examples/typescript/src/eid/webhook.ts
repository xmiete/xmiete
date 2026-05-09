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
 * WebhookHandler processes signed webhook events from the eID provider.
 *
 * Mount on your HTTP server at the endpoint registered with the provider:
 *
 *   // Express — use express.raw() so the body arrives as a Buffer for HMAC verification
 *   app.post('/webhook/eid', express.raw({ type: 'application/json' }), async (req, res) => {
 *     try {
 *       await handler.handleWebhook(req.body, req.headers['x-signature'] as string, webhookSecret);
 *       res.sendStatus(200);
 *     } catch (err) {
 *       res.status(400).send(String(err));
 *     }
 *   });
 */

import { createHmac, timingSafeEqual } from 'node:crypto';
import type { IdentityVerifier, KYCUpdatePayload, WebhookEvent } from './models';
import { EID_STATUSES } from './models';

export class EidWebhookError extends Error {
  constructor(message: string) {
    super(message);
    this.name = 'EidWebhookError';
  }
}

export const ErrMissingSignature = new EidWebhookError('eid: missing X-Signature header');
export const ErrSignatureMismatch = new EidWebhookError('eid: HMAC signature mismatch');
export const ErrMissingFields = new EidWebhookError('eid: webhook missing required fields: deposit_id, status');

export class WebhookHandler {
  private readonly service: IdentityVerifier;
  private readonly bearerToken: string;
  private readonly onComplete?: (event: WebhookEvent) => void;

  constructor(
    service: IdentityVerifier,
    bearerToken: string,
    onComplete?: (event: WebhookEvent) => void,
  ) {
    this.service = service;
    this.bearerToken = bearerToken;
    this.onComplete = onComplete;
  }

  /**
   * Validates the HMAC signature, parses the event, and dispatches the result.
   * rawBody must be the unmodified request body; signature is the X-Signature header value.
   */
  async handleWebhook(
    rawBody: Buffer | string,
    signature: string,
    webhookSecret: string,
  ): Promise<void> {
    const bodyBuf = Buffer.isBuffer(rawBody) ? rawBody : Buffer.from(rawBody, 'utf8');
    verifyHmac(bodyBuf, signature, webhookSecret);
    const event = parseWebhookEvent(bodyBuf);
    await this.dispatch(event);
  }

  private async dispatch(event: WebhookEvent): Promise<void> {
    if (event.status === 'VERIFIED') {
      const payload: KYCUpdatePayload = {
        eidStatus: 'VERIFIED',
        verificationTimestamp: event.completedAt,
        providerReference: event.providerReference,
      };
      try {
        await this.service.updateDepositKYCStatus(event.depositId, payload, this.bearerToken);
      } catch (err) {
        console.error(`eid: KYC update failed for deposit ${event.depositId}:`, err);
      }
      this.onComplete?.(event);
    } else if (event.status === 'FAILED' || event.status === 'EXPIRED') {
      console.warn(`eid: verification ${event.status} for deposit ${event.depositId} (error: "${event.errorCode}")`);
      this.onComplete?.(event);
    }
  }
}

export function verifyHmac(body: Buffer, signature: string, secret: string): void {
  if (!signature) {
    throw ErrMissingSignature;
  }
  const expected = createHmac('sha256', secret).update(body).digest('hex');
  const expectedBuf = Buffer.from(expected, 'utf8');
  const actualBuf = Buffer.from(signature, 'utf8');
  if (
    expectedBuf.length !== actualBuf.length ||
    !timingSafeEqual(expectedBuf, actualBuf)
  ) {
    throw ErrSignatureMismatch;
  }
}

export function parseWebhookEvent(data: Buffer): WebhookEvent {
  let raw: Record<string, string>;
  try {
    raw = JSON.parse(data.toString('utf8')) as Record<string, string>;
  } catch {
    throw new EidWebhookError('eid: parse webhook body: invalid JSON');
  }

  if (!raw['deposit_id'] || !raw['status']) {
    throw ErrMissingFields;
  }

  const status = raw['status'] as WebhookEvent['status'];
  if (!(EID_STATUSES as readonly string[]).includes(status)) {
    throw new EidWebhookError(`eid: unknown status value: "${raw['status']}"`);
  }

  let completedAt = new Date();
  if (raw['completed_at']) {
    const parsed = new Date(raw['completed_at']);
    if (!isNaN(parsed.getTime())) completedAt = parsed;
  }

  return {
    sessionId: raw['session_id'] ?? '',
    depositId: raw['deposit_id'],
    status,
    providerReference: raw['provider_reference'] ?? '',
    completedAt,
    errorCode: raw['error_code'] ?? '',
  };
}
