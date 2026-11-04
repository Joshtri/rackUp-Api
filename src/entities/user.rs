use chrono::{DateTime, Utc};
use uuid::Uuid;

/// Row of the `users` table.
#[derive(Debug, Clone, sqlx::FromRow)]
pub struct User {
    pub id: Uuid,
    /// Lowercase, used to log in.
    pub username: String,
    /// Argon2id PHC string. Never the plain password.
    pub password_hash: String,
    pub display_name: String,
    pub created_at: DateTime<Utc>,
    /// The account stops working at this moment.
    pub expires_at: DateTime<Utc>,
}

impl User {
    pub fn is_expired(&self) -> bool {
        is_expired(self.expires_at)
    }
}

pub fn is_expired(expires_at: DateTime<Utc>) -> bool {
    expires_at <= Utc::now()
}
