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
export declare class TokenValidationError extends Error {
    constructor(message: string);
}
export declare class InsufficientScopeError extends Error {
    readonly required: string;
    readonly present: string[];
    constructor(required: string, present: string[]);
}
export interface TokenClaims {
    subject: string;
    issuer: string;
    scopes: Set<string>;
    expiresAt: Date;
}
export declare function hasScope(claims: TokenClaims, scope: string): boolean;
export declare class OidcTokenValidator {
    /** JWKS endpoint derived from the discovery URL. */
    readonly jwksUri: string;
    /** Required value for the `iss` claim. */
    readonly expectedIssuer: string;
    constructor(oidcDiscoveryUrl: string);
    /**
     * Validates a Bearer token and checks every required scope.
     * The bearerToken may be a raw JWT or prefixed with "Bearer ".
     */
    validateToken(bearerToken: string, ...requiredScopes: string[]): TokenClaims;
    private parseAndValidateClaims;
    private checkScopes;
}
