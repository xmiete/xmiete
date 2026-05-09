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

import { test } from 'node:test';
import assert from 'node:assert/strict';
import { createHmac } from 'node:crypto';
import {
  WebhookHandler,
  ErrMissingSignature,
  ErrSignatureMismatch,
  ErrMissingFields,
  EidWebhookError,
} from './webhook';
import type {
  IdentityVerifier,
  KYCUpdatePayload,
  VerificationRequest,
  VerificationSession,
  WebhookEvent,
} from './models';

class MockVerifier implements IdentityVerifier {
  kycCalled = 0;
  kycError: Error | null = null;

  async initiateVerification(req: VerificationRequest): Promise<VerificationSession> {
    return {
      sessionId: `${req.depositId}-session`,
      authorizationUrl: 'https://example.com/auth',
      expiresAt: new Date(Date.now() + 15 * 60 * 1000),
    };
  }

  async updateDepositKYCStatus(_id: string, _payload: KYCUpdatePayload, _token: string): Promise<void> {
    this.kycCalled++;
    if (this.kycError) throw this.kycError;
  }
}

function sign(body: Buffer, secret: string): string {
  return createHmac('sha256', secret).update(body).digest('hex');
}

function verifiedBody(): Buffer {
  return Buffer.from(JSON.stringify({
    deposit_id: 'DEP-1',
    status: 'VERIFIED',
    provider_reference: 'ref-abc',
    completed_at: new Date().toISOString(),
  }));
}

test('handleWebhook: valid VERIFIED event calls KYC update and onComplete', async () => {
  const mock = new MockVerifier();
  let completedEvent: WebhookEvent | undefined;
  const handler = new WebhookHandler(mock, 'bearer-tok', (ev) => { completedEvent = ev; });

  const secret = 'test-secret';
  const body = verifiedBody();
  await handler.handleWebhook(body, sign(body, secret), secret);

  assert.equal(mock.kycCalled, 1);
  assert.ok(completedEvent);
  assert.equal(completedEvent?.depositId, 'DEP-1');
});

test('handleWebhook: FAILED status does not call KYC update', async () => {
  const mock = new MockVerifier();
  let completedEvent: WebhookEvent | undefined;
  const handler = new WebhookHandler(mock, 'tok', (ev) => { completedEvent = ev; });

  const secret = 'secret';
  const body = Buffer.from(JSON.stringify({ deposit_id: 'DEP-2', status: 'FAILED' }));
  await handler.handleWebhook(body, sign(body, secret), secret);

  assert.equal(mock.kycCalled, 0);
  assert.ok(completedEvent);
  assert.equal(completedEvent?.status, 'FAILED');
});

test('handleWebhook: missing signature throws ErrMissingSignature', async () => {
  const handler = new WebhookHandler(new MockVerifier(), 'tok');
  const body = verifiedBody();
  await assert.rejects(
    () => handler.handleWebhook(body, '', 'secret'),
    ErrMissingSignature,
  );
});

test('handleWebhook: wrong signature throws ErrSignatureMismatch', async () => {
  const handler = new WebhookHandler(new MockVerifier(), 'tok');
  const body = verifiedBody();
  await assert.rejects(
    () => handler.handleWebhook(body, 'deadbeef', 'secret'),
    ErrSignatureMismatch,
  );
});

test('handleWebhook: missing fields throws ErrMissingFields', async () => {
  const handler = new WebhookHandler(new MockVerifier(), 'tok');
  const secret = 'secret';
  const body = Buffer.from(JSON.stringify({ status: 'VERIFIED' }));
  await assert.rejects(
    () => handler.handleWebhook(body, sign(body, secret), secret),
    ErrMissingFields,
  );
});

test('handleWebhook: unknown status throws EidWebhookError', async () => {
  const handler = new WebhookHandler(new MockVerifier(), 'tok');
  const secret = 'secret';
  const body = Buffer.from(JSON.stringify({ deposit_id: 'DEP-1', status: 'BOGUS' }));
  await assert.rejects(
    () => handler.handleWebhook(body, sign(body, secret), secret),
    EidWebhookError,
  );
});

test('handleWebhook: custom adapter wires in without SDK changes (provider agnosticism)', async () => {
  class CustomAdapter extends MockVerifier {}
  const adapter = new CustomAdapter();
  let done = false;
  const handler = new WebhookHandler(adapter, 'tok', () => { done = true; });

  const secret = 'secret';
  const body = verifiedBody();
  await handler.handleWebhook(body, sign(body, secret), secret);

  assert.equal(adapter.kycCalled, 1);
  assert.ok(done);
});

test('handleWebhook: string body is accepted alongside Buffer', async () => {
  const mock = new MockVerifier();
  const handler = new WebhookHandler(mock, 'tok');
  const secret = 'secret';
  const body = verifiedBody();
  const bodyStr = body.toString('utf8');
  await handler.handleWebhook(bodyStr, sign(body, secret), secret);
  assert.equal(mock.kycCalled, 1);
});

test('handleWebhook: KYC update failure is logged but does not reject', async () => {
  const mock = new MockVerifier();
  mock.kycError = new Error('network error');
  let done = false;
  const handler = new WebhookHandler(mock, 'tok', () => { done = true; });

  const secret = 'secret';
  const body = verifiedBody();
  await handler.handleWebhook(body, sign(body, secret), secret);

  assert.equal(mock.kycCalled, 1);
  assert.ok(done);
});

test('EXPIRED status fires onComplete but does not call KYC', async () => {
  const mock = new MockVerifier();
  let completedEvent: WebhookEvent | undefined;
  const handler = new WebhookHandler(mock, 'tok', (ev) => { completedEvent = ev; });

  const secret = 'secret';
  const body = Buffer.from(JSON.stringify({ deposit_id: 'DEP-3', status: 'EXPIRED', error_code: 'session_timeout' }));
  await handler.handleWebhook(body, sign(body, secret), secret);

  assert.equal(mock.kycCalled, 0);
  assert.equal(completedEvent?.status, 'EXPIRED');
});
