//! Local entrypoint.
//!
//! - `cargo run`                starts the API on http://localhost:$PORT (default 8080)
//! - `cargo run -- migrate`     applies the SQL migrations and exits
//! - `cargo run -- user ...`    manages accounts (see `src/cli.rs`)

use std::net::SocketAddr;

use rackup_api::config::{Settings, logging};
use rackup_api::{cli, db, startup};

type BoxError = Box<dyn std::error::Error + Send + Sync>;

#[tokio::main]
async fn main() -> Result<(), BoxError> {
    dotenvy::dotenv().ok();
    logging::init();

    let args: Vec<String> = std::env::args().skip(1).collect();
    match args.first().map(String::as_str) {
        Some("migrate") => {
            db::migrations::run_from_env().await?;
            tracing::info!("migrations applied");
            return Ok(());
        }
        Some("user") => return cli::run_user_command(&args[1..]).await,
        _ => {}
    }

    let settings = Settings::from_env()?;
    let addr = SocketAddr::from(([0, 0, 0, 0], settings.port));
    let app = startup::build_app(settings)?;

    let listener = tokio::net::TcpListener::bind(addr).await?;
    tracing::info!("listening on http://localhost:{}", addr.port());
    axum::serve(listener, app)
        .with_graceful_shutdown(async {
            let _ = tokio::signal::ctrl_c().await;
        })
        .await?;
    Ok(())
}
