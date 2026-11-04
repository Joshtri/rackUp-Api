use axum::Json;
use axum::extract::State;

use super::dto::{AccountResponse, AuthResponse, LoginRequest, UserResponse};
use super::repo;
use super::service;
use crate::app::state::AppState;
use crate::middlewares::AuthUser;
use crate::shared::AppError;
use crate::shared::extract::AppJson;

pub async fn login(
    State(state): State<AppState>,
    AppJson(req): AppJson<LoginRequest>,
) -> Result<Json<AuthResponse>, AppError> {
    Ok(Json(service::login(&state.db, &state.settings, req).await?))
}

pub async fn me(
    State(state): State<AppState>,
    user: AuthUser,
) -> Result<Json<UserResponse>, AppError> {
    Ok(Json(service::me(&state.db, user.id).await?))
}

/// Active accounts, so a session's players can be linked to them.
pub async fn users(
    State(state): State<AppState>,
    _user: AuthUser,
) -> Result<Json<Vec<AccountResponse>>, AppError> {
    let users = repo::list(&state.db).await?;
    Ok(Json(
        users
            .into_iter()
            .filter(|u| !u.is_expired())
            .map(Into::into)
            .collect(),
    ))
}
