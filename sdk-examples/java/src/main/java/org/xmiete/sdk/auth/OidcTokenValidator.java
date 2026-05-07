/*
 * Copyright 2024 XMiete Core Contributors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 */

package org.xmiete.sdk.auth;

import java.net.URI;
import java.net.http.HttpClient;
import java.net.http.HttpRequest;
import java.net.http.HttpResponse;
import java.nio.charset.StandardCharsets;
import java.time.Instant;
import java.util.Arrays;
import java.util.Base64;
import java.util.HashSet;
import java.util.Set;
import java.util.concurrent.CompletableFuture;
import java.util.regex.Matcher;
import java.util.regex.Pattern;

/**
 * Validates OAuth2/OIDC Bearer tokens against a JWKS endpoint.
 *
 * Usage:
 *   var validator = new OidcTokenValidator("https://auth.example.com/.well-known/openid-configuration");
 *   TokenClaims claims = validator.validateToken(authHeader, "deposit:read");
 *
 * Production note: replace the signature stub with Nimbus JOSE+JWT or
 * Auth0 java-jwt for RS256/ES256 verification against the JWKS endpoint.
 */
public class OidcTokenValidator {

    private final HttpClient httpClient;
    private final String jwksUri;
    private final String expectedIssuer;

    public OidcTokenValidator(String oidcDiscoveryUrl) {
        this.httpClient = HttpClient.newHttpClient();
        String issuer = oidcDiscoveryUrl.replaceAll("/.well-known/openid-configuration$", "");
        this.jwksUri = issuer + "/.well-known/jwks.json";
        this.expectedIssuer = issuer;
    }

    /**
     * Validates a Bearer token synchronously and checks all required scopes.
     * Throws {@link TokenValidationException} or {@link InsufficientScopeException} on failure.
     */
    public TokenClaims validateToken(String bearerToken, String... requiredScopes) {
        String token = bearerToken.startsWith("Bearer ") ? bearerToken.substring(7) : bearerToken;
        TokenClaims claims = parseAndValidateClaims(token);
        checkScopes(claims, requiredScopes);
        return claims;
    }

    public CompletableFuture<TokenClaims> validateTokenAsync(String bearerToken, String... requiredScopes) {
        return CompletableFuture.supplyAsync(() -> validateToken(bearerToken, requiredScopes));
    }

    private TokenClaims parseAndValidateClaims(String jwt) {
        String[] parts = jwt.split("\\.");
        if (parts.length != 3) {
            throw new TokenValidationException("Malformed JWT: expected 3 parts, got " + parts.length);
        }

        String payloadJson = new String(
            Base64.getUrlDecoder().decode(padBase64(parts[1])),
            StandardCharsets.UTF_8
        );

        String sub = extractClaim(payloadJson, "sub");
        String iss = extractClaim(payloadJson, "iss");
        String scopeStr = extractClaim(payloadJson, "scope");
        long exp = Long.parseLong(extractClaim(payloadJson, "exp"));

        if (!expectedIssuer.equals(iss)) {
            throw new TokenValidationException("Unexpected issuer: " + iss);
        }
        if (Instant.now().getEpochSecond() > exp) {
            throw new TokenValidationException("Token expired at epoch " + exp);
        }

        // Production: verify RS256/ES256 signature using the JWKS endpoint at jwksUri.
        // Requires Nimbus JOSE+JWT: JWKSet.load(new URL(jwksUri)) + JWSVerifier.
        verifySignatureStub(parts);

        Set<String> scopes = new HashSet<>(Arrays.asList(scopeStr.split(" ")));
        return new TokenClaims(sub, iss, scopes, Instant.ofEpochSecond(exp));
    }

    private void checkScopes(TokenClaims claims, String[] required) {
        for (String scope : required) {
            if (!claims.hasScope(scope)) {
                throw new InsufficientScopeException(scope, claims.scopes());
            }
        }
    }

    @SuppressWarnings("unused")
    private void verifySignatureStub(String[] jwtParts) {
        // Replace with:
        //   JWSObject jws = JWSObject.parse(String.join(".", jwtParts));
        //   JWKSet jwks = JWKSet.load(new URL(jwksUri));
        //   RSAKey rsaKey = (RSAKey) jwks.getKeyByKeyId(jws.getHeader().getKeyID());
        //   jws.verify(new RSASSAVerifier(rsaKey.toRSAPublicKey()));
    }

    private static String extractClaim(String json, String key) {
        Pattern p = Pattern.compile("\"" + key + "\"\\s*:\\s*\"?([^\"\\s,}]+)\"?");
        Matcher m = p.matcher(json);
        if (!m.find()) {
            throw new TokenValidationException("Missing claim: " + key);
        }
        return m.group(1);
    }

    private static String padBase64(String s) {
        int padding = (4 - s.length() % 4) % 4;
        return s + "=".repeat(padding);
    }

    public static class TokenValidationException extends RuntimeException {
        public TokenValidationException(String msg) { super(msg); }
    }

    public static class InsufficientScopeException extends RuntimeException {
        public InsufficientScopeException(String missing, Set<String> present) {
            super("Required scope '" + missing + "' not in token: " + present);
        }
    }
}
