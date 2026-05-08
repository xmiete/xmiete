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
	"time"
)

// IdentityVerifier is the interface a bank must satisfy to integrate an eID provider.
// Implement this to swap in AusweisApp2, Authada, SkIDentity, Bundesdruckerei,
// or any custom BSI TR-03130 compatible provider without changing any other SDK code.
type IdentityVerifier interface {
	InitiateVerification(ctx context.Context, req VerificationRequest) (*VerificationSession, error)
	UpdateDepositKYCStatus(ctx context.Context, depositID string, payload KYCUpdatePayload, bearerToken string) error
}

type EIDStatus string

const (
	EIDStatusNotStarted EIDStatus = "NOT_STARTED"
	EIDStatusPending    EIDStatus = "PENDING"
	EIDStatusVerified   EIDStatus = "VERIFIED"
	EIDStatusFailed     EIDStatus = "FAILED"
	EIDStatusExpired    EIDStatus = "EXPIRED"
)

// VerificationRequest initiates an eID session for a tenant.
type VerificationRequest struct {
	DepositID   string
	TenantEmail string
	RedirectURI string
	ClientID    string
}

// VerificationSession is returned by the eID provider after session creation.
// Redirect the tenant's browser to AuthorizationURL within the validity window.
type VerificationSession struct {
	SessionID        string
	AuthorizationURL string
	ExpiresAt        time.Time
}

// KYCUpdatePayload is sent to PATCH /deposits/{id}/identity once verification completes.
// Only ProviderReference is stored — never raw PII from the eID chip.
type KYCUpdatePayload struct {
	EIDStatus             EIDStatus
	VerificationTimestamp time.Time
	ProviderReference     string
}

// WebhookEvent is the parsed body of a POST from the eID provider to your webhook endpoint.
type WebhookEvent struct {
	SessionID         string
	DepositID         string
	Status            EIDStatus
	ProviderReference string
	CompletedAt       time.Time
	ErrorCode         string
}
