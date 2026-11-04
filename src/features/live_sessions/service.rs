use sqlx::PgPool;
use uuid::Uuid;

use super::dto::{
    CreateLiveSessionRequest, EndLiveSessionRequest, LiveSessionResponse, UpdateLiveSessionRequest,
    validate_state,
};
use super::repo;
use crate::features::matches::dto::MatchResponse;
use crate::features::matches::service as matches;
use crate::shared::AppError;

// Like the history, live sessions are shared: any logged-in account can
// follow and control any of them.

pub async fn list_active(db: &PgPool) -> Result<Vec<LiveSessionResponse>, AppError> {
    let rows = repo::list_active(db).await?;
    Ok(rows.into_iter().map(Into::into).collect())
}

/// The session, or `None` when the caller already has version `since`.
pub async fn get(
    db: &PgPool,
    id: Uuid,
    since: Option<i32>,
) -> Result<Option<LiveSessionResponse>, AppError> {
    let session = repo::find(db, id).await?.ok_or(AppError::NotFound)?;
    if since == Some(session.version) {
        return Ok(None);
    }
    Ok(Some(session.into()))
}

/// Creates a session. Repeating it with the same id returns the stored one
/// unchanged; the bool is `true` when a new row was created.
pub async fn create(
    db: &PgPool,
    user_id: Uuid,
    req: CreateLiveSessionRequest,
) -> Result<(LiveSessionResponse, bool), AppError> {
    validate_state(&req.state)?;
    let created = repo::insert(db, req.id, user_id, &req.state).await?;
    let session = repo::find(db, req.id).await?.ok_or(AppError::NotFound)?;
    Ok((session.into(), created))
}

/// Saves a change made on top of `req.version`.
/// 409 when another device changed the session first or it has ended: the
/// client then fetches the latest state and re-applies its tap.
pub async fn update(
    db: &PgPool,
    id: Uuid,
    req: UpdateLiveSessionRequest,
) -> Result<LiveSessionResponse, AppError> {
    validate_state(&req.state)?;
    if !repo::update_state(db, id, req.version, &req.state).await? {
        let current = repo::find(db, id).await?.ok_or(AppError::NotFound)?;
        let reason = if current.ended_at.is_some() {
            "this session has ended"
        } else {
            "this session was changed on another device"
        };
        return Err(AppError::Conflict(reason.into()));
    }
    let session = repo::find(db, id).await?.ok_or(AppError::NotFound)?;
    Ok(session.into())
}

/// Ends a session, saving its result in the history when `req.save` is set.
/// Ending an already ended session changes nothing, so when two phones end
/// it at the same time the first one wins and at most one match is saved.
pub async fn end(
    db: &PgPool,
    user_id: Uuid,
    id: Uuid,
    req: EndLiveSessionRequest,
) -> Result<(LiveSessionResponse, Option<MatchResponse>), AppError> {
    let new_match = req
        .save
        .map(|mut m| {
            // Same id as the session, so a retried end can't save a second match.
            m.id = Some(id);
            matches::to_new_match(m)
        })
        .transpose()?;

    let mut tx = db.begin().await?;
    let ended = repo::lock_ended(&mut *tx, id)
        .await?
        .ok_or(AppError::NotFound)?;
    let mut saved = None;
    if !ended {
        if let Some(m) = new_match {
            saved = Some(matches::create_in(&mut tx, user_id, m).await?.0);
        }
        let saved_id = saved.as_ref().map(|m| m.id);
        repo::mark_ended(&mut *tx, id, user_id, saved_id).await?;
    }
    let session = repo::find(&mut *tx, id).await?.ok_or(AppError::NotFound)?;
    tx.commit().await?;
    Ok((session.into(), saved))
}
