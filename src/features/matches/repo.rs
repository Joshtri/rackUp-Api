use chrono::{DateTime, NaiveDate, Utc};
use serde::Serialize;
use sqlx::PgExecutor;
use sqlx::types::Json;
use uuid::Uuid;

use super::dto::MatchData;
use crate::entities::{Match, MatchLog};

// History is shared: every account sees and edits every match.
// `user_id` is only written on insert, to remember who saved it.

/// A match ready to insert. Serialized as-is for bulk import,
/// so field names must match the `jsonb_to_recordset` columns below.
#[derive(Debug, Serialize)]
pub struct NewMatch {
    pub id: Uuid,
    pub played_on: NaiveDate,
    pub player_a: String,
    pub player_b: String,
    pub score_a: i32,
    pub score_b: i32,
    pub note: Option<String>,
    pub log: Option<MatchLog>,
    pub created_at: DateTime<Utc>,
}

pub async fn list<'e>(db: impl PgExecutor<'e>) -> Result<Vec<Match>, sqlx::Error> {
    sqlx::query_as::<_, Match>("SELECT * FROM matches ORDER BY played_on DESC, created_at DESC")
        .fetch_all(db)
        .await
}

pub async fn find<'e>(db: impl PgExecutor<'e>, id: Uuid) -> Result<Option<Match>, sqlx::Error> {
    sqlx::query_as::<_, Match>("SELECT * FROM matches WHERE id = $1")
        .bind(id)
        .fetch_optional(db)
        .await
}

/// Locks the row until the transaction ends.
pub async fn find_for_update<'e>(
    db: impl PgExecutor<'e>,
    id: Uuid,
) -> Result<Option<Match>, sqlx::Error> {
    sqlx::query_as::<_, Match>("SELECT * FROM matches WHERE id = $1 FOR UPDATE")
        .bind(id)
        .fetch_optional(db)
        .await
}

/// Saves a match recorded by `user_id`. Returns `None` when the id already exists.
pub async fn insert<'e>(
    db: impl PgExecutor<'e>,
    user_id: Uuid,
    m: &NewMatch,
) -> Result<Option<Match>, sqlx::Error> {
    sqlx::query_as::<_, Match>(
        "INSERT INTO matches
           (id, user_id, played_on, player_a, player_b, score_a, score_b, note, log, created_at)
         VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
         ON CONFLICT (id) DO NOTHING
         RETURNING *",
    )
    .bind(m.id)
    .bind(user_id)
    .bind(m.played_on)
    .bind(&m.player_a)
    .bind(&m.player_b)
    .bind(m.score_a)
    .bind(m.score_b)
    .bind(&m.note)
    .bind(m.log.as_ref().map(Json))
    .bind(m.created_at)
    .fetch_optional(db)
    .await
}

/// Inserts many matches in one round trip, recorded by `user_id`.
/// Existing ids are skipped. Returns how many rows were added.
pub async fn insert_many<'e>(
    db: impl PgExecutor<'e>,
    user_id: Uuid,
    matches: &[NewMatch],
) -> Result<u64, sqlx::Error> {
    if matches.is_empty() {
        return Ok(0);
    }
    let result = sqlx::query(
        "INSERT INTO matches
           (id, user_id, played_on, player_a, player_b, score_a, score_b, note, log, created_at)
         SELECT x.id, $1, x.played_on, x.player_a, x.player_b, x.score_a, x.score_b,
                x.note, x.log, x.created_at
         FROM jsonb_to_recordset($2) AS x(
           id UUID, played_on DATE, player_a TEXT, player_b TEXT, score_a INTEGER,
           score_b INTEGER, note TEXT, log JSONB, created_at TIMESTAMPTZ)
         ON CONFLICT (id) DO NOTHING",
    )
    .bind(user_id)
    .bind(Json(matches))
    .execute(db)
    .await?;
    Ok(result.rows_affected())
}

pub async fn update<'e>(
    db: impl PgExecutor<'e>,
    id: Uuid,
    data: &MatchData,
    log: Option<&MatchLog>,
) -> Result<Match, sqlx::Error> {
    sqlx::query_as::<_, Match>(
        "UPDATE matches
         SET played_on = $2, player_a = $3, player_b = $4, score_a = $5, score_b = $6,
             note = $7, log = $8, updated_at = now()
         WHERE id = $1
         RETURNING *",
    )
    .bind(id)
    .bind(data.played_on)
    .bind(&data.player_a)
    .bind(&data.player_b)
    .bind(data.score_a)
    .bind(data.score_b)
    .bind(&data.note)
    .bind(log.map(Json))
    .fetch_one(db)
    .await
}

/// Rewrites every spelling of `name` that differs only in case ("budi" → "Budi")
/// across all matches.
pub async fn rename_player<'e>(db: impl PgExecutor<'e>, name: &str) -> Result<u64, sqlx::Error> {
    let result = sqlx::query(
        "UPDATE matches
         SET player_a = CASE WHEN lower(player_a) = lower($1) THEN $1 ELSE player_a END,
             player_b = CASE WHEN lower(player_b) = lower($1) THEN $1 ELSE player_b END,
             updated_at = now()
         WHERE (lower(player_a) = lower($1) AND player_a <> $1)
            OR (lower(player_b) = lower($1) AND player_b <> $1)",
    )
    .bind(name)
    .execute(db)
    .await?;
    Ok(result.rows_affected())
}

pub async fn delete<'e>(db: impl PgExecutor<'e>, id: Uuid) -> Result<u64, sqlx::Error> {
    let result = sqlx::query("DELETE FROM matches WHERE id = $1")
        .bind(id)
        .execute(db)
        .await?;
    Ok(result.rows_affected())
}
