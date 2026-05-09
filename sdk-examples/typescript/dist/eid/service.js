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
exports.VerificationService = void 0;
class VerificationService {
    eidProviderBaseUrl;
    xmieteApiBaseUrl;
    constructor(eidProviderBaseUrl, xmieteApiBaseUrl) {
        this.eidProviderBaseUrl = eidProviderBaseUrl.replace(/\/$/, '');
        this.xmieteApiBaseUrl = xmieteApiBaseUrl.replace(/\/$/, '');
    }
    /** Creates an eID session and returns the authorization redirect URL. */
    async initiateVerification(req) {
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
        const result = await res.json();
        return {
            sessionId: result.session_id ?? `${req.depositId}-session`,
            authorizationUrl: result.authorization_url ??
                `${this.eidProviderBaseUrl}/authorize?deposit_id=${encodeURIComponent(req.depositId)}`,
            expiresAt: new Date(Date.now() + 15 * 60 * 1000),
        };
    }
    /**
     * Pushes the verified eID result to the XMiete API.
     * Only providerReference is forwarded — never raw PII from the eID chip.
     */
    async updateDepositKYCStatus(depositId, payload, bearerToken) {
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
exports.VerificationService = VerificationService;
