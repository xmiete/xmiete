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

use xmiete_sdk::XMieteClient;
use xmiete_sdk::models::Deposit;
use async_trait::async_trait;

struct MockClient;

#[async_trait]
impl XMieteClient for MockClient {
    async fn create_deposit(&self, deposit: Deposit) -> Result<Deposit, String> {
        Ok(deposit)
    }

    async fn get_deposit(&self, _id: &str) -> Result<Deposit, String> {
        Err("Not implemented in mock".to_string())
    }

    async fn confirm_pledge(&self, _id: &str, _date: &str) -> Result<(), String> {
        println!("Mock: Confirming pledge...");
        Ok(())
    }

    async fn release_deposit(&self, _id: &str, _tok: &str) -> Result<(), String> {
        Ok(())
    }
}

#[tokio::main]
async fn main() {
    let client = MockClient;

    println!("XMiete Rust SDK Example Usage");

    let pledge_result = client.confirm_pledge("DEP-456", "2026-05-07").await;

    match pledge_result {
        Ok(_) => println!("Successfully called confirm_pledge"),
        Err(e) => eprintln!("Error: {}", e),
    }
}
