use axum::Json;
use axum::extract::{Query, State};
use axum::http::StatusCode;
use chrono::Utc;
use uuid::Uuid;

use super::dto::{
    CreateLiveSessionRequest, EndLiveSessionRequest, EndResponse, ListResponse, OneResponse,
    SinceQuery, UpdateLiveSessionRequest,
};
use super::service;
use crate::app::state::AppState;
use crate::middlewares::AuthUser;
use crate::shared::AppError;
use crate::shared::extract::{AppJson, AppPath};

fn now() -> i64 {
    Utc::now().timestamp_millis()
}

pub async fn list(
    State(state): State<AppState>,
    _user: AuthUser,
) -> Result<Json<ListResponse>, AppError> {
    let sessions = service::list_active(&state.db).await?;
    Ok(Json(ListResponse {
        server_now: now(),
        sessions,
    }))
}

/// `?since=<version>`: `session` is left out when nothing changed, which keeps
/// the once-a-second poll tiny.
pub async fn get(
    State(state): State<AppState>,
    _user: AuthUser,
    AppPath(id): AppPath<Uuid>,
    Query(q): Query<SinceQuery>,
) -> Result<Json<OneResponse>, AppError> {
    let session = service::get(&state.db, id, q.since).await?;
    Ok(Json(OneResponse {
        server_now: now(),
        session,
    }))
}

/// 201 when created, 200 when the id already existed.
pub async fn create(
    State(state): State<AppState>,
    user: AuthUser,
    AppJson(req): AppJson<CreateLiveSessionRequest>,
) -> Result<(StatusCode, Json<OneResponse>), AppError> {
    let (session, created) = service::create(&state.db, user.id, req).await?;
    let status = if created {
        StatusCode::CREATED
    } else {
        StatusCode::OK
    };
    Ok((
        status,
        Json(OneResponse {
            server_now: now(),
            session: Some(session),
        }),
    ))
}

pub async fn update(
    State(state): State<AppState>,
    _user: AuthUser,
    AppPath(id): AppPath<Uuid>,
    AppJson(req): AppJson<UpdateLiveSessionRequest>,
) -> Result<Json<OneResponse>, AppError> {
    let session = service::update(&state.db, id, req).await?;
    Ok(Json(OneResponse {
        server_now: now(),
        session: Some(session),
    }))
}

pub async fn end(
    State(state): State<AppState>,
    user: AuthUser,
    AppPath(id): AppPath<Uuid>,
    AppJson(req): AppJson<EndLiveSessionRequest>,
) -> Result<Json<EndResponse>, AppError> {
    let (session, saved) = service::end(&state.db, user.id, id, req).await?;
    Ok(Json(EndResponse {
        server_now: now(),
        session,
        saved,
    }))
}
