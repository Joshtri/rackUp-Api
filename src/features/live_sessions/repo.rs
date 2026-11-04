use serde_json::Value;
use sqlx::PgExecutor;
use sqlx::types::Json;
use uuid::Uuid;

use crate::entities::LiveSession;

/// `SELECT` of every `LiveSession` column, with the name of whoever ended
/// the session, followed by `$rest`. A macro so the SQL stays a static string.
macro_rules! select {
    ($rest:literal) => {
        concat!(
            "SELECT s.*, u.display_name AS ended_by_name
             FROM live_sessions s LEFT JOIN users u ON u.id = s.ended_by ",
            $rest
        )
    };
}

/// Running sessions touched in the last 12 hours, newest first.
/// Older ones were most likely left open by accident.
pub async fn list_active<'e>(db: impl PgExecutor<'e>) -> Result<Vec<LiveSession>, sqlx::Error> {
    sqlx::query_as::<_, LiveSession>(select!(
        "WHERE s.ended_at IS NULL AND s.updated_at > now() - INTERVAL '12 hours'
         ORDER BY s.updated_at DESC LIMIT 20"
    ))
    .fetch_all(db)
    .await
}

pub async fn find<'e>(
    db: impl PgExecutor<'e>,
    id: Uuid,
) -> Result<Option<LiveSession>, sqlx::Error> {
    sqlx::query_as::<_, LiveSession>(select!("WHERE s.id = $1"))
        .bind(id)
        .fetch_optional(db)
        .await
}

/// Whether the session has ended, locking its row until the transaction ends.
/// `None` when it doesn't exist.
pub async fn lock_ended<'e>(
    db: impl PgExecutor<'e>,
    id: Uuid,
) -> Result<Option<bool>, sqlx::Error> {
    sqlx::query_scalar::<_, bool>(
        "SELECT ended_at IS NOT NULL FROM live_sessions WHERE id = $1 FOR UPDATE",
    )
    .bind(id)
    .fetch_optional(db)
    .await
}

/// Returns `false` when the id already exists.
pub async fn insert<'e>(
    db: impl PgExecutor<'e>,
    id: Uuid,
    user_id: Uuid,
    state: &Value,
) -> Result<bool, sqlx::Error> {
    let result = sqlx::query(
        "INSERT INTO live_sessions (id, created_by, state) VALUES ($1, $2, $3)
         ON CONFLICT (id) DO NOTHING",
    )
    .bind(id)
    .bind(user_id)
    .bind(Json(state))
    .execute(db)
    .await?;
    Ok(result.rows_affected() == 1)
}

/// Saves a new state if the session is still at `version` and running.
/// Returns `false` when it isn't (another device changed it first, or it ended).
pub async fn update_state<'e>(
    db: impl PgExecutor<'e>,
    id: Uuid,
    version: i32,
    state: &Value,
) -> Result<bool, sqlx::Error> {
    let result = sqlx::query(
        "UPDATE live_sessions SET state = $3, version = version + 1, updated_at = now()
         WHERE id = $1 AND version = $2 AND ended_at IS NULL",
    )
    .bind(id)
    .bind(version)
    .bind(Json(state))
    .execute(db)
    .await?;
    Ok(result.rows_affected() == 1)
}

pub async fn mark_ended<'e>(
    db: impl PgExecutor<'e>,
    id: Uuid,
    user_id: Uuid,
    saved_match_id: Option<Uuid>,
) -> Result<(), sqlx::Error> {
    sqlx::query(
        "UPDATE live_sessions
         SET ended_at = now(), ended_by = $2, saved_match_id = $3,
             version = version + 1, updated_at = now()
         WHERE id = $1",
    )
    .bind(id)
    .bind(user_id)
    .bind(saved_match_id)
    .execute(db)
    .await?;
    Ok(())
}
