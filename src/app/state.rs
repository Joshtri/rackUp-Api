use std::sync::Arc;

use sqlx::PgPool;

use crate::config::Settings;

/// Shared by every request. Cheap to clone.
#[derive(Clone)]
pub struct AppState {
    pub db: PgPool,
    pub settings: Arc<Settings>,
}

impl AppState {
    pub fn new(db: PgPool, settings: Settings) -> Self {
        Self {
            db,
            settings: Arc::new(settings),
        }
    }
}
