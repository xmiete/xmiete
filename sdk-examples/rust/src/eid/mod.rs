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
pub mod webhook;

use chrono::{DateTime, Utc};
use serde::{Deserialize, Serialize};

#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "SCREAMING_SNAKE_CASE")]
pub enum EidStatus {
    NotStarted,
    Pending,
    Verified,
    Failed,
    Expired,
}

/// Input to start a new eID verification session for a tenant.
#[derive(Debug, Clone)]
pub struct VerificationRequest {
    pub deposit_id: String,
    pub tenant_email: String,
    pub redirect_uri: String,
    pub client_id: String,
}

/// Response from the eID provider after session creation.
/// Redirect the tenant's browser to `authorization_url` within the validity window.
#[derive(Debug, Clone)]
pub struct VerificationSession {
    pub session_id: String,
    pub authorization_url: String,
    pub expires_at: DateTime<Utc>,
}

/// Payload for PATCH /deposits/{id}/identity — only ProviderReference is stored, never raw PII.
#[derive(Debug, Clone)]
pub struct KycUpdatePayload {
    pub eid_status: EidStatus,
    pub verification_timestamp: DateTime<Utc>,
    pub provider_reference: String,
}

/// Incoming webhook event from the eID provider after the user completes verification.
#[derive(Debug, Clone, Deserialize)]
pub struct WebhookEvent {
    pub session_id: Option<String>,
    pub deposit_id: String,
    pub status: EidStatus,
    pub provider_reference: Option<String>,
    pub completed_at: Option<DateTime<Utc>>,
    pub error_code: Option<String>,
}
