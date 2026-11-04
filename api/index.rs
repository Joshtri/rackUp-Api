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
    let settings = Settings::from_env()?;
    // Built once per instance, so the DB pool is reused across requests.
    let app = startup::build_app(settings)?;
    let service = ServiceBuilder::new().layer(VercelLayer::new()).service(app);
    vercel_runtime::run(service).await
}
