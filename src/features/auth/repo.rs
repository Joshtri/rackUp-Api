use chrono::{DateTime, Utc};
use sqlx::PgPool;
use uuid::Uuid;

use crate::entities::User;

pub async fn insert(
    db: &PgPool,
    username: &str,
    password_hash: &str,
    display_name: &str,
    expires_at: DateTime<Utc>,
) -> Result<User, sqlx::Error> {
    sqlx::query_as::<_, User>(
        "INSERT INTO users (id, username, password_hash, display_name, expires_at)
         VALUES ($1, $2, $3, $4, $5)
         RETURNING *",
    )
    .bind(Uuid::new_v4())
    .bind(username)
    .bind(password_hash)
    .bind(display_name)
    .bind(expires_at)
    .fetch_one(db)
    .await
}

pub async fn find_by_username(db: &PgPool, username: &str) -> Result<Option<User>, sqlx::Error> {
    sqlx::query_as::<_, User>("SELECT * FROM users WHERE username = $1")
        .bind(username)
        .fetch_optional(db)
        .await
}

pub async fn find_by_id(db: &PgPool, id: Uuid) -> Result<Option<User>, sqlx::Error> {
    sqlx::query_as::<_, User>("SELECT * FROM users WHERE id = $1")
        .bind(id)
        .fetch_optional(db)
        .await
}

/// Only the expiry, for the per-request auth check.
pub async fn find_expiry(db: &PgPool, id: Uuid) -> Result<Option<DateTime<Utc>>, sqlx::Error> {
    sqlx::query_scalar("SELECT expires_at FROM users WHERE id = $1")
        .bind(id)
        .fetch_optional(db)
        .await
}

pub async fn list(db: &PgPool) -> Result<Vec<User>, sqlx::Error> {
    sqlx::query_as::<_, User>("SELECT * FROM users ORDER BY username")
        .fetch_all(db)
        .await
}

pub async fn set_expiry(
    db: &PgPool,
    username: &str,
    expires_at: DateTime<Utc>,
) -> Result<Option<User>, sqlx::Error> {
    sqlx::query_as::<_, User>("UPDATE users SET expires_at = $2 WHERE username = $1 RETURNING *")
        .bind(username)
        .bind(expires_at)
        .fetch_optional(db)
        .await
}

pub async fn set_password(
    db: &PgPool,
    username: &str,
    password_hash: &str,
) -> Result<Option<User>, sqlx::Error> {
    sqlx::query_as::<_, User>("UPDATE users SET password_hash = $2 WHERE username = $1 RETURNING *")
        .bind(username)
        .bind(password_hash)
        .fetch_optional(db)
        .await
}
