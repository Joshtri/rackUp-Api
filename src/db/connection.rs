use std::str::FromStr;
use std::time::Duration;

use sqlx::PgPool;
use sqlx::postgres::{PgConnectOptions, PgPoolOptions};

use crate::config::Settings;

/// Lazy pool: no connection opens until the first query,
/// so a cold start doesn't wait on the database.
pub fn create_pool(settings: &Settings) -> Result<PgPool, sqlx::Error> {
    let options = PgConnectOptions::from_str(&settings.database_url)?;
    Ok(PgPoolOptions::new()
        .max_connections(settings.db_max_connections)
        .min_connections(0)
        // Neon can take a few seconds to wake a suspended compute.
        .acquire_timeout(Duration::from_secs(10))
        .idle_timeout(Duration::from_secs(60))
        .connect_lazy_with(options))
}
