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

//! EidVerificationService manages eID sessions against a BSI TR-03130 compatible provider
//! (e.g., AusweisApp2 SDK, Authada, SkIDentity).
//!
//! Flow:
//!  1. Call `initiate_verification` → receive `VerificationSession` with `authorization_url`
//!  2. Redirect the tenant's browser to `authorization_url`
//!  3. The provider POSTs to your webhook → handled by `WebhookHandler`
//!  4. `WebhookHandler` calls `update_deposit_kyc_status` to finalize the deposit state

use chrono::Utc;
use reqwest::Client;
use serde::Deserialize;

use super::{KycUpdatePayload, VerificationRequest, VerificationSession};

#[derive(Debug, thiserror::Error)]
pub enum EidError {
    #[error("http error: {0}")]
    Http(#[from] reqwest::Error),
    #[error("eid provider returned HTTP {0}")]
    ProviderError(u16),
    #[error("kyc update failed: HTTP {0}")]
    KycUpdateFailed(u16),
}

pub struct EidVerificationService {
    client: Client,
    eid_provider_base_url: String,
    xmiete_api_base_url: String,
}

impl EidVerificationService {
    pub fn new(eid_provider_base_url: impl Into<String>, xmiete_api_base_url: impl Into<String>) -> Self {
        Self {
            client: Client::new(),
            eid_provider_base_url: eid_provider_base_url.into(),
            xmiete_api_base_url: xmiete_api_base_url.into(),
        }
    }

    /// Creates an eID verification session. Redirect the tenant to the returned
    /// `authorization_url` within the 15-minute validity window.
    pub async fn initiate_verification(
        &self,
        req: &VerificationRequest,
    ) -> Result<VerificationSession, EidError> {
        #[derive(serde::Serialize)]
        struct SessionRequest<'a> {
            client_id: &'a str,
            deposit_id: &'a str,
            tenant_email: &'a str,
            redirect_uri: &'a str,
            scope: &'static str,
        }

        #[derive(Deserialize, Default)]
        struct SessionResponse {
            session_id: Option<String>,
            authorization_url: Option<String>,
        }

        let response = self
            .client
            .post(format!("{}/sessions", self.eid_provider_base_url))
            .json(&SessionRequest {
                client_id: &req.client_id,
                deposit_id: &req.deposit_id,
                tenant_email: &req.tenant_email,
                redirect_uri: &req.redirect_uri,
                scope: "openid eid",
            })
            .send()
            .await?;

        if response.status().as_u16() != 201 {
            return Err(EidError::ProviderError(response.status().as_u16()));
        }

        let body: SessionResponse = response.json().await.unwrap_or_default();
        let session_id = body
            .session_id
            .unwrap_or_else(|| format!("{}-session", req.deposit_id));
        let authorization_url = body.authorization_url.unwrap_or_else(|| {
            format!("{}/authorize?deposit_id={}", self.eid_provider_base_url, req.deposit_id)
        });

        Ok(VerificationSession {
            session_id,
            authorization_url,
            expires_at: Utc::now() + chrono::Duration::minutes(15),
        })
    }

    /// Pushes the verified eID result to the XMiete API (PATCH /deposits/{id}/identity).
    /// Only `provider_reference` is forwarded — never raw PII from the eID chip.
    pub async fn update_deposit_kyc_status(
        &self,
        deposit_id: &str,
        payload: &KycUpdatePayload,
        bearer_token: &str,
    ) -> Result<(), EidError> {
        #[derive(serde::Serialize)]
        struct KycRequest<'a> {
            eid_status: &'a super::EidStatus,
            verification_timestamp: String,
            provider_reference: &'a str,
        }

        let response = self
            .client
            .patch(format!(
                "{}/deposits/{}/identity",
                self.xmiete_api_base_url, deposit_id
            ))
            .bearer_auth(bearer_token)
            .json(&KycRequest {
                eid_status: &payload.eid_status,
                verification_timestamp: payload.verification_timestamp.to_rfc3339(),
                provider_reference: &payload.provider_reference,
            })
            .send()
            .await?;

        let status = response.status().as_u16();
        if status != 200 && status != 204 {
            return Err(EidError::KycUpdateFailed(status));
        }
        Ok(())
    }
}
