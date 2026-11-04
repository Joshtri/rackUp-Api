use chrono::{DateTime, NaiveDate, Utc};
use serde::{Deserialize, Serialize};
use sqlx::types::Json;
use uuid::Uuid;

/// Row of the `matches` table.
#[derive(Debug, Clone, sqlx::FromRow)]
pub struct Match {
    pub id: Uuid,
    pub user_id: Uuid,
    pub played_on: NaiveDate,
    pub player_a: String,
    pub player_b: String,
    pub score_a: i32,
    pub score_b: i32,
    pub note: Option<String>,
    pub log: Option<Json<MatchLog>>,
    pub created_at: DateTime<Utc>,
    pub updated_at: DateTime<Utc>,
}

// The types below mirror `SessionLog` / `MatchLog` in the frontend
// (src/lib/session.ts and src/lib/matches.ts). Keep the JSON shape in sync.

#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
pub enum Side {
    A,
    B,
}

impl Side {
    pub fn flip(self) -> Self {
        match self {
            Self::A => Self::B,
            Self::B => Self::A,
        }
    }
}

/// A value per player side, serialized as `{"A": .., "B": ..}`.
#[derive(Debug, Clone, Copy, PartialEq, Serialize, Deserialize)]
pub struct PerSide<T> {
    #[serde(rename = "A")]
    pub a: T,
    #[serde(rename = "B")]
    pub b: T,
}

impl<T> PerSide<T> {
    pub fn flipped(self) -> Self {
        Self {
            a: self.b,
            b: self.a,
        }
    }
}

#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "lowercase")]
pub enum ShotEnd {
    Continue,
    Turnover,
    Rack,
}

#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
pub struct Shot {
    pub side: Side,
    /// Shot clock time used, in ms.
    pub ms: f64,
    pub end: ShotEnd,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "lowercase")]
pub enum RackFinal {
    Added,
    Marked,
}

#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct RackLog {
    pub winner: Side,
    pub started_at: i64,
    pub ended_at: i64,
    pub shot_end: i64,
    #[serde(rename = "final")]
    pub final_shot: Option<RackFinal>,
}

/// Tap log of a live shot-clock session, kept for the stats.
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct MatchLog {
    pub shots: Vec<Shot>,
    pub racks: Vec<RackLog>,
    pub extensions: PerSide<i64>,
    pub timeouts: PerSide<i64>,
    pub started_at: i64,
    pub ended_at: i64,
}

impl MatchLog {
    /// Same log seen from the other side. Used when players A and B swap.
    pub fn flipped(self) -> Self {
        Self {
            shots: self
                .shots
                .into_iter()
                .map(|s| Shot {
                    side: s.side.flip(),
                    ..s
                })
                .collect(),
            racks: self
                .racks
                .into_iter()
                .map(|r| RackLog {
                    winner: r.winner.flip(),
                    ..r
                })
                .collect(),
            extensions: self.extensions.flipped(),
            timeouts: self.timeouts.flipped(),
            ..self
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use serde_json::json;

    fn sample() -> serde_json::Value {
        json!({
            "shots": [
                { "side": "A", "ms": 12000, "end": "continue" },
                { "side": "A", "ms": 8000, "end": "rack" }
            ],
            "racks": [
                { "winner": "A", "startedAt": 1000, "endedAt": 9000, "shotEnd": 2, "final": null }
            ],
            "extensions": { "A": 1, "B": 0 },
            "timeouts": { "A": 0, "B": 2 },
            "startedAt": 1000,
            "endedAt": 20000
        })
    }

    #[test]
    fn reads_frontend_json() {
        let log: MatchLog = serde_json::from_value(sample()).unwrap();
        assert_eq!(log.shots.len(), 2);
        assert_eq!(log.racks[0].final_shot, None);
        assert_eq!(log.extensions.a, 1);
        // Writes back the same keys the frontend expects.
        let back = serde_json::to_value(&log).unwrap();
        assert_eq!(back["racks"][0]["shotEnd"], 2);
        assert_eq!(back["timeouts"]["B"], 2);
    }

    #[test]
    fn flips_every_side() {
        let log: MatchLog = serde_json::from_value(sample()).unwrap();
        let flipped = log.clone().flipped();
        assert!(flipped.shots.iter().all(|s| s.side == Side::B));
        assert_eq!(flipped.racks[0].winner, Side::B);
        assert_eq!(flipped.extensions, PerSide { a: 0, b: 1 });
        assert_eq!(flipped.timeouts, PerSide { a: 2, b: 0 });
        assert_eq!(flipped.started_at, log.started_at);
        assert_eq!(flipped.flipped(), log);
    }

    #[test]
    fn rejects_unknown_side() {
        let mut bad = sample();
        bad["shots"][0]["side"] = json!("C");
        assert!(serde_json::from_value::<MatchLog>(bad).is_err());
    }
}
