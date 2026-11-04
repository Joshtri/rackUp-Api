use std::sync::OnceLock;

use argon2::Argon2;
use argon2::password_hash::phc::PasswordHash;
use argon2::password_hash::{PasswordHasher, PasswordVerifier};
use chrono::{DateTime, Utc};
use jsonwebtoken::{Algorithm, DecodingKey, EncodingKey, Header, Validation, decode, encode};
use serde::{Deserialize, Serialize};
use sqlx::PgPool;
use uuid::Uuid;

use super::dto::{AuthResponse, LoginRequest, UserResponse};
use super::repo;
use crate::config::Settings;
use crate::entities::User;
use crate::shared::AppError;

pub async fn login(
    db: &PgPool,
    settings: &Settings,
    req: LoginRequest,
) -> Result<AuthResponse, AppError> {
    let username = req.username.trim().to_lowercase();
    let user = repo::find_by_username(db, &username).await?;

    // Hash something even when the username is unknown, so response time
    // doesn't reveal which usernames exist.
    let stored = match &user {
        Some(u) => u.password_hash.clone(),
        None => dummy_hash().to_string(),
    };
    let valid = verify_password(req.password, stored).await?;

    match user {
        // Only say "expired" after the password checked out, so the
        // message can't be used to probe for usernames.
        Some(user) if valid && user.is_expired() => Err(AppError::AccountExpired),
        Some(user) if valid => auth_response(user, settings),
        _ => Err(AppError::InvalidCredentials),
    }
}

pub async fn me(db: &PgPool, user_id: Uuid) -> Result<UserResponse, AppError> {
    let user = repo::find_by_id(db, user_id)
        .await?
        // Token is valid but the account is gone.
        .ok_or(AppError::Unauthorized)?;
    Ok(user.into())
}

fn auth_response(user: User, settings: &Settings) -> Result<AuthResponse, AppError> {
    let exp = token_expiry(Utc::now(), settings.jwt_ttl_days, user.expires_at);
    let token = issue_token(user.id, &settings.jwt_secret, exp)?;
    Ok(AuthResponse {
        token,
        expires_at: exp * 1000,
        user: user.into(),
    })
}

/// Token lifetime in epoch seconds: `ttl_days` from now, but never past the
/// account's own expiry.
fn token_expiry(now: DateTime<Utc>, ttl_days: i64, account_expires_at: DateTime<Utc>) -> i64 {
    (now.timestamp() + ttl_days * 24 * 60 * 60).min(account_expires_at.timestamp())
}

// ---- Passwords (Argon2id). CPU-heavy, so it runs off the async threads. ----

pub async fn hash_password(password: String) -> Result<String, AppError> {
    tokio::task::spawn_blocking(move || {
        Argon2::default()
            .hash_password(password.as_bytes())
            .map(|h| h.to_string())
            .map_err(|e| AppError::Internal(format!("password hash failed: {e}")))
    })
    .await
    .map_err(|e| AppError::Internal(format!("password hash task failed: {e}")))?
}

async fn verify_password(password: String, stored_hash: String) -> Result<bool, AppError> {
    tokio::task::spawn_blocking(move || {
        let parsed = PasswordHash::new(&stored_hash)
            .map_err(|e| AppError::Internal(format!("stored password hash is invalid: {e}")))?;
        Ok(Argon2::default()
            .verify_password(password.as_bytes(), &parsed)
            .is_ok())
    })
    .await
    .map_err(|e| AppError::Internal(format!("password verify task failed: {e}")))?
}

fn dummy_hash() -> &'static str {
    static HASH: OnceLock<String> = OnceLock::new();
    HASH.get_or_init(|| {
        Argon2::default()
            .hash_password(b"not-a-real-password")
            .expect("hashing a constant cannot fail")
            .to_string()
    })
}

// ---- Tokens (JWT, HS256) ----

#[derive(Debug, Serialize, Deserialize)]
pub struct Claims {
    /// User id.
    pub sub: Uuid,
    pub iat: i64,
    pub exp: i64,
}

/// Signs a token for `user_id` that expires at `exp` (epoch seconds).
pub fn issue_token(user_id: Uuid, secret: &str, exp: i64) -> Result<String, AppError> {
    let claims = Claims {
        sub: user_id,
        iat: Utc::now().timestamp(),
        exp,
    };
    encode(
        &Header::new(Algorithm::HS256),
        &claims,
        &EncodingKey::from_secret(secret.as_bytes()),
    )
    .map_err(|e| AppError::Internal(format!("token encode failed: {e}")))
}

pub fn verify_token(token: &str, secret: &str) -> Result<Claims, AppError> {
    decode::<Claims>(
        token,
        &DecodingKey::from_secret(secret.as_bytes()),
        &Validation::new(Algorithm::HS256),
    )
    .map(|data| data.claims)
    .map_err(|_| AppError::Unauthorized)
}

#[cfg(test)]
mod tests {
    use chrono::Duration;

    use super::*;

    const SECRET: &str = "test-secret-that-is-at-least-32-chars";

    #[test]
    fn token_round_trip() {
        let id = Uuid::new_v4();
        let exp = Utc::now().timestamp() + 3600;
        let token = issue_token(id, SECRET, exp).unwrap();
        assert_eq!(verify_token(&token, SECRET).unwrap().sub, id);
    }

    #[test]
    fn rejects_wrong_secret_and_expired_token() {
        let token = issue_token(Uuid::new_v4(), SECRET, Utc::now().timestamp() + 3600).unwrap();
        assert!(verify_token(&token, "another-secret-that-is-32-chars-long").is_err());

        let expired = issue_token(Uuid::new_v4(), SECRET, 1_000).unwrap();
        assert!(verify_token(&expired, SECRET).is_err());
    }

    #[test]
    fn token_never_outlives_the_account() {
        let now = Utc::now();
        // Account ends in 3 days, token TTL is 30: token ends with the account.
        let soon = now + Duration::days(3);
        assert_eq!(token_expiry(now, 30, soon), soon.timestamp());
        // Account ends in 60 days: the normal TTL applies.
        let later = now + Duration::days(60);
        assert_eq!(token_expiry(now, 30, later), now.timestamp() + 30 * 86_400);
    }

    #[tokio::test]
    async fn password_hash_round_trip() {
        let hash = hash_password("correct horse".into()).await.unwrap();
        assert!(hash.starts_with("$argon2id$"));
        assert!(
            verify_password("correct horse".into(), hash.clone())
                .await
                .unwrap()
        );
        assert!(!verify_password("wrong".into(), hash).await.unwrap());
    }
}
