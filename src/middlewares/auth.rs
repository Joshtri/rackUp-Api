use axum::extract::FromRequestParts;
use axum::http::header::AUTHORIZATION;
use axum::http::request::Parts;
use uuid::Uuid;

use crate::app::state::AppState;
use crate::entities::user::is_expired;
use crate::features::auth::repo;
use crate::features::auth::service::verify_token;
use crate::shared::AppError;

/// The logged-in user. Add it as a handler argument to require
/// `Authorization: Bearer <token>`.
///
/// Rejects with 401 `unauthorized` (missing/invalid token, deleted account)
/// or 401 `account_expired`. The expiry is read from the database on every
/// request, so renewing or ending an account takes effect immediately.
#[derive(Debug, Clone, Copy)]
pub struct AuthUser {
    pub id: Uuid,
}

impl FromRequestParts<AppState> for AuthUser {
    type Rejection = AppError;

    async fn from_request_parts(parts: &mut Parts, state: &AppState) -> Result<Self, AppError> {
        let token = parts
            .headers
            .get(AUTHORIZATION)
            .and_then(|v| v.to_str().ok())
            .and_then(|v| v.strip_prefix("Bearer "))
            .ok_or(AppError::Unauthorized)?;
        let claims = verify_token(token.trim(), &state.settings.jwt_secret)?;
        match repo::find_expiry(&state.db, claims.sub).await? {
            None => Err(AppError::Unauthorized),
            Some(expires_at) if is_expired(expires_at) => Err(AppError::AccountExpired),
            Some(_) => Ok(Self { id: claims.sub }),
        }
    }
}
