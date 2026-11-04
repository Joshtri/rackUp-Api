use axum::Router;
use axum::routing::{get, post};

use super::handlers;
use crate::app::state::AppState;

pub fn routes() -> Router<AppState> {
    Router::new()
        .route("/live-sessions", get(handlers::list).post(handlers::create))
        .route(
            "/live-sessions/{id}",
            get(handlers::get).put(handlers::update),
        )
        .route("/live-sessions/{id}/end", post(handlers::end))
}
