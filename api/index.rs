//! Vercel function entrypoint. vercel.json rewrites every path here,
//! so the Axum router still sees the original URL (`/matches`, `/auth/login`, ...).

use rackup_api::config::{Settings, logging};
use rackup_api::startup;
use tower::ServiceBuilder;
use vercel_runtime::Error;
use vercel_runtime::axum::VercelLayer;

#[tokio::main]
async fn main() -> Result<(), Error> {
    logging::init();
    // Log startup failures before exiting. Otherwise Vercel only shows a bare 500
    // (INTERNAL_FUNCTION_INVOCATION_FAILED) with no reason.
    let settings = Settings::from_env().inspect_err(|e| {
        tracing::error!("startup failed: {e}. Check the project's Environment Variables on Vercel.");
    })?;
    // Built once per instance, so the DB pool is reused across requests.
    let app = startup::build_app(settings).inspect_err(|e| {
        tracing::error!("startup failed: could not set up the database pool: {e}");
    })?;
    let service = ServiceBuilder::new().layer(VercelLayer::new()).service(app);
    vercel_runtime::run(service).await
}
