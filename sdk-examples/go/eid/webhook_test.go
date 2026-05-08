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

package eid

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sync/atomic"
	"testing"
	"time"
)

// mockVerifier is a test double for IdentityVerifier.
type mockVerifier struct {
	kycCalled int32
	kycErr    error
}

func (m *mockVerifier) InitiateVerification(_ context.Context, req VerificationRequest) (*VerificationSession, error) {
	return &VerificationSession{
		SessionID:        req.DepositID + "-session",
		AuthorizationURL: "https://example.com/auth",
		ExpiresAt:        time.Now().Add(15 * time.Minute),
	}, nil
}

func (m *mockVerifier) UpdateDepositKYCStatus(_ context.Context, _ string, _ KYCUpdatePayload, _ string) error {
	atomic.AddInt32(&m.kycCalled, 1)
	return m.kycErr
}

// Compile-time proof that mockVerifier satisfies the interface.
var _ IdentityVerifier = (*mockVerifier)(nil)

func signWebhookBody(t *testing.T, body []byte, secret string) string {
	t.Helper()
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return hex.EncodeToString(mac.Sum(nil))
}

func verifiedBody(t *testing.T) []byte {
	t.Helper()
	b, _ := json.Marshal(map[string]string{
		"deposit_id":         "DEP-1",
		"status":             "VERIFIED",
		"provider_reference": "ref-abc",
		"completed_at":       time.Now().UTC().Format(time.RFC3339),
	})
	return b
}

func TestHandleWebhook_ValidVerified(t *testing.T) {
	mock := &mockVerifier{}
	done := make(chan struct{})
	handler := NewWebhookHandler(mock, "bearer-tok", func(WebhookEvent) { close(done) })

	secret := "test-secret"
	body := verifiedBody(t)
	sig := signWebhookBody(t, body, secret)

	if err := handler.HandleWebhook(body, sig, secret); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("timeout: onComplete callback never fired")
	}

	if n := atomic.LoadInt32(&mock.kycCalled); n != 1 {
		t.Errorf("UpdateDepositKYCStatus called %d times, want 1", n)
	}
}

func TestHandleWebhook_FailedStatus_DoesNotCallKYC(t *testing.T) {
	mock := &mockVerifier{}
	done := make(chan struct{})
	handler := NewWebhookHandler(mock, "tok", func(WebhookEvent) { close(done) })

	secret := "s"
	body, _ := json.Marshal(map[string]string{"deposit_id": "DEP-2", "status": "FAILED"})
	sig := signWebhookBody(t, body, secret)

	if err := handler.HandleWebhook(body, sig, secret); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("timeout: onComplete callback never fired")
	}

	if n := atomic.LoadInt32(&mock.kycCalled); n != 0 {
		t.Errorf("UpdateDepositKYCStatus should not be called for FAILED status, got %d calls", n)
	}
}

func TestHandleWebhook_MissingSignature(t *testing.T) {
	handler := NewWebhookHandler(&mockVerifier{}, "tok", nil)
	body := verifiedBody(t)
	if err := handler.HandleWebhook(body, "", "secret"); err != ErrMissingSignature {
		t.Fatalf("want ErrMissingSignature, got %v", err)
	}
}

func TestHandleWebhook_SignatureMismatch(t *testing.T) {
	handler := NewWebhookHandler(&mockVerifier{}, "tok", nil)
	body := verifiedBody(t)
	if err := handler.HandleWebhook(body, "deadbeef", "secret"); err != ErrSignatureMismatch {
		t.Fatalf("want ErrSignatureMismatch, got %v", err)
	}
}

func TestHandleWebhook_UnknownStatus(t *testing.T) {
	handler := NewWebhookHandler(&mockVerifier{}, "tok", nil)
	secret := "s"
	body := []byte(`{"deposit_id":"DEP-1","status":"BOGUS"}`)
	sig := signWebhookBody(t, body, secret)
	if err := handler.HandleWebhook(body, sig, secret); err == nil {
		t.Fatal("expected error for unknown status value")
	}
}

func TestHandleWebhook_MissingFields(t *testing.T) {
	handler := NewWebhookHandler(&mockVerifier{}, "tok", nil)
	secret := "s"
	body := []byte(`{"status":"VERIFIED"}`)
	sig := signWebhookBody(t, body, secret)
	if err := handler.HandleWebhook(body, sig, secret); err != ErrMissingFields {
		t.Fatalf("want ErrMissingFields, got %v", err)
	}
}

func TestHandleWebhook_CustomAdapter(t *testing.T) {
	// Verifies that any IdentityVerifier implementation wires into WebhookHandler
	// without modification — the core of provider agnosticism.
	type customAdapter struct{ mockVerifier }
	var adapter customAdapter

	done := make(chan struct{})
	handler := NewWebhookHandler(&adapter, "tok", func(WebhookEvent) { close(done) })

	secret := "s"
	body := verifiedBody(t)
	sig := signWebhookBody(t, body, secret)

	if err := handler.HandleWebhook(body, sig, secret); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("timeout")
	}
}

func TestParseWebhookEvent_AllFields(t *testing.T) {
	body := []byte(`{
		"session_id": "sess-1",
		"deposit_id": "DEP-42",
		"status": "VERIFIED",
		"provider_reference": "ref-xyz",
		"completed_at": "2026-05-01T10:00:00Z",
		"error_code": ""
	}`)
	ev, err := parseWebhookEvent(body)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ev.DepositID != "DEP-42" {
		t.Errorf("DepositID = %q, want DEP-42", ev.DepositID)
	}
	if ev.Status != EIDStatusVerified {
		t.Errorf("Status = %q, want VERIFIED", ev.Status)
	}
	if ev.ProviderReference != "ref-xyz" {
		t.Errorf("ProviderReference = %q, want ref-xyz", ev.ProviderReference)
	}
}

func TestVerifyHMAC_CorrectSignature(t *testing.T) {
	body := []byte("hello")
	mac := hmac.New(sha256.New, []byte("secret"))
	mac.Write(body)
	sig := hex.EncodeToString(mac.Sum(nil))
	if err := verifyHMAC(body, sig, "secret"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestVerifyHMAC_WrongSecret(t *testing.T) {
	body := []byte("hello")
	mac := hmac.New(sha256.New, []byte("correct-secret"))
	mac.Write(body)
	sig := hex.EncodeToString(mac.Sum(nil))
	if err := verifyHMAC(body, sig, "wrong-secret"); err != ErrSignatureMismatch {
		t.Fatalf("want ErrSignatureMismatch, got %v", err)
	}
}
