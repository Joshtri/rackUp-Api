use axum::extract::State;
use axum::http::StatusCode;
use axum::routing::get;
use axum::{Json, Router};
use serde_json::{Value, json};

use super::state::AppState;

pub fn routes() -> Router<AppState> {
    Router::new()
        .route("/health", get(health))
        .route("/ready", get(ready))
}

/// The process is up. Doesn't touch the database.
async fn health() -> Json<Value> {
    Json(json!({ "status": "ok" }))
}

/// The database is reachable.
async fn ready(State(state): State<AppState>) -> (StatusCode, Json<Value>) {
    match sqlx::query("SELECT 1").execute(&state.db).await {
        Ok(_) => (StatusCode::OK, Json(json!({ "status": "ready" }))),
        Err(err) => {
            tracing::warn!(error = %err, "database not reachable");
            (
                StatusCode::SERVICE_UNAVAILABLE,
                Json(json!({ "status": "unavailable" })),
            )
        }
    }
}
