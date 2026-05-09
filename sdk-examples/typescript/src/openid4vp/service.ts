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
 * OpenID4VP presentation request building and SD-JWT VP verification
 * for DepositPledgeAttestation credentials.
 *
 * Flow:
 *  1. Call VpVerifierService.buildVpRequest → store the returned nonce, send vpRequest to wallet
 *  2. Wallet POSTs a vp_token to your response_uri
 *  3. Call VpVerifierService.verifyVpToken with the stored nonce → receive VerifiedClaims
 */

import { createHash, randomUUID } from 'node:crypto';
import type {
  IssuerClaims,
  VerifiedClaims,
  VpRequest,
  VpRequestResult,
} from './models';

export interface VpVerifier {
  /**
   * Creates a VP request for a DepositPledgeAttestation presentation.
   * Persist result.nonce and pass it to verifyVpToken.
   */
  buildVpRequest(depositId: string, responseUri: string): Promise<VpRequestResult>;

  /**
   * Verifies a vp_token received from the wallet.
   * vpToken is the SD-JWT VP wire format: issuerJWT~disc1~...~discN~kbJWT
   * expectedNonce must match the nonce from the corresponding buildVpRequest call.
   */
  verifyVpToken(vpToken: string, expectedNonce: string, responseUri: string): Promise<VerifiedClaims>;
}

export class VpVerifierService implements VpVerifier {
  private readonly clientId: string;
  private readonly jwksUri: string;

  constructor(clientId: string, jwksUri: string) {
    this.clientId = clientId;
    this.jwksUri = jwksUri;
  }

  async buildVpRequest(depositId: string, responseUri: string): Promise<VpRequestResult> {
    const nonce = randomUUID();

    const vpRequest: VpRequest = {
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

  async verifyVpToken(
    vpToken: string,
    expectedNonce: string,
    responseUri: string,
  ): Promise<VerifiedClaims> {
    const parts = vpToken.split('~');
    if (!parts.length || !parts[0]) {
      throw new Error('openid4vp: missing issuer JWT in vp_token');
    }

    const issuerJwt = parts[0];
    let kbJwt = '';
    const disclosureEncodings: string[] = [];

    if (parts.length > 1) {
      const last = parts[parts.length - 1];
      if (last) {
        kbJwt = last;
      }
      const discEnd = kbJwt ? parts.length - 1 : parts.length;
      for (let i = 1; i < discEnd; i++) {
        if (parts[i]) disclosureEncodings.push(parts[i]);
      }
    }

    const issuerClaims = parseIssuerJwt(issuerJwt);

    if (Math.floor(Date.now() / 1000) >= issuerClaims.exp) {
      throw new Error('openid4vp: issuer JWT has expired');
    }

    const sdHashSet = new Set(issuerClaims.sdHashes);
    const disclosed = new Map<string, unknown>();

    for (const enc of disclosureEncodings) {
      const digest = sdJwtHash(enc);
      if (!sdHashSet.has(digest)) {
        throw new Error(`openid4vp: disclosure digest ${digest} not in _sd array`);
      }
      const raw = Buffer.from(enc, 'base64url').toString('utf8');
      const arr = JSON.parse(raw) as unknown[];
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

    const claims: VerifiedClaims = {
      credentialId: issuerClaims.credentialId,
      depositId: issuerClaims.depositId,
      pledgeDate: issuerClaims.pledgeDate,
      statutoryBasis: issuerClaims.statutoryBasis,
      issuingBank: issuerClaims.issuingBank,
      verifiedAt: new Date(),
    };

    const amount = disclosed.get('deposit_amount');
    if (typeof amount === 'number') claims.depositAmount = amount;

    const optStr = (key: string): string | undefined => {
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

function parseIssuerJwt(jwt: string): IssuerClaims {
  const parts = jwt.split('.');
  if (parts.length < 3) {
    throw new Error('openid4vp: malformed issuer JWT: expected 3 dot-separated parts');
  }

  let payload: {
    jti?: string;
    deposit_id?: string;
    pledge_date?: string;
    statutory_basis?: string;
    issuing_bank?: string;
    _sd?: string[];
    exp?: number;
  };
  try {
    payload = JSON.parse(Buffer.from(parts[1], 'base64url').toString('utf8'));
  } catch {
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

function verifyKbJwt(
  kbJwt: string,
  issuerJwt: string,
  disclosures: string[],
  expectedNonce: string,
  responseUri: string,
): void {
  const parts = kbJwt.split('.');
  if (parts.length < 2) throw new Error('openid4vp: malformed KB-JWT');

  const header = JSON.parse(Buffer.from(parts[0], 'base64url').toString('utf8')) as { typ?: string };
  if (header.typ !== 'kb+jwt') {
    throw new Error(`openid4vp: KB-JWT typ must be kb+jwt, got "${header.typ}"`);
  }

  const payload = JSON.parse(Buffer.from(parts[1], 'base64url').toString('utf8')) as {
    nonce?: string;
    aud?: string;
    sd_hash?: string;
    iat?: number;
  };

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
function sdJwtHash(s: string): string {
  return createHash('sha256').update(s).digest('base64url');
}
