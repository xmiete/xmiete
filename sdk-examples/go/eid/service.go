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

// EidVerificationService manages eID verification sessions against a BSI TR-03130
// compatible eID provider (e.g., AusweisApp2 SDK, Authada, SkIDentity).
//
// Flow:
//  1. Call InitiateVerification → receive VerificationSession with AuthorizationURL
//  2. Redirect the tenant's browser to AuthorizationURL
//  3. eID provider POSTs to your webhook → handled by WebhookHandler
//  4. WebhookHandler calls UpdateDepositKYCStatus to finalize the deposit state

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

var _ IdentityVerifier = (*VerificationService)(nil)

type VerificationService struct {
	httpClient         *http.Client
	eidProviderBaseURL string
	xmieteAPIBaseURL   string
}

func NewVerificationService(eidProviderBaseURL, xmieteAPIBaseURL string) *VerificationService {
	return &VerificationService{
		httpClient:         &http.Client{Timeout: 10 * time.Second},
		eidProviderBaseURL: eidProviderBaseURL,
		xmieteAPIBaseURL:   xmieteAPIBaseURL,
	}
}

// InitiateVerification creates an eID session and returns the authorization redirect URL.
func (s *VerificationService) InitiateVerification(ctx context.Context, req VerificationRequest) (*VerificationSession, error) {
	body := fmt.Sprintf(
		`{"client_id":%q,"deposit_id":%q,"tenant_email":%q,"redirect_uri":%q,"scope":"openid eid"}`,
		req.ClientID, req.DepositID, req.TenantEmail, req.RedirectURI,
	)

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost,
		s.eidProviderBaseURL+"/sessions", strings.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("eid: build session request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := s.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("eid: provider request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("eid: provider returned HTTP %d", resp.StatusCode)
	}

	var result struct {
		SessionID        string `json:"session_id"`
		AuthorizationURL string `json:"authorization_url"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&result)
	if result.SessionID == "" {
		result.SessionID = req.DepositID + "-session"
	}
	if result.AuthorizationURL == "" {
		result.AuthorizationURL = s.eidProviderBaseURL + "/authorize?deposit_id=" + req.DepositID
	}

	return &VerificationSession{
		SessionID:        result.SessionID,
		AuthorizationURL: result.AuthorizationURL,
		ExpiresAt:        time.Now().Add(15 * time.Minute),
	}, nil
}

// UpdateDepositKYCStatus pushes the verified eID result to the XMiete API.
// Only ProviderReference is forwarded — never raw PII from the eID chip.
func (s *VerificationService) UpdateDepositKYCStatus(ctx context.Context, depositID string, payload KYCUpdatePayload, bearerToken string) error {
	body := fmt.Sprintf(
		`{"eid_status":%q,"verification_timestamp":%q,"provider_reference":%q}`,
		string(payload.EIDStatus),
		payload.VerificationTimestamp.UTC().Format(time.RFC3339),
		payload.ProviderReference,
	)

	req, err := http.NewRequestWithContext(ctx, http.MethodPatch,
		s.xmieteAPIBaseURL+"/deposits/"+depositID+"/identity", strings.NewReader(body))
	if err != nil {
		return fmt.Errorf("eid: build kyc request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+bearerToken)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("eid: xmiete api request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("eid: kyc update failed: HTTP %d", resp.StatusCode)
	}
	return nil
}
