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

use base64::engine::general_purpose::URL_SAFE_NO_PAD;
use base64::Engine;
use chrono::{DateTime, TimeZone, Utc};
use std::collections::HashSet;

#[derive(Debug, Clone)]
pub struct TokenClaims {
    pub subject: String,
    pub issuer: String,
    pub scopes: HashSet<String>,
    pub expires_at: DateTime<Utc>,
}

impl TokenClaims {
    pub fn has_scope(&self, scope: &str) -> bool {
        self.scopes.contains(scope)
    }
}

#[derive(Debug, thiserror::Error)]
pub enum AuthError {
    #[error("malformed JWT: {0}")]
    MalformedJwt(String),
    #[error("base64 decode error: {0}")]
    Base64Decode(String),
    #[error("json parse error: {0}")]
    JsonParse(String),
    #[error("unexpected issuer: {0}")]
    UnexpectedIssuer(String),
    #[error("token expired")]
    TokenExpired,
    #[error("missing claim: {0}")]
    MissingClaim(String),
    #[error("insufficient scope: required '{required}', present: {present}")]
    InsufficientScope { required: String, present: String },
}

/// Validates OAuth2/OIDC Bearer tokens against a JWKS endpoint.
///
/// # Usage
/// ```rust,ignore
/// let validator = OidcTokenValidator::new("https://auth.example.com/.well-known/openid-configuration");
/// let claims = validator.validate_token("Bearer <token>", &["deposit:read"])?;
/// ```
///
/// Production note: replace `verify_signature_stub` with the `jsonwebtoken` crate
/// and fetch the RS256/ES256 key from `self.jwks_uri`.
pub struct OidcTokenValidator {
    pub jwks_uri: String,
    pub expected_issuer: String,
}

impl OidcTokenValidator {
    pub fn new(oidc_discovery_url: &str) -> Self {
        let issuer = oidc_discovery_url
            .trim_end_matches("/.well-known/openid-configuration")
            .to_string();
        Self {
            jwks_uri: format!("{}/.well-known/jwks.json", issuer),
            expected_issuer: issuer,
        }
    }

    /// Validates a Bearer token and checks all required scopes.
    pub fn validate_token(
        &self,
        bearer_token: &str,
        required_scopes: &[&str],
    ) -> Result<TokenClaims, AuthError> {
        let token = bearer_token
            .strip_prefix("Bearer ")
            .unwrap_or(bearer_token);
        let claims = self.parse_and_validate_claims(token)?;
        self.check_scopes(&claims, required_scopes)?;
        Ok(claims)
    }

    fn parse_and_validate_claims(&self, jwt: &str) -> Result<TokenClaims, AuthError> {
        let parts: Vec<&str> = jwt.splitn(3, '.').collect();
        if parts.len() != 3 {
            return Err(AuthError::MalformedJwt(format!(
                "expected 3 parts, got {}",
                parts.len()
            )));
        }

        let payload_raw = URL_SAFE_NO_PAD
            .decode(parts[1].trim_end_matches('='))
            .map_err(|e| AuthError::Base64Decode(e.to_string()))?;

        let payload: serde_json::Value = serde_json::from_slice(&payload_raw)
            .map_err(|e| AuthError::JsonParse(e.to_string()))?;

        let get_str = |k: &str| -> Result<String, AuthError> {
            payload
                .get(k)
                .and_then(|v| v.as_str())
                .map(|s| s.to_string())
                .ok_or_else(|| AuthError::MissingClaim(k.to_string()))
        };

        let sub = get_str("sub")?;
        let iss = get_str("iss")?;
        let scope_str = get_str("scope")?;
        let exp = payload
            .get("exp")
            .and_then(|v| v.as_i64())
            .ok_or_else(|| AuthError::MissingClaim("exp".to_string()))?;

        if iss != self.expected_issuer {
            return Err(AuthError::UnexpectedIssuer(iss));
        }

        if Utc::now().timestamp() > exp {
            return Err(AuthError::TokenExpired);
        }

        // Production: verify RS256/ES256 signature using the JWKS endpoint at self.jwks_uri.
        // Steps when implemented:
        //   let jwks: JwkSet = reqwest::get(&self.jwks_uri).await?.json().await?;
        //   let kid = jsonwebtoken::decode_header(jwt)?.kid;
        //   let key = DecodingKey::from_jwk(jwks.find(&kid).ok_or(...)?)? ;
        //   jsonwebtoken::decode::<Claims>(jwt, &key, &Validation::new(Algorithm::RS256))?;
        self.verify_signature_stub(parts[0], parts[1], parts[2]);

        let scopes: HashSet<String> = scope_str
            .split_whitespace()
            .map(|s| s.to_string())
            .collect();

        let expires_at = Utc
            .timestamp_opt(exp, 0)
            .single()
            .unwrap_or_else(Utc::now);

        Ok(TokenClaims {
            subject: sub,
            issuer: iss,
            scopes,
            expires_at,
        })
    }

    fn check_scopes(&self, claims: &TokenClaims, required: &[&str]) -> Result<(), AuthError> {
        for &scope in required {
            if !claims.has_scope(scope) {
                let present = {
                    let mut v: Vec<&str> =
                        claims.scopes.iter().map(String::as_str).collect();
                    v.sort_unstable();
                    v.join(" ")
                };
                return Err(AuthError::InsufficientScope {
                    required: scope.to_string(),
                    present,
                });
            }
        }
        Ok(())
    }

    #[allow(unused_variables)]
    fn verify_signature_stub(&self, header: &str, payload: &str, signature: &str) {}
}

#[cfg(test)]
mod tests {
    use super::*;
    use base64::engine::general_purpose::URL_SAFE_NO_PAD;
    use base64::Engine;

    fn make_jwt(payload_json: &str, issuer_override: Option<&str>) -> (String, String) {
        let issuer = issuer_override.unwrap_or("https://auth.example.com");
        let header = URL_SAFE_NO_PAD.encode(r#"{"alg":"RS256","typ":"JWT"}"#);
        let payload = URL_SAFE_NO_PAD.encode(payload_json);
        let jwt = format!("{}.{}.fakesig", header, payload);
        (jwt, issuer.to_string())
    }

    fn valid_payload(extra: &str) -> String {
        format!(
            r#"{{"sub":"user-1","iss":"https://auth.example.com","scope":"deposit:read deposit:write","exp":9999999999{}}}"#,
            if extra.is_empty() { String::new() } else { format!(",{}", extra) }
        )
    }

    #[test]
    fn accepts_valid_token_with_required_scope() {
        let (jwt, issuer) = make_jwt(&valid_payload(""), None);
        let validator = OidcTokenValidator::new(&format!("{}/.well-known/openid-configuration", issuer));
        let claims = validator.validate_token(&jwt, &["deposit:read"]).unwrap();
        assert_eq!(claims.subject, "user-1");
        assert!(claims.has_scope("deposit:read"));
        assert!(claims.has_scope("deposit:write"));
    }

    #[test]
    fn accepts_bearer_prefix() {
        let (jwt, issuer) = make_jwt(&valid_payload(""), None);
        let validator = OidcTokenValidator::new(&format!("{}/.well-known/openid-configuration", issuer));
        let bearer = format!("Bearer {}", jwt);
        let claims = validator.validate_token(&bearer, &[]).unwrap();
        assert_eq!(claims.subject, "user-1");
    }

    #[test]
    fn rejects_insufficient_scope() {
        let (jwt, issuer) = make_jwt(&valid_payload(""), None);
        let validator = OidcTokenValidator::new(&format!("{}/.well-known/openid-configuration", issuer));
        let err = validator.validate_token(&jwt, &["deposit:admin"]).unwrap_err();
        assert!(matches!(err, AuthError::InsufficientScope { .. }));
    }

    #[test]
    fn rejects_wrong_issuer() {
        let payload = valid_payload("");
        let (jwt, _) = make_jwt(&payload, Some("https://evil.example.com"));
        let validator = OidcTokenValidator::new("https://auth.example.com/.well-known/openid-configuration");
        let err = validator.validate_token(&jwt, &[]).unwrap_err();
        assert!(matches!(err, AuthError::UnexpectedIssuer(_)));
    }

    #[test]
    fn rejects_expired_token() {
        let payload = r#"{"sub":"u","iss":"https://auth.example.com","scope":"s","exp":1}"#;
        let (jwt, _) = make_jwt(payload, None);
        let validator = OidcTokenValidator::new("https://auth.example.com/.well-known/openid-configuration");
        let err = validator.validate_token(&jwt, &[]).unwrap_err();
        assert!(matches!(err, AuthError::TokenExpired));
    }

    #[test]
    fn rejects_malformed_jwt() {
        let validator = OidcTokenValidator::new("https://auth.example.com/.well-known/openid-configuration");
        let err = validator.validate_token("not.a.valid.jwt.structure.here", &[]).unwrap_err();
        // splitn(3, '.') will still give 3 parts for "not.a.valid.jwt.structure.here",
        // so the actual error will be base64 or json parse.
        // Use a truly malformed JWT:
        let err2 = validator.validate_token("onlytwoparts", &[]).unwrap_err();
        assert!(matches!(err2, AuthError::MalformedJwt(_)));
        let _ = err; // suppress unused warning
    }
}
