/*
 * Copyright 2026 XMiete Core Contributors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 */

package org.xmiete.sdk.auth;

import java.time.Instant;
import java.util.Set;

public record TokenClaims(
    String subject,
    String issuer,
    Set<String> scopes,
    Instant expiresAt
) {
    public boolean hasScope(String scope) {
        return scopes.contains(scope);
    }
}
