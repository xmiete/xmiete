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

package openid4vp

import "time"

// VpRequest is the OpenID4VP Authorization Request Object sent to the wallet.
type VpRequest struct {
	ClientID               string                 `json:"client_id"`
	ResponseType           string                 `json:"response_type"`
	ResponseMode           string                 `json:"response_mode"`
	ResponseURI            string                 `json:"response_uri"`
	Nonce                  string                 `json:"nonce"`
	State                  string                 `json:"state"`
	PresentationDefinition PresentationDefinition `json:"presentation_definition"`
}

// PresentationDefinition is a DIF Presentation Exchange top-level definition.
type PresentationDefinition struct {
	ID               string            `json:"id"`
	InputDescriptors []InputDescriptor `json:"input_descriptors"`
}

// InputDescriptor describes what credential the verifier is requesting.
type InputDescriptor struct {
	ID          string                `json:"id"`
	Format      map[string]FormatAlgs `json:"format"`
	Constraints Constraints           `json:"constraints"`
}

// FormatAlgs lists the supported signing algorithms for a given credential format.
type FormatAlgs struct {
	Alg []string `json:"alg"`
}

// Constraints holds the field-level constraints on the requested credential.
type Constraints struct {
	Fields          []Field `json:"fields"`
	LimitDisclosure string  `json:"limit_disclosure"`
}

// Field is a single JSONPath-addressed field constraint.
type Field struct {
	Path     []string     `json:"path"`
	Filter   *FieldFilter `json:"filter,omitempty"`
	Optional bool         `json:"optional,omitempty"`
}

// FieldFilter is a JSON Schema-style filter applied to a field value.
type FieldFilter struct {
	Type  string  `json:"type"`
	Const *string `json:"const,omitempty"`
}

// VpRequestResult bundles the nonce (to be stored for later verification) and the request object.
type VpRequestResult struct {
	Nonce     string
	VpRequest VpRequest
}

// VerifiedClaims holds the claims extracted from a successfully verified DepositPledgeAttestation VP.
type VerifiedClaims struct {
	CredentialID    string
	DepositID       string
	PledgeDate      string
	StatutoryBasis  string
	IssuingBank     string
	DepositAmount   *float64
	Currency        *string
	PropertyAddress *string
	TenantFirstName *string
	TenantLastName  *string
	PledgedUntil    *string
	VerifiedAt      time.Time
}

// IssuerClaims holds the parsed payload of the SD-JWT issuer JWT.
type IssuerClaims struct {
	CredentialID   string   // JWT claim: "jti"
	DepositID      string
	PledgeDate     string
	StatutoryBasis string
	IssuingBank    string
	SDHashes       []string // "_sd" array
	Exp            int64
}
