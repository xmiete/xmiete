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

use async_trait::async_trait;
use base64::engine::general_purpose::URL_SAFE_NO_PAD;
use base64::Engine;

use super::{
    build_vp_request_for_deposit, verify_sd_jwt_vp, IssuerClaims, VerifiedClaims, VpError,
    VpRequest, VpVerifier,
};

pub struct VpVerifierService {
    pub client_id: String,
    pub jwks_uri: String,
    http_client: reqwest::Client,
}

impl VpVerifierService {
    pub fn new(client_id: String, jwks_uri: String) -> Self {
        Self {
            client_id,
            jwks_uri,
            http_client: reqwest::Client::new(),
        }
    }

    // TODO: verify ES256 signature using the p256 crate; add p256 = "0.13" to Cargo.toml
    pub fn parse_issuer_jwt_claims(&self, jwt: &str) -> Result<IssuerClaims, VpError> {
        let parts: Vec<&str> = jwt.splitn(3, '.').collect();
        if parts.len() < 3 {
            return Err(VpError::InvalidFormat(
                "issuer JWT must have 3 dot-separated parts".to_string(),
            ));
        }

        let payload_raw = URL_SAFE_NO_PAD
            .decode(parts[1])
            .map_err(|e| VpError::Base64Decode(e.to_string()))?;
        let payload: serde_json::Value = serde_json::from_slice(&payload_raw)
            .map_err(|e| VpError::JsonParse(e.to_string()))?;

        let get_str = |k: &str| -> Result<String, VpError> {
            payload
                .get(k)
                .and_then(|v| v.as_str())
                .map(|s| s.to_string())
                .ok_or_else(|| VpError::InvalidFormat(format!("missing claim: {}", k)))
        };

        let sd_hashes = payload
            .get("_sd")
            .and_then(|v| v.as_array())
            .map(|arr| {
                arr.iter()
                    .filter_map(|v| v.as_str())
                    .map(|s| s.to_string())
                    .collect()
            })
            .unwrap_or_default();

        let sd_alg = payload
            .get("_sd_alg")
            .and_then(|v| v.as_str())
            .unwrap_or("sha-256")
            .to_string();

        let exp = payload
            .get("exp")
            .and_then(|v| v.as_i64())
            .unwrap_or(0);

        Ok(IssuerClaims {
            credential_id: payload
                .get("jti")
                .and_then(|v| v.as_str())
                .unwrap_or("")
                .to_string(),
            deposit_id: get_str("deposit_id")?,
            pledge_date: get_str("pledge_date")?,
            legal_reference: get_str("legal_reference")?,
            issuing_bank: get_str("issuing_bank")?,
            issuing_bank_id: payload
                .get("issuing_bank_id")
                .and_then(|v| v.as_str())
                .map(|s| s.to_string()),
            property_id: payload
                .get("property_id")
                .and_then(|v| v.as_str())
                .map(|s| s.to_string()),
            sd_hashes,
            sd_alg,
            exp,
        })
    }

    // Fetches the JWKS from jwks_uri. Signature verification is stubbed out pending p256 integration.
    pub async fn fetch_and_verify_issuer_jwt(&self, jwt: &str) -> Result<IssuerClaims, VpError> {
        // TODO: verify ES256 signature using the p256 crate; add p256 = "0.13" to Cargo.toml
        // Steps when implemented:
        //   1. Fetch JWKS from self.jwks_uri via self.http_client
        //   2. Match the key by `kid` from the JWT header
        //   3. Verify the ES256 signature over "<header>.<payload>" with the matching EC key
        let _ = &self.http_client;
        self.parse_issuer_jwt_claims(jwt)
    }
}

#[async_trait]
impl VpVerifier for VpVerifierService {
    async fn build_vp_request(
        &self,
        deposit_id: &str,
        response_uri: &str,
    ) -> Result<(String, VpRequest), VpError> {
        let nonce = uuid::Uuid::new_v4().to_string();
        let state = deposit_id.to_string();
        let req = build_vp_request_for_deposit(&self.client_id, response_uri, &nonce, &state);
        Ok((nonce, req))
    }

    async fn verify_vp_token(
        &self,
        vp_token: &str,
        expected_nonce: &str,
        response_uri: &str,
    ) -> Result<VerifiedClaims, VpError> {
        // TODO: async signature verification — restructure to pass IssuerClaims directly
        // once fetch_and_verify_issuer_jwt can be called from async context cleanly.
        verify_sd_jwt_vp(vp_token, expected_nonce, response_uri, &|jwt| {
            self.parse_issuer_jwt_claims(jwt)
        })
    }
}
