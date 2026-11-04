use serde::{Deserialize, Serialize};
use uuid::Uuid;

use crate::entities::User;

#[derive(Debug, Deserialize)]
pub struct LoginRequest {
    pub username: String,
    pub password: String,
}

#[derive(Debug, Serialize)]
#[serde(rename_all = "camelCase")]
pub struct UserResponse {
    pub id: Uuid,
    pub username: String,
    pub display_name: String,
    /// Epoch milliseconds, like `Date.now()`.
    pub created_at: i64,
    /// Epoch milliseconds when the account stops working.
    pub expires_at: i64,
}

impl From<User> for UserResponse {
    fn from(u: User) -> Self {
        Self {
            id: u.id,
            username: u.username,
            display_name: u.display_name,
            created_at: u.created_at.timestamp_millis(),
            expires_at: u.expires_at.timestamp_millis(),
        }
    }
}

#[derive(Debug, Serialize)]
#[serde(rename_all = "camelCase")]
pub struct AuthResponse {
    pub token: String,
    /// Epoch milliseconds when the token stops working.
    /// Never later than the account's own `expiresAt`.
    pub expires_at: i64,
    pub user: UserResponse,
}

/// Another account as seen by a logged-in user. Item of `GET /users`.
#[derive(Debug, Serialize)]
#[serde(rename_all = "camelCase")]
pub struct AccountResponse {
    pub id: Uuid,
    pub username: String,
    pub display_name: String,
}

impl From<User> for AccountResponse {
    fn from(u: User) -> Self {
        Self {
            id: u.id,
            username: u.username,
            display_name: u.display_name,
        }
    }
}
