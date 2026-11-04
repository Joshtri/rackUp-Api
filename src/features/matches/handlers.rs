use axum::Json;
use axum::extract::State;
use axum::http::StatusCode;
use uuid::Uuid;

use super::dto::{CreateMatchRequest, ImportRequest, ImportResponse, MatchFields, MatchResponse};
use super::service;
use crate::app::state::AppState;
use crate::middlewares::AuthUser;
use crate::shared::AppError;
use crate::shared::extract::{AppJson, AppPath};

// History is shared: any logged-in account can read and change every match.

pub async fn list(
    State(state): State<AppState>,
    _user: AuthUser,
) -> Result<Json<Vec<MatchResponse>>, AppError> {
    Ok(Json(service::list(&state.db).await?))
}

pub async fn get(
    State(state): State<AppState>,
    _user: AuthUser,
    AppPath(id): AppPath<Uuid>,
) -> Result<Json<MatchResponse>, AppError> {
    Ok(Json(service::get(&state.db, id).await?))
}

/// 201 when created, 200 when the id already existed (restore after undo).
pub async fn create(
    State(state): State<AppState>,
    user: AuthUser,
    AppJson(req): AppJson<CreateMatchRequest>,
) -> Result<(StatusCode, Json<MatchResponse>), AppError> {
    let (m, created) = service::create(&state.db, user.id, req).await?;
    let status = if created {
        StatusCode::CREATED
    } else {
        StatusCode::OK
    };
    Ok((status, Json(m)))
}

pub async fn update(
    State(state): State<AppState>,
    _user: AuthUser,
    AppPath(id): AppPath<Uuid>,
    AppJson(fields): AppJson<MatchFields>,
) -> Result<Json<MatchResponse>, AppError> {
    Ok(Json(service::update(&state.db, id, fields).await?))
}

pub async fn delete(
    State(state): State<AppState>,
    _user: AuthUser,
    AppPath(id): AppPath<Uuid>,
) -> Result<StatusCode, AppError> {
    service::delete(&state.db, id).await?;
    Ok(StatusCode::NO_CONTENT)
}

pub async fn import(
    State(state): State<AppState>,
    user: AuthUser,
    AppJson(req): AppJson<ImportRequest>,
) -> Result<Json<ImportResponse>, AppError> {
    Ok(Json(service::import(&state.db, user.id, req).await?))
}
