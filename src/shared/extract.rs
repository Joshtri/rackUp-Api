//! Axum's `Json` and `Path`, but rejections come back as our JSON error format.

use axum::extract::{FromRequest, FromRequestParts};

use super::AppError;

#[derive(FromRequest)]
#[from_request(via(axum::Json), rejection(AppError))]
pub struct AppJson<T>(pub T);

#[derive(FromRequestParts)]
#[from_request(via(axum::extract::Path), rejection(AppError))]
pub struct AppPath<T>(pub T);
