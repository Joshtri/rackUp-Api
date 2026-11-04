use axum::Json;
use axum::extract::State;

use super::repo;
use crate::app::state::AppState;
use crate::middlewares::AuthUser;
use crate::shared::AppError;

/// Names, most recently played first. Shared by all accounts.
pub async fn list(
    State(state): State<AppState>,
    _user: AuthUser,
) -> Result<Json<Vec<String>>, AppError> {
    Ok(Json(repo::list(&state.db).await?))
}
