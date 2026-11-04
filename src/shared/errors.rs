use axum::Json;
use axum::extract::rejection::{JsonRejection, PathRejection};
use axum::http::StatusCode;
use axum::response::{IntoResponse, Response};
use serde_json::json;

/// Every handler error. Turned into `{"error": {"code", "message"}}`.
#[derive(Debug, thiserror::Error)]
pub enum AppError {
    #[error("{0}")]
    Validation(String),
    /// Malformed request (bad JSON, bad path param, wrong content type).
    #[error("{1}")]
    BadRequest(StatusCode, String),
    #[error("login required")]
    Unauthorized,
    #[error("wrong username or password")]
    InvalidCredentials,
    #[error("account has expired")]
    AccountExpired,
    #[error("not found")]
    NotFound,
    #[error("{0}")]
    Conflict(String),
    #[error(transparent)]
    Database(#[from] sqlx::Error),
    #[error("{0}")]
    Internal(String),
}

impl AppError {
    fn status_and_code(&self) -> (StatusCode, &'static str) {
        match self {
            Self::Validation(_) => (StatusCode::UNPROCESSABLE_ENTITY, "validation_error"),
            Self::BadRequest(status, _) => (*status, "bad_request"),
            Self::Unauthorized => (StatusCode::UNAUTHORIZED, "unauthorized"),
            Self::InvalidCredentials => (StatusCode::UNAUTHORIZED, "invalid_credentials"),
            Self::AccountExpired => (StatusCode::UNAUTHORIZED, "account_expired"),
            Self::NotFound => (StatusCode::NOT_FOUND, "not_found"),
            Self::Conflict(_) => (StatusCode::CONFLICT, "conflict"),
            Self::Database(_) | Self::Internal(_) => {
                (StatusCode::INTERNAL_SERVER_ERROR, "internal_error")
            }
        }
    }
}

impl IntoResponse for AppError {
    fn into_response(self) -> Response {
        let (status, code) = self.status_and_code();
        let message = if status.is_server_error() {
            // Log the details, don't send them to the client.
            tracing::error!(error = %self, "request failed");
            "something went wrong".to_string()
        } else {
            self.to_string()
        };
        (
            status,
            Json(json!({ "error": { "code": code, "message": message } })),
        )
            .into_response()
    }
}

impl From<JsonRejection> for AppError {
    fn from(rejection: JsonRejection) -> Self {
        Self::BadRequest(rejection.status(), rejection.body_text())
    }
}

impl From<PathRejection> for AppError {
    fn from(rejection: PathRejection) -> Self {
        Self::BadRequest(rejection.status(), rejection.body_text())
    }
}

/// True when the error is a unique constraint violation.
pub fn is_unique_violation(err: &sqlx::Error) -> bool {
    matches!(err, sqlx::Error::Database(e) if e.is_unique_violation())
}
