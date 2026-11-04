use chrono::NaiveDate;
use serde::{Deserialize, Serialize};
use uuid::Uuid;

use crate::entities::{Match, MatchLog};
use crate::shared::AppError;
use crate::shared::validation::{clean_name, require_text, same_name};

const NAME_MAX: usize = 50;
const NOTE_MAX: usize = 500;
const SCORE_MAX: i32 = 999;

/// Editable fields. Body of `PUT /matches/{id}`.
#[derive(Debug, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct MatchFields {
    /// `YYYY-MM-DD`
    pub date: NaiveDate,
    pub player_a: String,
    pub player_b: String,
    pub score_a: i32,
    pub score_b: i32,
    #[serde(default)]
    pub note: Option<String>,
}

/// Body of `POST /matches`, and each item of an import.
#[derive(Debug, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct CreateMatchRequest {
    /// Optional client id, so a deleted match can be restored with the same id.
    #[serde(default)]
    pub id: Option<Uuid>,
    /// Epoch ms. Defaults to now.
    #[serde(default)]
    pub created_at: Option<i64>,
    /// Only for matches played with the shot clock.
    #[serde(default)]
    pub log: Option<MatchLog>,
    #[serde(flatten)]
    pub fields: MatchFields,
}

#[derive(Debug, Deserialize)]
pub struct ImportRequest {
    pub matches: Vec<CreateMatchRequest>,
}

#[derive(Debug, Serialize)]
pub struct ImportResponse {
    /// New matches saved.
    pub added: u64,
    /// Matches whose id already existed.
    pub skipped: u64,
}

/// Same shape as `Match` in the frontend (src/lib/matches.ts), plus `updatedAt`.
#[derive(Debug, Serialize)]
#[serde(rename_all = "camelCase")]
pub struct MatchResponse {
    pub id: Uuid,
    pub date: NaiveDate,
    pub player_a: String,
    pub player_b: String,
    pub score_a: i32,
    pub score_b: i32,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub note: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub log: Option<MatchLog>,
    pub created_at: i64,
    pub updated_at: i64,
}

impl From<Match> for MatchResponse {
    fn from(m: Match) -> Self {
        Self {
            id: m.id,
            date: m.played_on,
            player_a: m.player_a,
            player_b: m.player_b,
            score_a: m.score_a,
            score_b: m.score_b,
            note: m.note,
            log: m.log.map(|l| l.0),
            created_at: m.created_at.timestamp_millis(),
            updated_at: m.updated_at.timestamp_millis(),
        }
    }
}

/// `MatchFields` after validation and cleanup.
#[derive(Debug, Clone)]
pub struct MatchData {
    pub played_on: NaiveDate,
    pub player_a: String,
    pub player_b: String,
    pub score_a: i32,
    pub score_b: i32,
    pub note: Option<String>,
}

impl MatchFields {
    pub fn validate(self) -> Result<MatchData, AppError> {
        let player_a = clean_name(&self.player_a);
        let player_b = clean_name(&self.player_b);
        require_text("playerA", &player_a, NAME_MAX)?;
        require_text("playerB", &player_b, NAME_MAX)?;
        if same_name(&player_a, &player_b) {
            return Err(AppError::Validation(
                "playerA and playerB must be different".into(),
            ));
        }
        for (field, score) in [("scoreA", self.score_a), ("scoreB", self.score_b)] {
            if !(0..=SCORE_MAX).contains(&score) {
                return Err(AppError::Validation(format!(
                    "{field} must be between 0 and {SCORE_MAX}"
                )));
            }
        }
        let note = self
            .note
            .map(|n| n.trim().to_string())
            .filter(|n| !n.is_empty());
        if let Some(n) = &note
            && n.chars().count() > NOTE_MAX
        {
            return Err(AppError::Validation(format!(
                "note must be at most {NOTE_MAX} characters"
            )));
        }
        Ok(MatchData {
            played_on: self.date,
            player_a,
            player_b,
            score_a: self.score_a,
            score_b: self.score_b,
            note,
        })
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use serde_json::json;

    fn fields(a: &str, b: &str) -> MatchFields {
        serde_json::from_value(json!({
            "date": "2026-10-31", "playerA": a, "playerB": b, "scoreA": 6, "scoreB": 8
        }))
        .unwrap()
    }

    #[test]
    fn cleans_names_and_note() {
        let mut f = fields("  Budi  ", "Andi");
        f.note = Some("   ".into());
        let data = f.validate().unwrap();
        assert_eq!(data.player_a, "Budi");
        assert_eq!(data.note, None);
    }

    #[test]
    fn rejects_bad_input() {
        assert!(fields("Budi", "budi").validate().is_err());
        assert!(fields("", "Andi").validate().is_err());
        let mut f = fields("Budi", "Andi");
        f.score_a = -1;
        assert!(f.validate().is_err());
    }

    #[test]
    fn parses_create_request_from_frontend_shape() {
        let req: CreateMatchRequest = serde_json::from_value(json!({
            "id": "6f1c1b2e-6b8a-4c55-9a0e-2f4f3c1d9b7a",
            "date": "2026-10-31", "playerA": "Budi", "playerB": "Andi",
            "scoreA": 6, "scoreB": 8, "note": "seru", "createdAt": 1793433600000_i64
        }))
        .unwrap();
        assert!(req.id.is_some());
        assert_eq!(req.created_at, Some(1793433600000));
        assert_eq!(req.fields.score_b, 8);
        assert!(req.log.is_none());

        let bad_date = serde_json::from_value::<CreateMatchRequest>(json!({
            "date": "31-10-2026", "playerA": "Budi", "playerB": "Andi", "scoreA": 1, "scoreB": 2
        }));
        assert!(bad_date.is_err());
    }
}
