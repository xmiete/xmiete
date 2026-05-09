"use strict";
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
var __importDefault = (this && this.__importDefault) || function (mod) {
    return (mod && mod.__esModule) ? mod : { "default": mod };
};
Object.defineProperty(exports, "__esModule", { value: true });
const node_test_1 = require("node:test");
const strict_1 = __importDefault(require("node:assert/strict"));
const node_crypto_1 = require("node:crypto");
const webhook_1 = require("./webhook");
class MockVerifier {
    kycCalled = 0;
    kycError = null;
    async initiateVerification(req) {
        return {
            sessionId: `${req.depositId}-session`,
            authorizationUrl: 'https://example.com/auth',
            expiresAt: new Date(Date.now() + 15 * 60 * 1000),
        };
    }
    async updateDepositKYCStatus(_id, _payload, _token) {
        this.kycCalled++;
        if (this.kycError)
            throw this.kycError;
    }
}
function sign(body, secret) {
    return (0, node_crypto_1.createHmac)('sha256', secret).update(body).digest('hex');
}
function verifiedBody() {
    return Buffer.from(JSON.stringify({
        deposit_id: 'DEP-1',
        status: 'VERIFIED',
        provider_reference: 'ref-abc',
        completed_at: new Date().toISOString(),
    }));
}
(0, node_test_1.test)('handleWebhook: valid VERIFIED event calls KYC update and onComplete', async () => {
    const mock = new MockVerifier();
    let completedEvent;
    const handler = new webhook_1.WebhookHandler(mock, 'bearer-tok', (ev) => { completedEvent = ev; });
    const secret = 'test-secret';
    const body = verifiedBody();
    await handler.handleWebhook(body, sign(body, secret), secret);
    strict_1.default.equal(mock.kycCalled, 1);
    strict_1.default.ok(completedEvent);
    strict_1.default.equal(completedEvent?.depositId, 'DEP-1');
});
(0, node_test_1.test)('handleWebhook: FAILED status does not call KYC update', async () => {
    const mock = new MockVerifier();
    let completedEvent;
    const handler = new webhook_1.WebhookHandler(mock, 'tok', (ev) => { completedEvent = ev; });
    const secret = 'secret';
    const body = Buffer.from(JSON.stringify({ deposit_id: 'DEP-2', status: 'FAILED' }));
    await handler.handleWebhook(body, sign(body, secret), secret);
    strict_1.default.equal(mock.kycCalled, 0);
    strict_1.default.ok(completedEvent);
    strict_1.default.equal(completedEvent?.status, 'FAILED');
});
(0, node_test_1.test)('handleWebhook: missing signature throws ErrMissingSignature', async () => {
    const handler = new webhook_1.WebhookHandler(new MockVerifier(), 'tok');
    const body = verifiedBody();
    await strict_1.default.rejects(() => handler.handleWebhook(body, '', 'secret'), webhook_1.ErrMissingSignature);
});
(0, node_test_1.test)('handleWebhook: wrong signature throws ErrSignatureMismatch', async () => {
    const handler = new webhook_1.WebhookHandler(new MockVerifier(), 'tok');
    const body = verifiedBody();
    await strict_1.default.rejects(() => handler.handleWebhook(body, 'deadbeef', 'secret'), webhook_1.ErrSignatureMismatch);
});
(0, node_test_1.test)('handleWebhook: missing fields throws ErrMissingFields', async () => {
    const handler = new webhook_1.WebhookHandler(new MockVerifier(), 'tok');
    const secret = 'secret';
    const body = Buffer.from(JSON.stringify({ status: 'VERIFIED' }));
    await strict_1.default.rejects(() => handler.handleWebhook(body, sign(body, secret), secret), webhook_1.ErrMissingFields);
});
(0, node_test_1.test)('handleWebhook: unknown status throws EidWebhookError', async () => {
    const handler = new webhook_1.WebhookHandler(new MockVerifier(), 'tok');
    const secret = 'secret';
    const body = Buffer.from(JSON.stringify({ deposit_id: 'DEP-1', status: 'BOGUS' }));
    await strict_1.default.rejects(() => handler.handleWebhook(body, sign(body, secret), secret), webhook_1.EidWebhookError);
});
(0, node_test_1.test)('handleWebhook: custom adapter wires in without SDK changes (provider agnosticism)', async () => {
    class CustomAdapter extends MockVerifier {
    }
    const adapter = new CustomAdapter();
    let done = false;
    const handler = new webhook_1.WebhookHandler(adapter, 'tok', () => { done = true; });
    const secret = 'secret';
    const body = verifiedBody();
    await handler.handleWebhook(body, sign(body, secret), secret);
    strict_1.default.equal(adapter.kycCalled, 1);
    strict_1.default.ok(done);
});
(0, node_test_1.test)('handleWebhook: string body is accepted alongside Buffer', async () => {
    const mock = new MockVerifier();
    const handler = new webhook_1.WebhookHandler(mock, 'tok');
    const secret = 'secret';
    const body = verifiedBody();
    const bodyStr = body.toString('utf8');
    await handler.handleWebhook(bodyStr, sign(body, secret), secret);
    strict_1.default.equal(mock.kycCalled, 1);
});
(0, node_test_1.test)('handleWebhook: KYC update failure is logged but does not reject', async () => {
    const mock = new MockVerifier();
    mock.kycError = new Error('network error');
    let done = false;
    const handler = new webhook_1.WebhookHandler(mock, 'tok', () => { done = true; });
    const secret = 'secret';
    const body = verifiedBody();
    await handler.handleWebhook(body, sign(body, secret), secret);
    strict_1.default.equal(mock.kycCalled, 1);
    strict_1.default.ok(done);
});
(0, node_test_1.test)('EXPIRED status fires onComplete but does not call KYC', async () => {
    const mock = new MockVerifier();
    let completedEvent;
    const handler = new webhook_1.WebhookHandler(mock, 'tok', (ev) => { completedEvent = ev; });
    const secret = 'secret';
    const body = Buffer.from(JSON.stringify({ deposit_id: 'DEP-3', status: 'EXPIRED', error_code: 'session_timeout' }));
    await handler.handleWebhook(body, sign(body, secret), secret);
    strict_1.default.equal(mock.kycCalled, 0);
    strict_1.default.equal(completedEvent?.status, 'EXPIRED');
});
