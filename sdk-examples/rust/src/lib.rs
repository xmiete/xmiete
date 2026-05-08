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

pub mod eid;
pub mod models;
pub mod openid4vp;

use async_trait::async_trait;
use crate::models::Deposit;

#[async_trait]
pub trait XMieteClient {
    /// Creates a new deposit request.
    async fn create_deposit(&self, deposit: Deposit) -> Result<Deposit, String>;

    /// Retrieves the current state of a deposit.
    async fn get_deposit(&self, id: &str) -> Result<Deposit, String>;

    /// Confirms the legal pledge.
    async fn confirm_pledge(&self, id: &str, pledge_date: &str) -> Result<(), String>;

    /// Authorizes the release of a deposit.
    async fn release_deposit(&self, id: &str, signature_token: &str) -> Result<(), String>;
}
