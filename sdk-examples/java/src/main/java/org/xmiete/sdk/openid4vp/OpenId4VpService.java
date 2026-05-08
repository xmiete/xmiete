/*
 * Copyright 2026 XMiete Core Contributors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 */

package org.xmiete.sdk.openid4vp;

import org.xmiete.sdk.openid4vp.OpenId4VpModels.*;
import org.xmiete.sdk.openid4vp.VpVerifier.VpRequestResult;

import java.net.http.HttpClient;
import java.nio.charset.StandardCharsets;
import java.security.MessageDigest;
import java.security.NoSuchAlgorithmException;
import java.time.Instant;
import java.util.ArrayList;
import java.util.Base64;
import java.util.HashMap;
import java.util.HashSet;
import java.util.List;
import java.util.Map;
import java.util.Optional;
import java.util.Set;
import java.util.UUID;
import java.util.concurrent.CompletableFuture;
import java.util.regex.Matcher;
import java.util.regex.Pattern;

/**
 * Standard OpenID4VP service for DepositPledgeAttestation SD-JWT VC presentations.
 * Uses Java 11 HttpClient only — no external dependencies.
 */
public class OpenId4VpService implements VpVerifier {

    private final String clientId;
    private final String jwksUri;
    private final HttpClient httpClient;

    public OpenId4VpService(String clientId, String jwksUri) {
        this.clientId = clientId;
        this.jwksUri = jwksUri;
        this.httpClient = HttpClient.newHttpClient();
    }

    @Override
    public CompletableFuture<VpRequestResult> buildVpRequest(String depositId, String responseUri) {
        String nonce = UUID.randomUUID().toString();
        String state = UUID.randomUUID().toString();

        List<Field> fields = List.of(
            new Field(List.of("$.vct"),
                Optional.of(new FieldFilter("string", Optional.of("DepositPledgeAttestation"))),
                false),
            new Field(List.of("$.deposit_id"), Optional.empty(), false),
            new Field(List.of("$.pledge_date"), Optional.empty(), false),
            new Field(List.of("$.statutory_basis"), Optional.empty(), false),
            new Field(List.of("$.issuing_bank"), Optional.empty(), false),
            new Field(List.of("$.deposit_amount"), Optional.empty(), false),
            new Field(List.of("$.pledged_until"), Optional.empty(), true)
        );

        Constraints constraints = new Constraints(fields, "required");

        InputDescriptor descriptor = new InputDescriptor(
            "deposit_pledge_attestation",
            Map.of("vc+sd-jwt", new FormatAlgs(List.of("ES256"))),
            constraints
        );

        PresentationDefinition presentationDefinition = new PresentationDefinition(
            UUID.randomUUID().toString(),
            List.of(descriptor)
        );

        VpRequest vpRequest = new VpRequest(
            clientId,
            "vp_token",
            "direct_post",
            responseUri,
            nonce,
            state,
            presentationDefinition
        );

        return CompletableFuture.completedFuture(new VpRequestResult(nonce, vpRequest));
    }

    @Override
    public CompletableFuture<VerifiedClaims> verifyVpToken(
        String vpToken,
        String expectedNonce,
        String responseUri
    ) {
        return CompletableFuture.supplyAsync(() -> {
            String[] parts = vpToken.split("~", -1);
            if (parts.length < 1 || parts[0].isBlank()) {
                throw new VpVerificationException("Invalid vp_token: missing issuer JWT");
            }

            String issuerJwt = parts[0];

            // Last element is KB-JWT if non-empty; middle non-empty elements are disclosures.
            String kbJwt = null;
            List<String> disclosureEncodings = new ArrayList<>();
            for (int i = 1; i < parts.length; i++) {
                if (i == parts.length - 1) {
                    if (!parts[i].isBlank()) kbJwt = parts[i];
                } else {
                    if (!parts[i].isBlank()) disclosureEncodings.add(parts[i]);
                }
            }

            // --- Step 2: Parse issuer JWT payload (no signature verification here) ---
            String[] jwtParts = issuerJwt.split("\\.", -1);
            if (jwtParts.length < 2) {
                throw new VpVerificationException("Malformed issuer JWT");
            }
            String issuerPayloadJson = base64UrlDecodeToString(jwtParts[1]);

            String credentialId  = extractJsonString(issuerPayloadJson, "jti");
            String depositId     = extractJsonString(issuerPayloadJson, "deposit_id");
            String pledgeDate    = extractJsonString(issuerPayloadJson, "pledge_date");
            String legalRef      = extractJsonString(issuerPayloadJson, "statutory_basis");
            String issuingBank   = extractJsonString(issuerPayloadJson, "issuing_bank");
            List<String> sdHashes = extractJsonArray(issuerPayloadJson, "_sd");
            long exp              = extractJsonLong(issuerPayloadJson, "exp");

            if (Instant.now().getEpochSecond() >= exp) {
                throw new VpVerificationException("Issuer JWT has expired");
            }

            Set<String> sdHashSet = new HashSet<>(sdHashes);

            // --- Step 3: Verify and decode disclosures ---
            Map<String, Object> disclosed = new HashMap<>();
            for (String encoding : disclosureEncodings) {
                byte[] rawBytes = Base64.getUrlDecoder().decode(addPadding(encoding));
                String disclosureHash = base64UrlEncodeNoPad(sha256(encoding.getBytes(StandardCharsets.US_ASCII)));
                if (!sdHashSet.contains(disclosureHash)) {
                    throw new VpVerificationException(
                        "Disclosure hash not found in _sd array: " + disclosureHash);
                }
                // Disclosure is a JSON array: [salt, name, value]
                String disclosureJson = new String(rawBytes, StandardCharsets.UTF_8);
                Map.Entry<String, Object> entry = parseDisclosure(disclosureJson);
                if (entry != null) disclosed.put(entry.getKey(), entry.getValue());
            }

            // --- Step 4: Verify KB-JWT ---
            if (kbJwt != null) {
                String[] kbParts = kbJwt.split("\\.", -1);
                if (kbParts.length < 2) {
                    throw new VpVerificationException("Malformed KB-JWT");
                }

                String kbHeaderJson  = base64UrlDecodeToString(kbParts[0]);
                String kbPayloadJson = base64UrlDecodeToString(kbParts[1]);

                String typ = extractJsonString(kbHeaderJson, "typ");
                if (!"kb+jwt".equals(typ)) {
                    throw new VpVerificationException("KB-JWT typ must be kb+jwt, got: " + typ);
                }

                String kbNonce   = extractJsonString(kbPayloadJson, "nonce");
                String kbAud     = extractJsonString(kbPayloadJson, "aud");
                String kbSdHash  = extractJsonString(kbPayloadJson, "sd_hash");
                long   kbIat     = extractJsonLong(kbPayloadJson, "iat");

                if (!expectedNonce.equals(kbNonce)) {
                    throw new VpVerificationException(
                        "KB-JWT nonce mismatch: expected " + expectedNonce + ", got " + kbNonce);
                }

                if (!responseUri.equals(kbAud)) {
                    throw new VpVerificationException(
                        "KB-JWT aud mismatch: expected " + responseUri + ", got " + kbAud);
                }

                // Compute sd_hash over: issuerJWT~disc1~...~discN~
                StringBuilder sdInput = new StringBuilder(issuerJwt).append("~");
                for (String enc : disclosureEncodings) sdInput.append(enc).append("~");
                String expectedSdHash = base64UrlEncodeNoPad(
                    sha256(sdInput.toString().getBytes(StandardCharsets.US_ASCII)));
                if (!expectedSdHash.equals(kbSdHash)) {
                    throw new VpVerificationException("KB-JWT sd_hash mismatch");
                }

                Instant kbIatInstant = Instant.ofEpochSecond(kbIat);
                if (kbIatInstant.isBefore(Instant.now().minusSeconds(300))) {
                    throw new VpVerificationException("KB-JWT iat is older than 5 minutes");
                }

                // TODO: verify KB-JWT ES256 signature using the holder's public key (cnf.jwk from issuer claims)
            }

            // --- Step 5: Build VerifiedClaims ---
            return new VerifiedClaims(
                credentialId,
                depositId,
                pledgeDate,
                legalRef,
                issuingBank,
                Optional.ofNullable((Double) getDouble(disclosed, "deposit_amount")),
                Optional.ofNullable((String) disclosed.get("currency")),
                Optional.ofNullable((String) disclosed.get("property_address")),
                Optional.ofNullable((String) disclosed.get("tenant_first_name")),
                Optional.ofNullable((String) disclosed.get("tenant_last_name")),
                Optional.ofNullable((String) disclosed.get("pledged_until")),
                Instant.now()
            );
        });
    }

    // --- Helper: disclosure parsing ---

    /**
     * Parses disclosure name (index 1) and value (index 2) from a JSON array string.
     * Returns null if parsing fails.
     */
    private String parseDisclosureNameIndexed(String disclosureJson) {
        // Strip outer brackets, split on comma respecting strings.
        List<String> elements = splitJsonArray(disclosureJson);
        if (elements.size() < 2) return null;
        String raw = elements.get(1).trim();
        if (raw.startsWith("\"") && raw.endsWith("\"")) {
            return raw.substring(1, raw.length() - 1);
        }
        return raw;
    }

    private Object parseDisclosureValueIndexed(String disclosureJson) {
        List<String> elements = splitJsonArray(disclosureJson);
        if (elements.size() < 3) return null;
        String raw = elements.get(2).trim();
        if (raw.startsWith("\"") && raw.endsWith("\"")) {
            return raw.substring(1, raw.length() - 1);
        }
        try {
            return Double.parseDouble(raw);
        } catch (NumberFormatException e) {
            return raw;
        }
    }

    private Double getDouble(Map<String, Object> map, String key) {
        Object v = map.get(key);
        if (v instanceof Double d) return d;
        if (v instanceof Number n) return n.doubleValue();
        if (v instanceof String s) {
            try { return Double.parseDouble(s); } catch (NumberFormatException e) { return null; }
        }
        return null;
    }

    /** Splits a JSON array string (e.g. {@code ["salt","name","value"]}) into elements. */
    private List<String> splitJsonArray(String json) {
        String inner = json.trim();
        if (inner.startsWith("[")) inner = inner.substring(1);
        if (inner.endsWith("]")) inner = inner.substring(0, inner.length() - 1);

        List<String> result = new ArrayList<>();
        int depth = 0;
        boolean inString = false;
        boolean escape = false;
        int start = 0;
        for (int i = 0; i < inner.length(); i++) {
            char c = inner.charAt(i);
            if (escape) { escape = false; continue; }
            if (c == '\\' && inString) { escape = true; continue; }
            if (c == '"') { inString = !inString; continue; }
            if (!inString) {
                if (c == '{' || c == '[') depth++;
                else if (c == '}' || c == ']') depth--;
                else if (c == ',' && depth == 0) {
                    result.add(inner.substring(start, i));
                    start = i + 1;
                }
            }
        }
        result.add(inner.substring(start));
        return result;
    }

    // --- Base64 / crypto helpers ---

    private String base64UrlDecodeToString(String encoded) {
        return new String(Base64.getUrlDecoder().decode(addPadding(encoded)), StandardCharsets.UTF_8);
    }

    private String base64UrlEncodeNoPad(byte[] bytes) {
        return Base64.getUrlEncoder().withoutPadding().encodeToString(bytes);
    }

    private String addPadding(String s) {
        int pad = (4 - s.length() % 4) % 4;
        return s + "=".repeat(pad);
    }

    private byte[] sha256(byte[] input) {
        try {
            return MessageDigest.getInstance("SHA-256").digest(input);
        } catch (NoSuchAlgorithmException e) {
            throw new IllegalStateException("SHA-256 not available", e);
        }
    }

    // --- JSON extraction helpers (regex-based, no external library) ---

    private static String extractJsonString(String json, String key) {
        if (json == null) return null;
        int idx = json.indexOf("\"" + key + "\"");
        if (idx < 0) return null;
        int colon = json.indexOf(':', idx);
        int start = json.indexOf('"', colon + 1);
        if (start < 0) return null;
        int end = json.indexOf('"', start + 1);
        return end > start ? json.substring(start + 1, end) : null;
    }

    private static List<String> extractJsonArray(String json, String key) {
        List<String> result = new ArrayList<>();
        if (json == null) return result;
        int idx = json.indexOf("\"" + key + "\"");
        if (idx < 0) return result;
        int arrStart = json.indexOf('[', idx);
        int arrEnd   = json.indexOf(']', arrStart);
        if (arrStart < 0 || arrEnd < 0) return result;
        String arrContent = json.substring(arrStart + 1, arrEnd);
        Matcher m = Pattern.compile("\"([^\"]+)\"").matcher(arrContent);
        while (m.find()) result.add(m.group(1));
        return result;
    }

    private static double extractJsonDouble(String json, String key) {
        if (json == null) return 0.0;
        int idx = json.indexOf("\"" + key + "\"");
        if (idx < 0) return 0.0;
        int colon = json.indexOf(':', idx);
        int start = colon + 1;
        while (start < json.length() && Character.isWhitespace(json.charAt(start))) start++;
        int end = start;
        while (end < json.length() && (Character.isDigit(json.charAt(end))
            || json.charAt(end) == '.' || json.charAt(end) == '-')) end++;
        try {
            return Double.parseDouble(json.substring(start, end));
        } catch (NumberFormatException e) {
            return 0.0;
        }
    }

    private static long extractJsonLong(String json, String key) {
        return (long) extractJsonDouble(json, key);
    }

    // --- Disclosure parsing (delegates to indexed helpers) ---

    private Map.Entry<String, Object> parseDisclosure(String disclosureJson) {
        String name  = parseDisclosureNameIndexed(disclosureJson);
        Object value = parseDisclosureValueIndexed(disclosureJson);
        if (name == null) return null;
        return Map.entry(name, value != null ? value : "");
    }

    public static class VpVerificationException extends RuntimeException {
        public VpVerificationException(String msg) { super(msg); }
    }
}
