use tracing_subscriber::EnvFilter;

/// Logs go to stdout, which Vercel collects. Level comes from `RUST_LOG`.
pub fn init() {
    let filter =
        EnvFilter::try_from_default_env().unwrap_or_else(|_| EnvFilter::new("info,sqlx=warn"));
    let _ = tracing_subscriber::fmt()
        .with_env_filter(filter)
        .with_target(false)
        // No color codes in Vercel's log viewer.
        .with_ansi(std::env::var_os("VERCEL").is_none())
        .try_init();
}
