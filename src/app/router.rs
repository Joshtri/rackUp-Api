use axum::Router;
use axum::extract::DefaultBodyLimit;
use tower_http::trace::TraceLayer;

use super::health;
use super::state::AppState;
use crate::features;
use crate::middlewares::cors;
use crate::shared::AppError;

/// Vercel caps request bodies at 4.5 MB, so a larger limit here would never be reached.
const BODY_LIMIT_BYTES: usize = 4_500_000;

pub fn build(state: AppState) -> Router {
    Router::new()
        .merge(health::routes())
        .merge(features::routes())
        .fallback(|| async { AppError::NotFound })
        .layer(DefaultBodyLimit::max(BODY_LIMIT_BYTES))
        .layer(cors::layer(&state.settings))
        .layer(TraceLayer::new_for_http())
        .with_state(state)
}
