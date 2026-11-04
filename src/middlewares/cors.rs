use std::time::Duration;

use axum::http::header::{AUTHORIZATION, CONTENT_TYPE};
use axum::http::{HeaderValue, Method};
use tower_http::cors::{AllowOrigin, CorsLayer};

use crate::config::Settings;

/// Lets the frontend (another domain) call the API.
/// Auth uses a bearer token, not cookies, so no credentials mode is needed.
pub fn layer(settings: &Settings) -> CorsLayer {
    let origin = if settings.cors_origins.iter().any(|o| o == "*") {
        AllowOrigin::any()
    } else {
        let list: Vec<HeaderValue> = settings
            .cors_origins
            .iter()
            .filter_map(|o| match o.parse() {
                Ok(v) => Some(v),
                Err(_) => {
                    tracing::warn!(origin = %o, "ignoring invalid CORS origin");
                    None
                }
            })
            .collect();
        AllowOrigin::list(list)
    };

    CorsLayer::new()
        .allow_origin(origin)
        .allow_methods([Method::GET, Method::POST, Method::PUT, Method::DELETE])
        .allow_headers([AUTHORIZATION, CONTENT_TYPE])
        .max_age(Duration::from_secs(3600))
}
