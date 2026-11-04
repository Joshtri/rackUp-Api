use serde::{Deserialize, Serialize};
use serde_json::Value;
use uuid::Uuid;

use crate::entities::LiveSession;
use crate::features::matches::dto::{CreateMatchRequest, MatchResponse};
use crate::shared::AppError;

/// A session's state is the whole tap log, so it can grow during a long
/// evening; this is far above what a real session reaches.
const STATE_MAX_BYTES: usize = 2_000_000;

/// Body of `POST /live-sessions`. The client picks the id, so a retried
/// create is harmless.
#[derive(Debug, Deserialize)]
pub struct CreateLiveSessionRequest {
    pub id: Uuid,
    pub state: Value,
}

/// Body of `PUT /live-sessions/{id}`.
#[derive(Debug, Deserialize)]
pub struct UpdateLiveSessionRequest {
    /// The version this change was based on. A different current version → 409.
    pub version: i32,
    pub state: Value,
}

/// Body of `POST /live-sessions/{id}/end`.
#[derive(Debug, Deserialize)]
pub struct EndLiveSessionRequest {
    /// The result to save in the history, or `null` to discard the session.
    /// Its id is replaced by the session id, so ending twice saves one match.
    #[serde(default, rename = "match")]
    pub save: Option<CreateMatchRequest>,
}

/// Query of `GET /live-sessions/{id}`.
#[derive(Debug, Deserialize)]
pub struct SinceQuery {
    /// The version the client already has. If nothing changed, `session` is left out.
    pub since: Option<i32>,
}

#[derive(Debug, Serialize)]
#[serde(rename_all = "camelCase")]
pub struct LiveSessionResponse {
    pub id: Uuid,
    pub state: Value,
    pub version: i32,
    pub created_by: Option<Uuid>,
    /// Epoch ms, only once the session has ended.
    pub ended_at: Option<i64>,
    pub ended_by: Option<Uuid>,
    pub ended_by_name: Option<String>,
    /// The match saved when the session ended, if it was saved.
    pub saved_match_id: Option<Uuid>,
    pub created_at: i64,
    pub updated_at: i64,
}

impl From<LiveSession> for LiveSessionResponse {
    fn from(s: LiveSession) -> Self {
        Self {
            id: s.id,
            state: s.state.0,
            version: s.version,
            created_by: s.created_by,
            ended_at: s.ended_at.map(|t| t.timestamp_millis()),
            ended_by: s.ended_by,
            ended_by_name: s.ended_by_name,
            saved_match_id: s.saved_match_id,
            created_at: s.created_at.timestamp_millis(),
            updated_at: s.updated_at.timestamp_millis(),
        }
    }
}

// Every response carries the server's clock, so all phones can count the shot
// clock down from the same time even when their own clocks differ.

#[derive(Debug, Serialize)]
#[serde(rename_all = "camelCase")]
pub struct OneResponse {
    pub server_now: i64,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub session: Option<LiveSessionResponse>,
}

#[derive(Debug, Serialize)]
#[serde(rename_all = "camelCase")]
pub struct ListResponse {
    pub server_now: i64,
    pub sessions: Vec<LiveSessionResponse>,
}

#[derive(Debug, Serialize)]
#[serde(rename_all = "camelCase")]
pub struct EndResponse {
    pub server_now: i64,
    pub session: LiveSessionResponse,
    #[serde(rename = "match", skip_serializing_if = "Option::is_none")]
    pub saved: Option<MatchResponse>,
}

/// The state must look like a frontend `Session` and stay within size.
pub fn validate_state(state: &Value) -> Result<(), AppError> {
    let obj = state
        .as_object()
        .ok_or_else(|| AppError::Validation("state must be an object".into()))?;
    for key in ["playerA", "playerB"] {
        if !obj.get(key).is_some_and(Value::is_string) {
            return Err(AppError::Validation(format!(
                "state.{key} must be a string"
            )));
        }
    }
    if !obj.get("clock").is_some_and(Value::is_object) {
        return Err(AppError::Validation("state.clock must be an object".into()));
    }
    let size = serde_json::to_vec(state)
        .map(|v| v.len())
        .unwrap_or(usize::MAX);
    if size > STATE_MAX_BYTES {
        return Err(AppError::Validation("state is too large".into()));
    }
    Ok(())
}

#[cfg(test)]
mod tests {
    use super::*;
    use serde_json::json;

    #[test]
    fn accepts_a_session_state() {
        let state = json!({ "playerA": "Budi", "playerB": "Andi", "clock": { "running": false } });
        assert!(validate_state(&state).is_ok());
    }

    #[test]
    fn rejects_other_shapes() {
        assert!(validate_state(&json!([])).is_err());
        assert!(validate_state(&json!({ "playerA": "Budi", "clock": {} })).is_err());
        assert!(validate_state(&json!({ "playerA": "Budi", "playerB": 3, "clock": {} })).is_err());
        assert!(validate_state(&json!({ "playerA": "Budi", "playerB": "Andi" })).is_err());
    }

    #[test]
    fn rejects_huge_state() {
        let big = "x".repeat(STATE_MAX_BYTES);
        let state = json!({ "playerA": "Budi", "playerB": "Andi", "clock": {}, "pad": big });
        assert!(validate_state(&state).is_err());
    }

    #[test]
    fn end_request_reads_match_or_null() {
        let discard: EndLiveSessionRequest =
            serde_json::from_value(json!({ "match": null })).unwrap();
        assert!(discard.save.is_none());
        let empty: EndLiveSessionRequest = serde_json::from_value(json!({})).unwrap();
        assert!(empty.save.is_none());
        let save: EndLiveSessionRequest = serde_json::from_value(json!({
            "match": { "date": "2026-11-04", "playerA": "Budi", "playerB": "Andi", "scoreA": 3, "scoreB": 1 }
        }))
        .unwrap();
        assert_eq!(save.save.unwrap().fields.score_a, 3);
    }
}
