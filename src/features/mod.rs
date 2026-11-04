use axum::Router;

use crate::app::state::AppState;

pub mod auth;
pub mod live_sessions;
pub mod matches;
pub mod players;

pub fn routes() -> Router<AppState> {
    Router::new()
        .merge(auth::routes::routes())
        .merge(live_sessions::routes::routes())
        .merge(matches::routes::routes())
        .merge(players::routes::routes())
}
