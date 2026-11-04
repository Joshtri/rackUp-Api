use axum::Router;
use axum::routing::{get, post};

use super::handlers;
use crate::app::state::AppState;

pub fn routes() -> Router<AppState> {
    Router::new()
        .route("/matches", get(handlers::list).post(handlers::create))
        .route("/matches/import", post(handlers::import))
        .route(
            "/matches/{id}",
            get(handlers::get)
                .put(handlers::update)
                .delete(handlers::delete),
        )
}
