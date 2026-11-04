use axum::Router;

use crate::app::{router, state::AppState};
use crate::config::Settings;
use crate::db;

/// Builds the full app. Shared by the local server and the Vercel function.
pub fn build_app(settings: Settings) -> Result<Router, sqlx::Error> {
    let pool = db::connection::create_pool(&settings)?;
    Ok(router::build(AppState::new(pool, settings)))
}
