pub mod models;

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
