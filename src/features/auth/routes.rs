use axum::Router;
use axum::routing::{get, post};

use super::handlers;
use crate::app::state::AppState;

/// No sign-up route: accounts are created with `cargo run -- user create`.
pub fn routes() -> Router<AppState> {
    Router::new()
        .route("/auth/login", post(handlers::login))
        .route("/auth/me", get(handlers::me))
        .route("/users", get(handlers::users))
}
