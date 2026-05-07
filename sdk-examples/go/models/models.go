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

type Deposit struct {
	Meta           Meta           `json:"meta"`
	Tenant         Tenant         `json:"tenant"`
	Landlord       Landlord       `json:"landlord"`
	Property       Property       `json:"property"`
	DepositDetails DepositDetails `json:"deposit_details"`
	Pledge         *Pledge        `json:"pledge,omitempty"`
	Provider       Provider       `json:"provider"`
	History        []HistoryEntry `json:"history"`
}

type Meta struct {
	Version    string    `json:"version"`
	Timestamp  time.Time `json:"timestamp"`
	ExternalID string    `json:"external_id,omitempty"`
}

type Tenant struct {
	FirstName string  `json:"first_name"`
	LastName  string  `json:"last_name"`
	Email     string  `json:"email"`
	TaxID     string  `json:"tax_id,omitempty"`
	EIDStatus string  `json:"eid_status"`
	Address   Address `json:"address"`
}

type Address struct {
	Street  string `json:"street"`
	Zip     string `json:"zip"`
	City    string `json:"city"`
	Country string `json:"country"`
}

type Landlord struct {
	Name string `json:"name"`
	Type string `json:"type"`
	IBAN string `json:"iban,omitempty"`
}

type Property struct {
	Address Address `json:"address"`
	UnitID  string  `json:"unit_id,omitempty"`
}

type DepositDetails struct {
	Amount         float64 `json:"amount"`
	Currency       string  `json:"currency"`
	Type           string  `json:"type"`
	LifecycleState string  `json:"lifecycle_state"`
}

type Pledge struct {
	PledgeDate         string `json:"pledge_date,omitempty"`
	LegalReference     string `json:"legal_reference"`
	IsConfirmedByBank  bool   `json:"is_confirmed_by_bank"`
}

type Provider struct {
	ProviderType           string                 `json:"provider_type"`
	ExecutingEntity        string                 `json:"executing_entity"`
	BrandName              string                 `json:"brand_name,omitempty"`
	InsurancePolicyNumber  string                 `json:"insurance_policy_number,omitempty"`
	CustomFields           map[string]interface{} `json:"custom_fields,omitempty"`
}

type HistoryEntry struct {
	State     string    `json:"state"`
	Timestamp time.Time `json:"timestamp"`
	Actor     string    `json:"actor"`
	Comment   string    `json:"comment,omitempty"`
}
