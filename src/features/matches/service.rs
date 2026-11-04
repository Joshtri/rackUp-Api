use std::collections::HashSet;

use chrono::{DateTime, Utc};
use sqlx::{PgConnection, PgPool};
use uuid::Uuid;

use super::dto::{CreateMatchRequest, ImportRequest, ImportResponse, MatchFields, MatchResponse};
use super::repo::{self, NewMatch};
use crate::entities::MatchLog;
use crate::features::players;
use crate::shared::AppError;
use crate::shared::validation::same_name;

const MAX_IMPORT: usize = 5_000;
const MAX_SHOTS: usize = 20_000;
const MAX_RACKS: usize = 2_000;

// History is shared by all accounts. `user_id` below is only used to record
// who saved a match.

pub async fn list(db: &PgPool) -> Result<Vec<MatchResponse>, AppError> {
    let rows = repo::list(db).await?;
    Ok(rows.into_iter().map(Into::into).collect())
}

pub async fn get(db: &PgPool, id: Uuid) -> Result<MatchResponse, AppError> {
    repo::find(db, id)
        .await?
        .map(Into::into)
        .ok_or(AppError::NotFound)
}

/// Saves a match. Player names reuse an existing spelling ("budi" → "Budi"),
/// like `canonicalName` in the frontend.
///
/// If `id` is given and already exists, the stored match is returned unchanged
/// (so undoing a delete twice is safe). The bool is `true` when a new row was
/// created.
pub async fn create(
    db: &PgPool,
    user_id: Uuid,
    req: CreateMatchRequest,
) -> Result<(MatchResponse, bool), AppError> {
    let mut conn = db.acquire().await?;
    create_in(&mut conn, user_id, to_new_match(req)?).await
}

/// `create` on an open connection or transaction, for an already validated match.
pub async fn create_in(
    conn: &mut PgConnection,
    user_id: Uuid,
    mut new: NewMatch,
) -> Result<(MatchResponse, bool), AppError> {
    if let Some(name) = players::repo::find_spelling(&mut *conn, &new.player_a).await? {
        new.player_a = name;
    }
    if let Some(name) = players::repo::find_spelling(&mut *conn, &new.player_b).await? {
        new.player_b = name;
    }

    if let Some(created) = repo::insert(&mut *conn, user_id, &new).await? {
        return Ok((created.into(), true));
    }
    match repo::find(&mut *conn, new.id).await? {
        Some(existing) => Ok((existing.into(), false)),
        None => Err(AppError::Conflict("this match id is already in use".into())),
    }
}

/// Edits a match, matching the frontend's `update`:
/// - the shot-clock log can't be edited; it is kept, and flipped if the
///   players swapped sides;
/// - a name typed with different capitalisation is treated as a fix and
///   renamed across all matches.
pub async fn update(db: &PgPool, id: Uuid, fields: MatchFields) -> Result<MatchResponse, AppError> {
    let data = fields.validate()?;
    let mut tx = db.begin().await?;

    let existing = repo::find_for_update(&mut *tx, id)
        .await?
        .ok_or(AppError::NotFound)?;
    let swapped = same_name(&data.player_a, &existing.player_b)
        && same_name(&data.player_b, &existing.player_a);
    let log = existing
        .log
        .map(|l| if swapped { l.0.flipped() } else { l.0 });

    let updated = repo::update(&mut *tx, id, &data, log.as_ref()).await?;
    repo::rename_player(&mut *tx, &data.player_a).await?;
    repo::rename_player(&mut *tx, &data.player_b).await?;

    tx.commit().await?;
    Ok(updated.into())
}

pub async fn delete(db: &PgPool, id: Uuid) -> Result<(), AppError> {
    match repo::delete(db, id).await? {
        0 => Err(AppError::NotFound),
        _ => Ok(()),
    }
}

/// Merges a backup by id: existing ids are skipped, nothing is overwritten.
/// The whole import is rejected if any match is invalid.
pub async fn import(
    db: &PgPool,
    user_id: Uuid,
    req: ImportRequest,
) -> Result<ImportResponse, AppError> {
    let received = req.matches.len() as u64;
    if req.matches.is_empty() {
        return Err(AppError::Validation("matches is empty".into()));
    }
    if req.matches.len() > MAX_IMPORT {
        return Err(AppError::Validation(format!(
            "at most {MAX_IMPORT} matches per import"
        )));
    }

    let mut seen = HashSet::new();
    let mut rows = Vec::with_capacity(req.matches.len());
    for (i, item) in req.matches.into_iter().enumerate() {
        let row = to_new_match(item).map_err(|err| match err {
            AppError::Validation(msg) => AppError::Validation(format!("matches[{i}]: {msg}")),
            other => other,
        })?;
        if seen.insert(row.id) {
            rows.push(row);
        }
    }

    let added = repo::insert_many(db, user_id, &rows).await?;
    Ok(ImportResponse {
        added,
        skipped: received - added,
    })
}

/// Validates a create request and turns it into a row to insert.
pub fn to_new_match(req: CreateMatchRequest) -> Result<NewMatch, AppError> {
    let data = req.fields.validate()?;
    if let Some(log) = &req.log {
        validate_log(log)?;
    }
    let created_at = match req.created_at {
        Some(ms) => DateTime::from_timestamp_millis(ms)
            .ok_or_else(|| AppError::Validation("createdAt is out of range".into()))?,
        None => Utc::now(),
    };
    Ok(NewMatch {
        id: req.id.unwrap_or_else(Uuid::new_v4),
        played_on: data.played_on,
        player_a: data.player_a,
        player_b: data.player_b,
        score_a: data.score_a,
        score_b: data.score_b,
        note: data.note,
        log: req.log,
        created_at,
    })
}

fn validate_log(log: &MatchLog) -> Result<(), AppError> {
    if log.shots.len() > MAX_SHOTS || log.racks.len() > MAX_RACKS {
        return Err(AppError::Validation("log is too large".into()));
    }
    Ok(())
}
