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

import java.time.Instant;
import java.util.List;
import java.util.Map;
import java.util.Optional;

public final class OpenId4VpModels {

    private OpenId4VpModels() {}

    /** OpenID4VP Authorization Request Object sent to the wallet. */
    public record VpRequest(
        String clientId,                             // JSON: "client_id"
        String responseType,                         // "vp_token"
        String responseMode,                         // "direct_post"
        String responseUri,                          // JSON: "response_uri"
        String nonce,
        String state,
        PresentationDefinition presentationDefinition // JSON: "presentation_definition"
    ) {}

    /** DIF Presentation Exchange — top-level definition. */
    public record PresentationDefinition(
        String id,
        List<InputDescriptor> inputDescriptors       // JSON: "input_descriptors"
    ) {}

    /** Describes what credential the verifier is requesting. */
    public record InputDescriptor(
        String id,
        Map<String, FormatAlgs> format,
        Constraints constraints
    ) {}

    /** Supported signing algorithms for a given credential format. */
    public record FormatAlgs(List<String> alg) {}

    /** Field constraints on the requested credential. */
    public record Constraints(
        List<Field> fields,
        String limitDisclosure                       // JSON: "limit_disclosure", value "required"
    ) {}

    /** A single JSONPath-addressed field constraint. */
    public record Field(
        List<String> path,
        Optional<FieldFilter> filter,
        boolean optional
    ) {}

    /** JSON Schema-style filter applied to a field value. */
    public record FieldFilter(
        String type,
        Optional<String> constValue                  // JSON key: "const"
    ) {}

    /** Claims extracted from a successfully verified DepositPledgeAttestation VP. */
    public record VerifiedClaims(
        String credentialId,
        String depositId,
        String pledgeDate,
        String statutoryBasis,
        String issuingBank,
        Optional<Double> depositAmount,
        Optional<String> currency,
        Optional<String> propertyAddress,
        Optional<String> tenantFirstName,
        Optional<String> tenantLastName,
        Optional<String> pledgedUntil,
        Instant verifiedAt
    ) {}

    /** Parsed payload of the SD-JWT issuer JWT. */
    public record IssuerClaims(
        String credentialId,                         // JWT claim: "jti"
        String depositId,
        String pledgeDate,
        String statutoryBasis,
        String issuingBank,
        List<String> sdHashes,                       // JSON: "_sd" array
        long exp
    ) {}
}
