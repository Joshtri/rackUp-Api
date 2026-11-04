use sqlx::migrate::Migrator;
use sqlx::postgres::PgPoolOptions;

/// SQL files in `./migrations`, embedded at compile time.
pub static MIGRATOR: Migrator = sqlx::migrate!();

/// Applies pending migrations.
///
/// Prefers `DATABASE_URL_UNPOOLED`: sqlx takes a session-level lock while
/// migrating, which Neon's pooler (PgBouncer in transaction mode) doesn't support.
pub async fn run_from_env() -> Result<(), Box<dyn std::error::Error + Send + Sync>> {
    let url = std::env::var("DATABASE_URL_UNPOOLED")
        .or_else(|_| std::env::var("DATABASE_URL"))
        .map_err(|_| "set DATABASE_URL_UNPOOLED (or DATABASE_URL) to run migrations")?;
    let pool = PgPoolOptions::new()
        .max_connections(1)
        .connect(&url)
        .await?;
    MIGRATOR.run(&pool).await?;
    pool.close().await;
    Ok(())
}
