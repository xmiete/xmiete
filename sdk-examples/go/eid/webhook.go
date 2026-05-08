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

// WebhookHandler processes signed webhook events from the eID provider.
//
// Mount on your HTTP server at the endpoint registered with the provider:
//
//	http.HandleFunc("/webhook/eid", func(w http.ResponseWriter, r *http.Request) {
//	    body, _ := io.ReadAll(r.Body)
//	    if err := handler.HandleWebhook(body, r.Header.Get("X-Signature"), webhookSecret); err != nil {
//	        http.Error(w, err.Error(), http.StatusBadRequest)
//	        return
//	    }
//	    w.WriteHeader(http.StatusOK)
//	})

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"time"
)

var (
	ErrMissingSignature  = errors.New("eid: missing X-Signature header")
	ErrSignatureMismatch = errors.New("eid: HMAC signature mismatch")
	ErrMissingFields     = errors.New("eid: webhook missing required fields: deposit_id, status")
	ErrUnknownStatus     = errors.New("eid: unknown status value")
)

type WebhookHandler struct {
	service     IdentityVerifier
	bearerToken string
	onComplete  func(WebhookEvent)
}

// NewWebhookHandler creates a handler that validates signatures, then calls service.UpdateDepositKYCStatus
// and optionally invokes onComplete (may be nil) once verification is dispatched.
func NewWebhookHandler(service IdentityVerifier, bearerToken string, onComplete func(WebhookEvent)) *WebhookHandler {
	return &WebhookHandler{
		service:     service,
		bearerToken: bearerToken,
		onComplete:  onComplete,
	}
}

// HandleWebhook validates the HMAC signature, parses the event, and dispatches the result.
// rawBody must be the unmodified request body; signature is the X-Signature header value.
func (h *WebhookHandler) HandleWebhook(rawBody []byte, signature, webhookSecret string) error {
	if err := verifyHMAC(rawBody, signature, webhookSecret); err != nil {
		return err
	}
	event, err := parseWebhookEvent(rawBody)
	if err != nil {
		return err
	}
	h.dispatch(event)
	return nil
}

func (h *WebhookHandler) dispatch(event WebhookEvent) {
	switch event.Status {
	case EIDStatusVerified:
		payload := KYCUpdatePayload{
			EIDStatus:             EIDStatusVerified,
			VerificationTimestamp: event.CompletedAt,
			ProviderReference:     event.ProviderReference,
		}
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			if err := h.service.UpdateDepositKYCStatus(ctx, event.DepositID, payload, h.bearerToken); err != nil {
				log.Printf("eid: kyc update failed for deposit %s: %v", event.DepositID, err)
			}
			if h.onComplete != nil {
				h.onComplete(event)
			}
		}()

	case EIDStatusFailed, EIDStatusExpired:
		log.Printf("eid: verification %s for deposit %s (error: %q)", event.Status, event.DepositID, event.ErrorCode)
		if h.onComplete != nil {
			h.onComplete(event)
		}
	}
}

func verifyHMAC(body []byte, signature, secret string) error {
	if signature == "" {
		return ErrMissingSignature
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	expected := hex.EncodeToString(mac.Sum(nil))
	if subtle.ConstantTimeCompare([]byte(expected), []byte(signature)) != 1 {
		return ErrSignatureMismatch
	}
	return nil
}

func parseWebhookEvent(data []byte) (WebhookEvent, error) {
	var raw struct {
		SessionID         string `json:"session_id"`
		DepositID         string `json:"deposit_id"`
		Status            string `json:"status"`
		ProviderReference string `json:"provider_reference"`
		CompletedAt       string `json:"completed_at"`
		ErrorCode         string `json:"error_code"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return WebhookEvent{}, fmt.Errorf("eid: parse webhook body: %w", err)
	}
	if raw.DepositID == "" || raw.Status == "" {
		return WebhookEvent{}, ErrMissingFields
	}

	status := EIDStatus(raw.Status)
	switch status {
	case EIDStatusNotStarted, EIDStatusPending, EIDStatusVerified, EIDStatusFailed, EIDStatusExpired:
	default:
		return WebhookEvent{}, fmt.Errorf("%w: %q", ErrUnknownStatus, raw.Status)
	}

	completedAt := time.Now().UTC()
	if raw.CompletedAt != "" {
		if t, err := time.Parse(time.RFC3339, raw.CompletedAt); err == nil {
			completedAt = t
		}
	}

	return WebhookEvent{
		SessionID:         raw.SessionID,
		DepositID:         raw.DepositID,
		Status:            status,
		ProviderReference: raw.ProviderReference,
		CompletedAt:       completedAt,
		ErrorCode:         raw.ErrorCode,
	}, nil
}
