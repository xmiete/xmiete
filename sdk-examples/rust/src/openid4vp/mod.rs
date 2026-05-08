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

pub mod service;

use async_trait::async_trait;
use base64::engine::general_purpose::URL_SAFE_NO_PAD;
use base64::Engine;
use chrono::{DateTime, Utc};
use serde::{Deserialize, Serialize};
use sha2::{Digest, Sha256};
use std::collections::{HashMap, HashSet};

#[derive(Debug, thiserror::Error)]
pub enum VpError {
    #[error("invalid format: {0}")]
    InvalidFormat(String),
    #[error("disclosure mismatch: {0}")]
    DisclosureMismatch(String),
    #[error("nonce mismatch")]
    NonceMismatch,
    #[error("aud mismatch")]
    AudMismatch,
    #[error("sd_hash mismatch")]
    SDHashMismatch,
    #[error("kb-jwt is stale or from the future")]
    StaleKbJwt,
    #[error("unexpected vct: {0}")]
    UnexpectedVct(String),
    #[error("json parse error: {0}")]
    JsonParse(String),
    #[error("base64 decode error: {0}")]
    Base64Decode(String),
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct VpRequest {
    pub client_id: String,
    pub response_type: String,
    pub response_mode: String,
    pub response_uri: String,
    pub nonce: String,
    pub state: String,
    pub presentation_definition: PresentationDefinition,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct PresentationDefinition {
    pub id: String,
    pub input_descriptors: Vec<InputDescriptor>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct InputDescriptor {
    pub id: String,
    pub format: HashMap<String, FormatAlgs>,
    pub constraints: Constraints,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct FormatAlgs {
    pub alg: Vec<String>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Constraints {
    pub fields: Vec<Field>,
    pub limit_disclosure: String,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Field {
    pub path: Vec<String>,
    pub filter: Option<FieldFilter>,
    pub optional: bool,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct FieldFilter {
    #[serde(rename = "type")]
    pub type_: String,
    #[serde(rename = "const")]
    pub const_: Option<String>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct VerifiedClaims {
    pub credential_id: String,
    pub deposit_id: String,
    pub pledge_date: String,
    pub statutory_basis: String,
    pub issuing_bank: String,
    pub deposit_amount: Option<f64>,
    pub currency: Option<String>,
    pub property_address: Option<String>,
    pub tenant_first_name: Option<String>,
    pub tenant_last_name: Option<String>,
    pub pledged_until: Option<String>,
    pub verified_at: DateTime<Utc>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct IssuerClaims {
    pub credential_id: String,
    pub deposit_id: String,
    pub pledge_date: String,
    pub statutory_basis: String,
    pub issuing_bank: String,
    pub issuing_bank_id: Option<String>,
    pub property_id: Option<String>,
    pub sd_hashes: Vec<String>,
    pub sd_alg: String,
    pub exp: i64,
}

#[async_trait]
pub trait VpVerifier: Send + Sync {
    async fn build_vp_request(
        &self,
        deposit_id: &str,
        response_uri: &str,
    ) -> Result<(String, VpRequest), VpError>;

    async fn verify_vp_token(
        &self,
        vp_token: &str,
        expected_nonce: &str,
        response_uri: &str,
    ) -> Result<VerifiedClaims, VpError>;
}

pub fn build_vp_request_for_deposit(
    client_id: &str,
    response_uri: &str,
    nonce: &str,
    state: &str,
) -> VpRequest {
    let mut format = HashMap::new();
    format.insert(
        "vc+sd-jwt".to_string(),
        FormatAlgs {
            alg: vec!["ES256".to_string()],
        },
    );

    let fields = vec![
        Field {
            path: vec!["$.vct".to_string()],
            filter: Some(FieldFilter {
                type_: "string".to_string(),
                const_: Some("DepositPledgeAttestation".to_string()),
            }),
            optional: false,
        },
        Field {
            path: vec!["$.deposit_id".to_string()],
            filter: None,
            optional: false,
        },
        Field {
            path: vec!["$.pledge_date".to_string()],
            filter: None,
            optional: false,
        },
        Field {
            path: vec!["$.statutory_basis".to_string()],
            filter: None,
            optional: false,
        },
        Field {
            path: vec!["$.issuing_bank".to_string()],
            filter: None,
            optional: false,
        },
        Field {
            path: vec!["$.deposit_amount".to_string()],
            filter: None,
            optional: true,
        },
        Field {
            path: vec!["$.currency".to_string()],
            filter: None,
            optional: true,
        },
        Field {
            path: vec!["$.property_address".to_string()],
            filter: None,
            optional: true,
        },
        Field {
            path: vec!["$.tenant_first_name".to_string()],
            filter: None,
            optional: true,
        },
        Field {
            path: vec!["$.tenant_last_name".to_string()],
            filter: None,
            optional: true,
        },
        Field {
            path: vec!["$.pledged_until".to_string()],
            filter: None,
            optional: true,
        },
    ];

    VpRequest {
        client_id: client_id.to_string(),
        response_type: "vp_token".to_string(),
        response_mode: "direct_post".to_string(),
        response_uri: response_uri.to_string(),
        nonce: nonce.to_string(),
        state: state.to_string(),
        presentation_definition: PresentationDefinition {
            id: "deposit-pledge-attestation-pd".to_string(),
            input_descriptors: vec![InputDescriptor {
                id: "deposit-pledge-attestation".to_string(),
                format,
                constraints: Constraints {
                    fields,
                    limit_disclosure: "required".to_string(),
                },
            }],
        },
    }
}

pub fn verify_sd_jwt_vp(
    vp_token: &str,
    expected_nonce: &str,
    response_uri: &str,
    verify_issuer_sig: &dyn Fn(&str) -> Result<IssuerClaims, VpError>,
) -> Result<VerifiedClaims, VpError> {
    let parts: Vec<&str> = vp_token.split('~').collect();
    if parts.is_empty() {
        return Err(VpError::InvalidFormat("empty vp_token".to_string()));
    }

    let issuer_jwt = parts[0];
    if issuer_jwt.is_empty() {
        return Err(VpError::InvalidFormat("missing issuer JWT".to_string()));
    }

    let last = *parts.last().unwrap();
    let kb_jwt = if !last.is_empty() && last != issuer_jwt {
        Some(last)
    } else {
        None
    };

    // Middle non-empty parts between issuer_jwt and the optional kb-jwt are disclosures.
    let disclosure_end = if kb_jwt.is_some() {
        parts.len() - 1
    } else {
        parts.len()
    };
    let disclosures: Vec<&str> = parts[1..disclosure_end]
        .iter()
        .copied()
        .filter(|p| !p.is_empty())
        .collect();

    let issuer_claims = verify_issuer_sig(issuer_jwt)?;

    let sd_hash_set: HashSet<&str> = issuer_claims.sd_hashes.iter().map(String::as_str).collect();

    let mut disclosed: HashMap<String, serde_json::Value> = HashMap::new();
    for &enc in &disclosures {
        let digest = {
            let mut hasher = Sha256::new();
            hasher.update(enc.as_bytes());
            URL_SAFE_NO_PAD.encode(hasher.finalize())
        };
        if !sd_hash_set.contains(digest.as_str()) {
            return Err(VpError::DisclosureMismatch(format!(
                "disclosure digest {} not found in _sd",
                digest
            )));
        }
        let raw = URL_SAFE_NO_PAD
            .decode(enc)
            .map_err(|e| VpError::Base64Decode(e.to_string()))?;
        let arr: serde_json::Value = serde_json::from_slice(&raw)
            .map_err(|e| VpError::JsonParse(e.to_string()))?;
        let arr = arr
            .as_array()
            .ok_or_else(|| VpError::InvalidFormat("disclosure is not a JSON array".to_string()))?;
        if arr.len() < 3 {
            return Err(VpError::InvalidFormat(
                "disclosure array must have at least 3 elements".to_string(),
            ));
        }
        let name = arr[1]
            .as_str()
            .ok_or_else(|| VpError::InvalidFormat("disclosure name is not a string".to_string()))?
            .to_string();
        disclosed.insert(name, arr[2].clone());
    }

    if let Some(kb) = kb_jwt {
        let kb_parts: Vec<&str> = kb.splitn(3, '.').collect();
        if kb_parts.len() < 2 {
            return Err(VpError::InvalidFormat("kb-jwt has fewer than 2 parts".to_string()));
        }

        let header_raw = URL_SAFE_NO_PAD
            .decode(kb_parts[0])
            .map_err(|e| VpError::Base64Decode(e.to_string()))?;
        let header: serde_json::Value = serde_json::from_slice(&header_raw)
            .map_err(|e| VpError::JsonParse(e.to_string()))?;
        let typ = header
            .get("typ")
            .and_then(|v| v.as_str())
            .unwrap_or("");
        if typ != "kb+jwt" {
            return Err(VpError::InvalidFormat(format!(
                "kb-jwt typ must be kb+jwt, got {}",
                typ
            )));
        }

        let payload_raw = URL_SAFE_NO_PAD
            .decode(kb_parts[1])
            .map_err(|e| VpError::Base64Decode(e.to_string()))?;
        let payload: serde_json::Value = serde_json::from_slice(&payload_raw)
            .map_err(|e| VpError::JsonParse(e.to_string()))?;

        let kb_nonce = payload
            .get("nonce")
            .and_then(|v| v.as_str())
            .unwrap_or("");
        if kb_nonce != expected_nonce {
            return Err(VpError::NonceMismatch);
        }

        let kb_aud = payload
            .get("aud")
            .and_then(|v| v.as_str())
            .unwrap_or("");
        if kb_aud != response_uri {
            return Err(VpError::AudMismatch);
        }

        // Reconstruct the exact string that was hashed: issuer_jwt~disc1~...~discN~
        let mut sd_input = issuer_jwt.to_string();
        for d in &disclosures {
            sd_input.push('~');
            sd_input.push_str(d);
        }
        sd_input.push('~');

        let expected_sd_hash = {
            let mut hasher = Sha256::new();
            hasher.update(sd_input.as_bytes());
            URL_SAFE_NO_PAD.encode(hasher.finalize())
        };
        let kb_sd_hash = payload
            .get("sd_hash")
            .and_then(|v| v.as_str())
            .unwrap_or("");
        if kb_sd_hash != expected_sd_hash {
            return Err(VpError::SDHashMismatch);
        }

        let iat = payload
            .get("iat")
            .and_then(|v| v.as_i64())
            .unwrap_or(0);
        let now = Utc::now().timestamp();
        // Allow up to 5 minutes of clock skew / presentation age.
        if iat > now + 30 || iat < now - 300 {
            return Err(VpError::StaleKbJwt);
        }
    }

    let get_str = |m: &HashMap<String, serde_json::Value>, k: &str| -> Option<String> {
        m.get(k).and_then(|v| v.as_str()).map(|s| s.to_string())
    };
    let get_f64 = |m: &HashMap<String, serde_json::Value>, k: &str| -> Option<f64> {
        m.get(k).and_then(|v| v.as_f64())
    };

    Ok(VerifiedClaims {
        credential_id: issuer_claims.credential_id,
        deposit_id: issuer_claims.deposit_id,
        pledge_date: issuer_claims.pledge_date,
        statutory_basis: issuer_claims.statutory_basis,
        issuing_bank: issuer_claims.issuing_bank,
        deposit_amount: get_f64(&disclosed, "deposit_amount"),
        currency: get_str(&disclosed, "currency"),
        property_address: get_str(&disclosed, "property_address"),
        tenant_first_name: get_str(&disclosed, "tenant_first_name"),
        tenant_last_name: get_str(&disclosed, "tenant_last_name"),
        pledged_until: get_str(&disclosed, "pledged_until"),
        verified_at: Utc::now(),
    })
}
