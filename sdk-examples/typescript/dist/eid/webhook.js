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
Object.defineProperty(exports, "__esModule", { value: true });
exports.WebhookHandler = exports.ErrMissingFields = exports.ErrSignatureMismatch = exports.ErrMissingSignature = exports.EidWebhookError = void 0;
exports.verifyHmac = verifyHmac;
exports.parseWebhookEvent = parseWebhookEvent;
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
const node_crypto_1 = require("node:crypto");
const models_1 = require("./models");
class EidWebhookError extends Error {
    constructor(message) {
        super(message);
        this.name = 'EidWebhookError';
    }
}
exports.EidWebhookError = EidWebhookError;
exports.ErrMissingSignature = new EidWebhookError('eid: missing X-Signature header');
exports.ErrSignatureMismatch = new EidWebhookError('eid: HMAC signature mismatch');
exports.ErrMissingFields = new EidWebhookError('eid: webhook missing required fields: deposit_id, status');
class WebhookHandler {
    service;
    bearerToken;
    onComplete;
    constructor(service, bearerToken, onComplete) {
        this.service = service;
        this.bearerToken = bearerToken;
        this.onComplete = onComplete;
    }
    /**
     * Validates the HMAC signature, parses the event, and dispatches the result.
     * rawBody must be the unmodified request body; signature is the X-Signature header value.
     */
    async handleWebhook(rawBody, signature, webhookSecret) {
        const bodyBuf = Buffer.isBuffer(rawBody) ? rawBody : Buffer.from(rawBody, 'utf8');
        verifyHmac(bodyBuf, signature, webhookSecret);
        const event = parseWebhookEvent(bodyBuf);
        await this.dispatch(event);
    }
    async dispatch(event) {
        if (event.status === 'VERIFIED') {
            const payload = {
                eidStatus: 'VERIFIED',
                verificationTimestamp: event.completedAt,
                providerReference: event.providerReference,
            };
            try {
                await this.service.updateDepositKYCStatus(event.depositId, payload, this.bearerToken);
            }
            catch (err) {
                console.error(`eid: KYC update failed for deposit ${event.depositId}:`, err);
            }
            this.onComplete?.(event);
        }
        else if (event.status === 'FAILED' || event.status === 'EXPIRED') {
            console.warn(`eid: verification ${event.status} for deposit ${event.depositId} (error: "${event.errorCode}")`);
            this.onComplete?.(event);
        }
    }
}
exports.WebhookHandler = WebhookHandler;
function verifyHmac(body, signature, secret) {
    if (!signature) {
        throw exports.ErrMissingSignature;
    }
    const expected = (0, node_crypto_1.createHmac)('sha256', secret).update(body).digest('hex');
    const expectedBuf = Buffer.from(expected, 'utf8');
    const actualBuf = Buffer.from(signature, 'utf8');
    if (expectedBuf.length !== actualBuf.length ||
        !(0, node_crypto_1.timingSafeEqual)(expectedBuf, actualBuf)) {
        throw exports.ErrSignatureMismatch;
    }
}
function parseWebhookEvent(data) {
    let raw;
    try {
        raw = JSON.parse(data.toString('utf8'));
    }
    catch {
        throw new EidWebhookError('eid: parse webhook body: invalid JSON');
    }
    if (!raw['deposit_id'] || !raw['status']) {
        throw exports.ErrMissingFields;
    }
    const status = raw['status'];
    if (!models_1.EID_STATUSES.includes(status)) {
        throw new EidWebhookError(`eid: unknown status value: "${raw['status']}"`);
    }
    let completedAt = new Date();
    if (raw['completed_at']) {
        const parsed = new Date(raw['completed_at']);
        if (!isNaN(parsed.getTime()))
            completedAt = parsed;
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
