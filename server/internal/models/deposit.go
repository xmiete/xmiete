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
package models

import "time"

type LifecycleState string

const (
	StateRequested  LifecycleState = "REQUESTED"
	StateIdentified LifecycleState = "IDENTIFIED"
	StateFunded     LifecycleState = "FUNDED"
	StatePledged    LifecycleState = "PLEDGED"
	StateReleased   LifecycleState = "RELEASED"
	StateClaimed    LifecycleState = "CLAIMED"
	StateClosed     LifecycleState = "CLOSED"
)

type DepositType string

const (
	DepositTypeCash          DepositType = "CASH_EQUIVALENT"
	DepositTypeBankGuarantee DepositType = "BANK_GUARANTEE"
	DepositTypeInsurance     DepositType = "INSURANCE"
)

type EIDStatus string

const (
	EIDNotStarted EIDStatus = "NOT_STARTED"
	EIDInProgress EIDStatus = "IN_PROGRESS"
	EIDVerified   EIDStatus = "VERIFIED"
	EIDFailed     EIDStatus = "FAILED"
)

type Address struct {
	Street  string `json:"street"`
	ZIP     string `json:"zip"`
	City    string `json:"city"`
	Country string `json:"country"`
}

// WalletMetadata holds eIDAS 2.0 / EUDI-Wallet credential details from a VP presentation.
type WalletMetadata struct {
	Issuer          string `json:"issuer,omitempty"`
	IssuerID        string `json:"issuer_id,omitempty"`
	WalletProvider  string `json:"wallet_provider,omitempty"`
	CredentialType  string `json:"credential_type,omitempty"`
	CredentialFormat string `json:"credential_format,omitempty"`
	AssuranceLevel  string `json:"assurance_level,omitempty"`
	PresentationID  string `json:"presentation_id,omitempty"`
	IssuingCountry  string `json:"issuing_country,omitempty"`
	VerifiedAt      string `json:"verified_at,omitempty"`
	ExpiresAt       string `json:"expires_at,omitempty"`
}

type Tenant struct {
	FirstName      string          `json:"first_name"`
	LastName       string          `json:"last_name"`
	Email          string          `json:"email"`
	TaxID          string          `json:"tax_id,omitempty"`
	EIDStatus      EIDStatus       `json:"eid_status,omitempty"`
	WalletMetadata *WalletMetadata `json:"wallet_metadata,omitempty"`
	Address        *Address        `json:"address,omitempty"`
}

type Landlord struct {
	Name string `json:"name"`
	Type string `json:"type,omitempty"`
	IBAN string `json:"iban,omitempty"`
}

type Property struct {
	Address Address `json:"address"`
	UnitID  string  `json:"unit_id,omitempty"`
}

type HistoryEntry struct {
	State     LifecycleState `json:"state"`
	Timestamp time.Time      `json:"timestamp"`
	Actor     string         `json:"actor,omitempty"`
	Comment   string         `json:"comment,omitempty"`
	Signature string         `json:"signature,omitempty"`
}

type DepositData struct {
	Amount         float64        `json:"amount"`
	Currency       string         `json:"currency"`
	Type           DepositType    `json:"type"`
	LifecycleState LifecycleState `json:"lifecycle_state"`
	History        []HistoryEntry `json:"history,omitempty"`
}

type Pledge struct {
	PledgeDate        string `json:"pledge_date,omitempty"`
	StatutoryBasis    string `json:"statutory_basis,omitempty"`
	IsConfirmedByBank bool   `json:"is_confirmed_by_bank"`
}

type TrustAccountType string

const (
	TrustAccountTreuhandkonto TrustAccountType = "TREUHANDKONTO"
	TrustAccountAnderkonto    TrustAccountType = "ANDERKONTO"
	TrustAccountPooled        TrustAccountType = "POOLED_TREUHAND"
)

// Trusteeship models the BGB § 551 Abs. 3 requirement that deposit funds are held
// in a legally separated trust account (Treuhandkonto), insolvency-proof from the landlord.
type Trusteeship struct {
	AccountType                       TrustAccountType `json:"account_type,omitempty"`
	TrusteeEntity                     string           `json:"trustee_entity,omitempty"`
	TrustAccountIBAN                  string           `json:"trust_account_iban,omitempty"`
	InsolvencyProtectionConfirmed     bool             `json:"insolvency_protection_confirmed"`
	InsolvencyProtectionConfirmedDate string           `json:"insolvency_protection_confirmed_date,omitempty"`
	StatutoryBasis                    string           `json:"statutory_basis,omitempty"`
}

type Provider struct {
	ProviderType          string `json:"provider_type"`
	ExecutingEntity       string `json:"executing_entity"`
	BrandName             string `json:"brand_name,omitempty"`
	ProviderID            string `json:"provider_id,omitempty"`
	InsurancePolicyNumber string `json:"insurance_policy_number,omitempty"`
}

type Meta struct {
	Version    string    `json:"version"`
	Timestamp  time.Time `json:"timestamp"`
	ExternalID string    `json:"external_id,omitempty"`
}

type TransportProtocol string

const (
	TransportREST  TransportProtocol = "REST"
	TransportEBICS TransportProtocol = "EBICS"
)

type EBICSVersion string

const (
	EBICS24 EBICSVersion = "2.4"
	EBICS25 EBICSVersion = "2.5"
	EBICS30 EBICSVersion = "3.0"
)

type EBICSSignatureClass string

const (
	EBICSSignatureT EBICSSignatureClass = "T" // transport only
	EBICSSignatureE EBICSSignatureClass = "E" // electronic / single signature
	EBICSSignatureA EBICSSignatureClass = "A" // authorization / collective signature
)

// EBICSBTF is the EBICS 3.0 Business Transaction Format descriptor (replaces order types).
type EBICSBTF struct {
	ServiceName   string `json:"service_name"`
	ServiceOption string `json:"service_option,omitempty"`
	Scope         string `json:"scope,omitempty"`
	Container     string `json:"container,omitempty"`
	MsgName       string `json:"msg_name"`
	MsgVersion    string `json:"msg_version,omitempty"`
}

// EBICSMetadata holds the EBICS connection parameters for a bank integration.
type EBICSMetadata struct {
	HostID         string              `json:"host_id"`
	PartnerID      string              `json:"partner_id"`
	UserID         string              `json:"user_id"`
	EBICSVersion   EBICSVersion        `json:"ebics_version"`
	BankURL        string              `json:"bank_url"`
	OrderType      string              `json:"order_type,omitempty"`
	BTF            *EBICSBTF           `json:"btf,omitempty"`
	SignatureClass EBICSSignatureClass `json:"signature_class,omitempty"`
	BankBIC        string              `json:"bank_bic,omitempty"`
}

// Transport declares how XMiete messages are exchanged with the bank system.
// Defaults to REST; set Protocol to EBICS and populate EBICSMetadata for EBICS-connected banks.
type Transport struct {
	Protocol      TransportProtocol `json:"protocol"`
	EBICSMetadata *EBICSMetadata    `json:"ebics_metadata,omitempty"`
}

// Deposit is the top-level domain object, mirroring xmiete_schema.json.
type Deposit struct {
	ID          string       `json:"id"`
	Meta        Meta         `json:"meta"`
	Tenant      Tenant       `json:"tenant"`
	Landlord    Landlord     `json:"landlord"`
	Property    Property     `json:"property"`
	Deposit     DepositData  `json:"deposit"`
	Pledge      *Pledge      `json:"pledge,omitempty"`
	Trusteeship *Trusteeship `json:"trusteeship,omitempty"`
	Provider    *Provider    `json:"provider,omitempty"`
	Transport   *Transport   `json:"transport,omitempty"`
}

// Request/response payloads for action endpoints.

type IdentityUpdateRequest struct {
	EIDStatus             EIDStatus       `json:"eid_status"`
	VerificationTimestamp string          `json:"verification_timestamp,omitempty"`
	ProviderReference     string          `json:"provider_reference,omitempty"`
	WalletMetadata        *WalletMetadata `json:"wallet_metadata,omitempty"`
}

type PledgeRequest struct {
	PledgeDate          string `json:"pledge_date"`
	IsConfirmedByBank   bool   `json:"is_confirmed_by_bank"`
	ProviderReference   string `json:"provider_reference,omitempty"`
}

type ReleaseRequest struct {
	ReleaseType            string  `json:"release_type,omitempty"`
	ReleaseAmount          float64 `json:"release_amount,omitempty"`
	LandlordSignatureToken string  `json:"landlord_signature_token,omitempty"`
}

type ClaimRequest struct {
	ClaimAmount  float64  `json:"claim_amount"`
	Reason       string   `json:"reason"`
	EvidenceURLs []string `json:"evidence_urls,omitempty"`
}

type ErrorResponse struct {
	Error string `json:"error"`
	Code  string `json:"code,omitempty"`
}

type WebhookEvent struct {
	EventType          string         `json:"event_type"`
	DepositID          string         `json:"deposit_id"`
	NewState           LifecycleState `json:"new_state"`
	Timestamp          time.Time      `json:"timestamp"`
	CredentialOfferURL string         `json:"credential_offer_url,omitempty"` // set on deposit.pledged when QEAA offer is created
}

// ── QEAA Issuance request/response models ─────────────────────────────────────

// IssueCredentialRequest is sent by the bank to trigger QEAA issuance after pledge.
type IssueCredentialRequest struct {
	// ValidUntil is the ISO 8601 date until which the pledge is in effect (optional).
	// Drives the credential expiry and the pledged_until selectively-disclosed claim.
	ValidUntil string `json:"valid_until,omitempty"`
}

// IssueCredentialResponse is returned to the bank with the credential offer.
type IssueCredentialResponse struct {
	SessionID          string    `json:"session_id"`
	CredentialOfferURL string    `json:"credential_offer_url"`
	QRCodePayload      string    `json:"qr_code_payload"`
	ExpiresAt          time.Time `json:"expires_at"`
}
