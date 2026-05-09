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
exports.VpVerifierService = void 0;
/**
 * OpenID4VP presentation request building and SD-JWT VP verification
 * for DepositPledgeAttestation credentials.
 *
 * Flow:
 *  1. Call VpVerifierService.buildVpRequest → store the returned nonce, send vpRequest to wallet
 *  2. Wallet POSTs a vp_token to your response_uri
 *  3. Call VpVerifierService.verifyVpToken with the stored nonce → receive VerifiedClaims
 */
const node_crypto_1 = require("node:crypto");
class VpVerifierService {
    clientId;
    jwksUri;
    constructor(clientId, jwksUri) {
        this.clientId = clientId;
        this.jwksUri = jwksUri;
    }
    async buildVpRequest(depositId, responseUri) {
        const nonce = (0, node_crypto_1.randomUUID)();
        const vpRequest = {
            client_id: this.clientId,
            response_type: 'vp_token',
            response_mode: 'direct_post',
            response_uri: responseUri,
            nonce,
            state: depositId,
            presentation_definition: {
                id: 'deposit-pledge-attestation-pd',
                input_descriptors: [
                    {
                        id: 'deposit-pledge-attestation',
                        format: { 'vc+sd-jwt': { alg: ['ES256'] } },
                        constraints: {
                            limit_disclosure: 'required',
                            fields: [
                                { path: ['$.vct'], filter: { type: 'string', const: 'DepositPledgeAttestation' } },
                                { path: ['$.deposit_id'] },
                                { path: ['$.pledge_date'] },
                                { path: ['$.statutory_basis'] },
                                { path: ['$.issuing_bank'] },
                                { path: ['$.deposit_amount'], optional: true },
                                { path: ['$.currency'], optional: true },
                                { path: ['$.property_address'], optional: true },
                                { path: ['$.tenant_first_name'], optional: true },
                                { path: ['$.tenant_last_name'], optional: true },
                                { path: ['$.pledged_until'], optional: true },
                            ],
                        },
                    },
                ],
            },
        };
        return { nonce, vpRequest };
    }
    async verifyVpToken(vpToken, expectedNonce, responseUri) {
        const parts = vpToken.split('~');
        if (!parts.length || !parts[0]) {
            throw new Error('openid4vp: missing issuer JWT in vp_token');
        }
        const issuerJwt = parts[0];
        let kbJwt = '';
        const disclosureEncodings = [];
        if (parts.length > 1) {
            const last = parts[parts.length - 1];
            if (last) {
                kbJwt = last;
            }
            const discEnd = kbJwt ? parts.length - 1 : parts.length;
            for (let i = 1; i < discEnd; i++) {
                if (parts[i])
                    disclosureEncodings.push(parts[i]);
            }
        }
        const issuerClaims = parseIssuerJwt(issuerJwt);
        if (Math.floor(Date.now() / 1000) >= issuerClaims.exp) {
            throw new Error('openid4vp: issuer JWT has expired');
        }
        const sdHashSet = new Set(issuerClaims.sdHashes);
        const disclosed = new Map();
        for (const enc of disclosureEncodings) {
            const digest = sdJwtHash(enc);
            if (!sdHashSet.has(digest)) {
                throw new Error(`openid4vp: disclosure digest ${digest} not in _sd array`);
            }
            const raw = Buffer.from(enc, 'base64url').toString('utf8');
            const arr = JSON.parse(raw);
            if (!Array.isArray(arr) || arr.length < 3) {
                throw new Error('openid4vp: disclosure must be a JSON array with at least 3 elements');
            }
            const name = arr[1];
            if (typeof name !== 'string') {
                throw new Error('openid4vp: disclosure name is not a string');
            }
            disclosed.set(name, arr[2]);
        }
        if (kbJwt) {
            verifyKbJwt(kbJwt, issuerJwt, disclosureEncodings, expectedNonce, responseUri);
        }
        // Production: verify ES256 issuer JWT signature using JWKS at this.jwksUri.
        const claims = {
            credentialId: issuerClaims.credentialId,
            depositId: issuerClaims.depositId,
            pledgeDate: issuerClaims.pledgeDate,
            statutoryBasis: issuerClaims.statutoryBasis,
            issuingBank: issuerClaims.issuingBank,
            verifiedAt: new Date(),
        };
        const amount = disclosed.get('deposit_amount');
        if (typeof amount === 'number')
            claims.depositAmount = amount;
        const optStr = (key) => {
            const v = disclosed.get(key);
            return typeof v === 'string' ? v : undefined;
        };
        claims.currency = optStr('currency');
        claims.propertyAddress = optStr('property_address');
        claims.tenantFirstName = optStr('tenant_first_name');
        claims.tenantLastName = optStr('tenant_last_name');
        claims.pledgedUntil = optStr('pledged_until');
        return claims;
    }
}
exports.VpVerifierService = VpVerifierService;
function parseIssuerJwt(jwt) {
    const parts = jwt.split('.');
    if (parts.length < 3) {
        throw new Error('openid4vp: malformed issuer JWT: expected 3 dot-separated parts');
    }
    let payload;
    try {
        payload = JSON.parse(Buffer.from(parts[1], 'base64url').toString('utf8'));
    }
    catch {
        throw new Error('openid4vp: failed to decode issuer JWT payload');
    }
    return {
        credentialId: payload.jti ?? '',
        depositId: payload.deposit_id ?? '',
        pledgeDate: payload.pledge_date ?? '',
        statutoryBasis: payload.statutory_basis ?? '',
        issuingBank: payload.issuing_bank ?? '',
        sdHashes: payload._sd ?? [],
        exp: payload.exp ?? 0,
    };
}
function verifyKbJwt(kbJwt, issuerJwt, disclosures, expectedNonce, responseUri) {
    const parts = kbJwt.split('.');
    if (parts.length < 2)
        throw new Error('openid4vp: malformed KB-JWT');
    const header = JSON.parse(Buffer.from(parts[0], 'base64url').toString('utf8'));
    if (header.typ !== 'kb+jwt') {
        throw new Error(`openid4vp: KB-JWT typ must be kb+jwt, got "${header.typ}"`);
    }
    const payload = JSON.parse(Buffer.from(parts[1], 'base64url').toString('utf8'));
    if (payload.nonce !== expectedNonce) {
        throw new Error(`openid4vp: KB-JWT nonce mismatch: expected "${expectedNonce}", got "${payload.nonce}"`);
    }
    if (payload.aud !== responseUri) {
        throw new Error(`openid4vp: KB-JWT aud mismatch: expected "${responseUri}", got "${payload.aud}"`);
    }
    const sdInput = [issuerJwt, ...disclosures, ''].join('~');
    const expectedSdHash = sdJwtHash(sdInput);
    if (payload.sd_hash !== expectedSdHash) {
        throw new Error('openid4vp: KB-JWT sd_hash mismatch');
    }
    const now = Math.floor(Date.now() / 1000);
    if ((payload.iat ?? 0) > now + 30 || (payload.iat ?? 0) < now - 300) {
        throw new Error('openid4vp: KB-JWT iat is stale or from the future');
    }
    // TODO: verify KB-JWT ES256 signature using the holder's public key (cnf.jwk from issuer claims).
}
/** Computes the base64url-unpadded SHA-256 digest of s, as required by the SD-JWT spec. */
function sdJwtHash(s) {
    return (0, node_crypto_1.createHash)('sha256').update(s).digest('base64url');
}
