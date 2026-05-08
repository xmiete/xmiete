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

use serde::{Deserialize, Serialize};
use chrono::{DateTime, Utc, NaiveDate};

#[derive(Debug, Serialize, Deserialize, Clone)]
pub struct Deposit {
    pub meta: Meta,
    pub tenant: Tenant,
    pub landlord: Landlord,
    pub property: Property,
    pub deposit_details: DepositDetails,
    pub pledge: Option<Pledge>,
    pub trusteeship: Option<Trusteeship>,
    pub provider: Provider,
    pub history: Vec<HistoryEntry>,
}

#[derive(Debug, Serialize, Deserialize, Clone)]
pub struct Meta {
    pub version: String,
    pub timestamp: DateTime<Utc>,
    pub external_id: Option<String>,
}

#[derive(Debug, Serialize, Deserialize, Clone)]
pub struct WalletMetadata {
    pub issuer: Option<String>,
    pub issuer_id: Option<String>,
    pub wallet_provider: Option<String>,
    pub credential_type: Option<String>,
    pub credential_format: Option<String>,
    pub assurance_level: Option<String>,
    pub presentation_id: Option<String>,
    pub issuing_country: Option<String>,
    pub verified_at: Option<DateTime<Utc>>,
    pub expires_at: Option<DateTime<Utc>>,
}

#[derive(Debug, Serialize, Deserialize, Clone)]
pub struct Tenant {
    pub first_name: String,
    pub last_name: String,
    pub email: String,
    pub tax_id: Option<String>,
    pub eid_status: String,
    pub wallet_metadata: Option<WalletMetadata>,
    pub address: Address,
}

#[derive(Debug, Serialize, Deserialize, Clone)]
pub struct Address {
    pub street: String,
    pub zip: String,
    pub city: String,
    pub country: String,
}

#[derive(Debug, Serialize, Deserialize, Clone)]
pub struct Landlord {
    pub name: String,
    pub landlord_type: String, // "PRIVATE" or "COMMERCIAL"
    pub iban: Option<String>,
}

#[derive(Debug, Serialize, Deserialize, Clone)]
pub struct Property {
    pub address: Address,
    pub unit_id: Option<String>,
}

#[derive(Debug, Serialize, Deserialize, Clone)]
pub struct DepositDetails {
    pub amount: f64,
    pub currency: String,
    pub deposit_type: String,
    pub lifecycle_state: String,
}

#[derive(Debug, Serialize, Deserialize, Clone)]
pub struct Pledge {
    pub pledge_date: Option<NaiveDate>,
    pub legal_reference: String,
    pub is_confirmed_by_bank: bool,
}

/// BGB § 551 Abs. 3 — insolvency-proof separation of deposit funds from the landlord's estate.
#[derive(Debug, Serialize, Deserialize, Clone)]
pub struct Trusteeship {
    pub is_treuhand: bool,
    pub account_type: Option<String>,
    pub trustee_entity: Option<String>,
    pub trust_account_iban: Option<String>,
    pub insolvency_protection_confirmed: bool,
    pub insolvency_protection_confirmed_date: Option<NaiveDate>,
    pub legal_basis: Option<String>,
}

#[derive(Debug, Serialize, Deserialize, Clone)]
pub struct Provider {
    pub provider_type: String,
    pub executing_entity: String,
    pub brand_name: Option<String>,
    pub insurance_policy_number: Option<String>,
    pub custom_fields: Option<serde_json::Value>,
}

#[derive(Debug, Serialize, Deserialize, Clone)]
pub struct HistoryEntry {
    pub state: String,
    pub timestamp: DateTime<Utc>,
    pub actor: String,
    pub comment: Option<String>,
}
