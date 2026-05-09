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
exports.OidcTokenValidator = exports.InsufficientScopeError = exports.TokenValidationError = void 0;
exports.hasScope = hasScope;
/**
 * OAuth2/OIDC Bearer token validation for the XMiete SDK.
 *
 * Usage:
 *   const validator = new OidcTokenValidator('https://auth.example.com/.well-known/openid-configuration');
 *   const claims = validator.validateToken(req.headers.authorization, 'deposit:read');
 *
 * Production note: replace the signature stub with a JWKS-backed ES256/RS256 verifier.
 * Use a library such as 'jose' (npm) for JWKS fetching and signature verification.
 */
class TokenValidationError extends Error {
    constructor(message) {
        super('auth: ' + message);
        this.name = 'TokenValidationError';
    }
}
exports.TokenValidationError = TokenValidationError;
class InsufficientScopeError extends Error {
    required;
    present;
    constructor(required, present) {
        super(`auth: required scope "${required}" not in token: [${present.join(', ')}]`);
        this.name = 'InsufficientScopeError';
        this.required = required;
        this.present = present;
    }
}
exports.InsufficientScopeError = InsufficientScopeError;
function hasScope(claims, scope) {
    return claims.scopes.has(scope);
}
class OidcTokenValidator {
    /** JWKS endpoint derived from the discovery URL. */
    jwksUri;
    /** Required value for the `iss` claim. */
    expectedIssuer;
    constructor(oidcDiscoveryUrl) {
        const issuer = oidcDiscoveryUrl.replace('/.well-known/openid-configuration', '');
        this.expectedIssuer = issuer;
        this.jwksUri = `${issuer}/.well-known/jwks.json`;
    }
    /**
     * Validates a Bearer token and checks every required scope.
     * The bearerToken may be a raw JWT or prefixed with "Bearer ".
     */
    validateToken(bearerToken, ...requiredScopes) {
        const token = bearerToken.startsWith('Bearer ') ? bearerToken.slice(7) : bearerToken;
        const claims = this.parseAndValidateClaims(token);
        this.checkScopes(claims, requiredScopes);
        return claims;
    }
    parseAndValidateClaims(jwt) {
        const parts = jwt.split('.');
        if (parts.length !== 3) {
            throw new TokenValidationError(`malformed JWT: expected 3 parts, got ${parts.length}`);
        }
        let payload;
        try {
            const raw = Buffer.from(parts[1], 'base64url').toString('utf8');
            payload = JSON.parse(raw);
        }
        catch {
            throw new TokenValidationError('failed to decode JWT payload');
        }
        if (!payload.sub)
            throw new TokenValidationError('missing claim: sub');
        if (!payload.iss)
            throw new TokenValidationError('missing claim: iss');
        if (!payload.exp)
            throw new TokenValidationError('missing claim: exp');
        if (payload.iss !== this.expectedIssuer) {
            throw new TokenValidationError(`unexpected issuer: "${payload.iss}"`);
        }
        if (Math.floor(Date.now() / 1000) > payload.exp) {
            throw new TokenValidationError(`token expired at epoch ${payload.exp}`);
        }
        // Production: verify RS256/ES256 signature using JWKS at this.jwksUri.
        // Fetch JWKS, match key by `kid` from the JWT header, then verify with node:crypto or 'jose'.
        const scopes = new Set((payload.scope ?? '').split(' ').filter(Boolean));
        return {
            subject: payload.sub,
            issuer: payload.iss,
            scopes,
            expiresAt: new Date(payload.exp * 1000),
        };
    }
    checkScopes(claims, required) {
        for (const scope of required) {
            if (!claims.scopes.has(scope)) {
                throw new InsufficientScopeError(scope, [...claims.scopes]);
            }
        }
    }
}
exports.OidcTokenValidator = OidcTokenValidator;
