use chrono::{DateTime, Utc};
use serde_json::Value;
use sqlx::types::Json;
use uuid::Uuid;

/// Row of the `live_sessions` table, plus the name of whoever ended it.
#[derive(Debug, Clone, sqlx::FromRow)]
pub struct LiveSession {
    pub id: Uuid,
    pub created_by: Option<Uuid>,
    /// The frontend's `Session` object (src/lib/session.ts), stored as-is.
    pub state: Json<Value>,
    /// Goes up by one on every change.
    pub version: i32,
    pub ended_at: Option<DateTime<Utc>>,
    pub ended_by: Option<Uuid>,
    pub ended_by_name: Option<String>,
    pub saved_match_id: Option<Uuid>,
    pub created_at: DateTime<Utc>,
    pub updated_at: DateTime<Utc>,
}
