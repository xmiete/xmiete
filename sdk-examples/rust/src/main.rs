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

//! Usage examples for the XMiete Rust SDK.
//! Each function demonstrates one integration pattern in isolation.

use std::sync::Arc;

use xmiete_sdk::auth::{AuthError, OidcTokenValidator};
use xmiete_sdk::eid::service::EidVerificationService;
use xmiete_sdk::eid::webhook::WebhookHandler;
use xmiete_sdk::eid::{EidVerifier, VerificationRequest};
use xmiete_sdk::openid4vp::service::VpVerifierService;
use xmiete_sdk::openid4vp::{VpError, VpVerifier};

// ── Authentication ────────────────────────────────────────────────────────────

// docs:start:auth-rust
fn validate_bearer_token(header: &str) -> Result<(), AuthError> {
    let validator = OidcTokenValidator::new(
        "https://auth.xmiete.example/.well-known/openid-configuration",
    );
    let claims = validator.validate_token(header, &["deposit:create"])?;
    println!("authenticated: sub={} expires={}", claims.subject, claims.expires_at);
    Ok(())
}
// docs:end:auth-rust

// ── eID Verification ──────────────────────────────────────────────────────────

// docs:start:eid-rust
async fn initiate_eid_verification(deposit_id: &str, tenant_email: &str) -> String {
    let service = EidVerificationService::new(
        "https://eid-provider.example.com", // e.g., Authada, SkIDentity
        "https://api.xmiete.org/v1",
    );
    let session = service
        .initiate_verification(&VerificationRequest {
            deposit_id:   deposit_id.to_string(),
            tenant_email: tenant_email.to_string(),
            redirect_uri: "https://yourapp.example.com/eid-callback".to_string(),
            client_id:    "xmiete-fintech-client".to_string(),
        })
        .await
        .expect("initiate eID session");
    // Redirect the tenant's browser to session.authorization_url.
    // The provider POSTs the result to your /webhook/eid endpoint.
    session.authorization_url
}
// docs:end:eid-rust

// ── Webhook Handling ──────────────────────────────────────────────────────────

// docs:start:webhook-rust
fn build_webhook_handler(
    service: Arc<dyn EidVerifier>,
    bearer_token: &str,
) -> WebhookHandler {
    WebhookHandler::new(
        service,
        bearer_token.to_string(),
        Some(Box::new(|event| {
            println!("eID done: deposit={} status={:?}", event.deposit_id, event.status);
        })),
    )
}
// In your Axum / Actix handler:
//   let sig = headers["X-Signature"].to_str().unwrap_or("");
//   handler.handle_webhook(&body, sig, &webhook_secret).await?;
// docs:end:webhook-rust

// ── OpenID4VP ─────────────────────────────────────────────────────────────────

// docs:start:openid4vp-rust
async fn request_wallet_presentation(
    deposit_id: &str,
    response_uri: &str,
) -> Result<(), VpError> {
    let verifier = VpVerifierService::new(
        "https://verifier.yourapp.example.com".to_string(),
        "https://auth.example.com/.well-known/jwks.json".to_string(),
    );

    // Step 1 — build the VP request; deliver it to the wallet via QR or deep-link.
    let (nonce, _vp_request) = verifier.build_vp_request(deposit_id, response_uri).await?;
    // Persist nonce in your session store — required for step 3.
    // Serialize _vp_request as JSON and embed in QR code or deep-link.

    // Step 3 — wallet POSTs vp_token to response_uri; verify it here.
    let claims = verifier
        .verify_vp_token("<vp_token from wallet>", &nonce, response_uri)
        .await?;
    println!(
        "verified: deposit={} pledged={} bank={}",
        claims.deposit_id, claims.pledge_date, claims.issuing_bank
    );
    Ok(())
}
// docs:end:openid4vp-rust

// ── Error Handling ────────────────────────────────────────────────────────────

// docs:start:error-rust
fn http_status_for_auth_error(err: &AuthError) -> u16 {
    match err {
        AuthError::TokenExpired | AuthError::MalformedJwt(_) => 401,
        AuthError::InsufficientScope { .. }                   => 403,
        AuthError::UnexpectedIssuer(_)                        => 401,
        _                                                     => 500,
    }
}

fn log_vp_error(err: &VpError) {
    match err {
        VpError::NonceMismatch  => eprintln!("replay attack: nonce mismatch"),
        VpError::StaleKbJwt     => eprintln!("presentation expired (>5 min old)"),
        VpError::AudMismatch    => eprintln!("response_uri mismatch"),
        other                   => eprintln!("VP verification failed: {other}"),
    }
}
// docs:end:error-rust

// ─────────────────────────────────────────────────────────────────────────────

struct MockClient;

#[async_trait::async_trait]
impl xmiete_sdk::XMieteClient for MockClient {
    async fn create_deposit(&self, d: xmiete_sdk::models::Deposit) -> Result<xmiete_sdk::models::Deposit, String> {
        Ok(d)
    }
    async fn get_deposit(&self, _id: &str) -> Result<xmiete_sdk::models::Deposit, String> {
        Err("not implemented".into())
    }
    async fn confirm_pledge(&self, _id: &str, _date: &str) -> Result<(), String> {
        Ok(())
    }
    async fn release_deposit(&self, _id: &str, _tok: &str) -> Result<(), String> {
        Ok(())
    }
}

#[tokio::main]
async fn main() {
    println!("XMiete Rust SDK — see function examples above for integration patterns.");

    let client = MockClient;
    client.confirm_pledge("DEP-456", "2026-05-07").await.ok();
}
