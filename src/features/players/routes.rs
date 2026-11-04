use axum::Router;
use axum::routing::get;

use super::handlers;
use crate::app::state::AppState;

pub fn routes() -> Router<AppState> {
    Router::new().route("/players", get(handlers::list))
}
