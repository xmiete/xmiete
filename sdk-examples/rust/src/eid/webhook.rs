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

//! WebhookHandler processes signed webhook events from the eID provider.
//!
//! Example (Axum):
//! ```ignore
//! async fn eid_callback(
//!     headers: HeaderMap,
//!     body: Bytes,
//! ) -> impl IntoResponse {
//!     let sig = headers.get("X-Signature").and_then(|v| v.to_str().ok()).unwrap_or("");
//!     match handler.handle_webhook(&body, sig, &webhook_secret).await {
//!         Ok(()) => StatusCode::OK,
//!         Err(e) => { tracing::warn!("bad webhook: {e}"); StatusCode::BAD_REQUEST }
//!     }
//! }
//! ```

use std::sync::Arc;

use chrono::Utc;
use hmac::{Hmac, Mac};
use sha2::Sha256;

use super::{EidVerifier, EidStatus, KycUpdatePayload, WebhookEvent};

type HmacSha256 = Hmac<Sha256>;

#[derive(Debug, thiserror::Error)]
pub enum WebhookError {
    #[error("missing X-Signature header")]
    MissingSignature,
    #[error("HMAC signature mismatch")]
    SignatureMismatch,
    #[error("invalid signature encoding")]
    InvalidSignatureEncoding,
    #[error("failed to parse webhook body: {0}")]
    ParseError(#[from] serde_json::Error),
    #[error("webhook missing required fields")]
    MissingFields,
}

pub struct WebhookHandler {
    service: Arc<dyn EidVerifier>,
    bearer_token: String,
    on_complete: Option<Box<dyn Fn(WebhookEvent) + Send + Sync>>,
}

impl WebhookHandler {
    pub fn new(
        service: Arc<dyn EidVerifier>,
        bearer_token: impl Into<String>,
        on_complete: Option<Box<dyn Fn(WebhookEvent) + Send + Sync>>,
    ) -> Self {
        Self {
            service,
            bearer_token: bearer_token.into(),
            on_complete,
        }
    }

    /// Validates the HMAC signature, parses the event, and dispatches the result.
    /// `raw_body` must be the unmodified request body bytes; `signature` is the X-Signature value.
    pub async fn handle_webhook(
        &self,
        raw_body: &[u8],
        signature: &str,
        webhook_secret: &str,
    ) -> Result<(), WebhookError> {
        verify_hmac(raw_body, signature, webhook_secret)?;
        let event: WebhookEvent = serde_json::from_slice(raw_body)?;
        if event.deposit_id.is_empty() {
            return Err(WebhookError::MissingFields);
        }
        self.dispatch(event).await;
        Ok(())
    }

    async fn dispatch(&self, event: WebhookEvent) {
        match event.status {
            EidStatus::Verified => {
                let payload = KycUpdatePayload {
                    eid_status: EidStatus::Verified,
                    verification_timestamp: event.completed_at.unwrap_or_else(Utc::now),
                    provider_reference: event.provider_reference.clone().unwrap_or_default(),
                };
                let service = Arc::clone(&self.service);
                let token = self.bearer_token.clone();
                let deposit_id = event.deposit_id.clone();
                let on_complete = self.on_complete.as_ref().map(|_| event.clone());

                tokio::spawn(async move {
                    if let Err(e) = service
                        .update_deposit_kyc_status(&deposit_id, &payload, &token)
                        .await
                    {
                        eprintln!("eid: kyc update failed for deposit {deposit_id}: {e}");
                    }
                });

                if let (Some(cb), Some(ev)) = (&self.on_complete, on_complete) {
                    cb(ev);
                }
            }
            EidStatus::Failed | EidStatus::Expired => {
                eprintln!(
                    "eid: verification {:?} for deposit {} (error: {:?})",
                    event.status, event.deposit_id, event.error_code
                );
                if let Some(cb) = &self.on_complete {
                    cb(event);
                }
            }
            _ => {}
        }
    }
}

fn verify_hmac(body: &[u8], signature: &str, secret: &str) -> Result<(), WebhookError> {
    if signature.is_empty() {
        return Err(WebhookError::MissingSignature);
    }
    let expected = hex::decode(signature).map_err(|_| WebhookError::InvalidSignatureEncoding)?;
    let mut mac =
        HmacSha256::new_from_slice(secret.as_bytes()).expect("HMAC accepts any key length");
    mac.update(body);
    mac.verify_slice(&expected)
        .map_err(|_| WebhookError::SignatureMismatch)
}

#[cfg(test)]
mod tests {
    use super::{WebhookError, WebhookHandler};
    use super::super::{EidError, EidVerifier, KycUpdatePayload, VerificationRequest, VerificationSession};
    use async_trait::async_trait;
    use hmac::{Hmac, Mac};
    use sha2::Sha256;
    use std::sync::{
        atomic::{AtomicU32, Ordering},
        Arc,
    };

    struct MockVerifier {
        kyc_called: Arc<AtomicU32>,
    }

    #[async_trait]
    impl EidVerifier for MockVerifier {
        async fn initiate_verification(&self, _req: &VerificationRequest) -> Result<VerificationSession, EidError> {
            unimplemented!("not exercised by webhook tests")
        }

        async fn update_deposit_kyc_status(&self, _deposit_id: &str, _payload: &KycUpdatePayload, _bearer_token: &str) -> Result<(), EidError> {
            self.kyc_called.fetch_add(1, Ordering::SeqCst);
            Ok(())
        }
    }

    fn mock_verifier() -> (Arc<MockVerifier>, Arc<AtomicU32>) {
        let counter = Arc::new(AtomicU32::new(0));
        let v = Arc::new(MockVerifier { kyc_called: counter.clone() });
        (v, counter)
    }

    fn sign(body: &[u8], secret: &str) -> String {
        let mut mac = Hmac::<Sha256>::new_from_slice(secret.as_bytes()).unwrap();
        mac.update(body);
        hex::encode(mac.finalize().into_bytes())
    }

    #[tokio::test]
    async fn test_valid_verified_webhook_calls_kyc() {
        let (verifier, counter) = mock_verifier();
        let handler = WebhookHandler::new(verifier, "tok", None);

        let body = serde_json::json!({
            "deposit_id": "DEP-1",
            "status": "VERIFIED",
            "provider_reference": "ref-abc",
        })
        .to_string();
        let sig = sign(body.as_bytes(), "secret");

        handler.handle_webhook(body.as_bytes(), &sig, "secret").await.unwrap();
        tokio::time::sleep(std::time::Duration::from_millis(50)).await;
        assert_eq!(counter.load(Ordering::SeqCst), 1, "update_deposit_kyc_status should be called once");
    }

    #[tokio::test]
    async fn test_failed_status_does_not_call_kyc() {
        let (verifier, counter) = mock_verifier();
        let handler = WebhookHandler::new(verifier, "tok", None);

        let body = serde_json::json!({"deposit_id": "DEP-2", "status": "FAILED"}).to_string();
        let sig = sign(body.as_bytes(), "s");

        handler.handle_webhook(body.as_bytes(), &sig, "s").await.unwrap();
        tokio::time::sleep(std::time::Duration::from_millis(50)).await;
        assert_eq!(counter.load(Ordering::SeqCst), 0, "update_deposit_kyc_status must not be called for FAILED");
    }

    #[tokio::test]
    async fn test_missing_signature_returns_error() {
        let (verifier, _) = mock_verifier();
        let handler = WebhookHandler::new(verifier, "tok", None);
        let body = b"{\"deposit_id\":\"DEP-1\",\"status\":\"VERIFIED\"}";
        let err = handler.handle_webhook(body, "", "secret").await.unwrap_err();
        assert!(matches!(err, WebhookError::MissingSignature));
    }

    #[tokio::test]
    async fn test_signature_mismatch_returns_error() {
        let (verifier, _) = mock_verifier();
        let handler = WebhookHandler::new(verifier, "tok", None);
        let body = b"{\"deposit_id\":\"DEP-1\",\"status\":\"VERIFIED\"}";
        // Correct length hex but wrong value.
        let bad_sig = "a".repeat(64);
        let err = handler.handle_webhook(body, &bad_sig, "secret").await.unwrap_err();
        assert!(matches!(err, WebhookError::SignatureMismatch));
    }

    #[tokio::test]
    async fn test_missing_deposit_id_returns_error() {
        let (verifier, _) = mock_verifier();
        let handler = WebhookHandler::new(verifier, "tok", None);
        let body = serde_json::json!({"status": "VERIFIED"}).to_string();
        let sig = sign(body.as_bytes(), "s");
        let err = handler.handle_webhook(body.as_bytes(), &sig, "s").await.unwrap_err();
        assert!(matches!(err, WebhookError::MissingFields));
    }

    #[tokio::test]
    async fn test_custom_adapter_wires_into_webhook_handler() {
        // Verifies provider agnosticism: any EidVerifier implementation
        // plugs into WebhookHandler without touching any other SDK code.
        struct CustomAdapter { inner: MockVerifier }

        #[async_trait]
        impl EidVerifier for CustomAdapter {
            async fn initiate_verification(&self, req: &VerificationRequest) -> Result<VerificationSession, EidError> {
                self.inner.initiate_verification(req).await
            }
            async fn update_deposit_kyc_status(&self, d: &str, p: &KycUpdatePayload, t: &str) -> Result<(), EidError> {
                self.inner.update_deposit_kyc_status(d, p, t).await
            }
        }

        let counter = Arc::new(AtomicU32::new(0));
        let adapter = Arc::new(CustomAdapter {
            inner: MockVerifier { kyc_called: counter.clone() },
        });
        let handler = WebhookHandler::new(adapter, "tok", None);

        let body = serde_json::json!({"deposit_id": "DEP-9", "status": "VERIFIED"}).to_string();
        let sig = sign(body.as_bytes(), "s");
        handler.handle_webhook(body.as_bytes(), &sig, "s").await.unwrap();
        tokio::time::sleep(std::time::Duration::from_millis(50)).await;
        assert_eq!(counter.load(Ordering::SeqCst), 1);
    }
}
