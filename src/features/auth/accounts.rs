//! Account management for the admin CLI (`cargo run -- user ...`).
//! There is no sign-up endpoint: every account is created here.

use chrono::{Duration, Utc};
use sqlx::PgPool;

use super::repo;
use super::service::hash_password;
use crate::entities::User;
use crate::shared::AppError;
use crate::shared::errors::is_unique_violation;
use crate::shared::validation::{clean_name, require_text};

/// How long a new or renewed account works, unless told otherwise.
pub const DEFAULT_ACCOUNT_DAYS: i64 = 30;

/// No look-alike characters (0/O/o, 1/l/I), so it can be read out or typed on a phone.
const PASSWORD_ALPHABET: &[u8] = b"ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnpqrstuvwxyz23456789";
/// 4 groups of 5 → 20 random characters ≈ 116 bits of entropy.
const PASSWORD_GROUPS: usize = 4;
const PASSWORD_GROUP_LEN: usize = 5;

/// Creates an account with a generated password.
/// Returns the user and the plain password, which is shown once and never stored.
pub async fn create(
    db: &PgPool,
    username: &str,
    display_name: &str,
    days: i64,
) -> Result<(User, String), AppError> {
    let username = normalize_username(username)?;
    let display_name = clean_name(display_name);
    require_text("display name", &display_name, 50)?;
    let days = valid_days(days)?;

    let password = generate_password()?;
    let hash = hash_password(password.clone()).await?;
    let expires_at = Utc::now() + Duration::days(days);
    let user = repo::insert(db, &username, &hash, &display_name, expires_at)
        .await
        .map_err(|err| {
            if is_unique_violation(&err) {
                AppError::Conflict(format!("username {username:?} is already taken"))
            } else {
                err.into()
            }
        })?;
    Ok((user, password))
}

/// Sets the account to expire `days` from now (also revives an expired account).
pub async fn renew(db: &PgPool, username: &str, days: i64) -> Result<User, AppError> {
    let days = valid_days(days)?;
    let expires_at = Utc::now() + Duration::days(days);
    repo::set_expiry(db, &username.trim().to_lowercase(), expires_at)
        .await?
        .ok_or(AppError::NotFound)
}

/// Ends the account now. Open sessions stop working on their next request.
pub async fn expire(db: &PgPool, username: &str) -> Result<User, AppError> {
    repo::set_expiry(db, &username.trim().to_lowercase(), Utc::now())
        .await?
        .ok_or(AppError::NotFound)
}

/// Replaces the password with a new generated one, returned in plain text once.
pub async fn reset_password(db: &PgPool, username: &str) -> Result<(User, String), AppError> {
    let password = generate_password()?;
    let hash = hash_password(password.clone()).await?;
    let user = repo::set_password(db, &username.trim().to_lowercase(), &hash)
        .await?
        .ok_or(AppError::NotFound)?;
    Ok((user, password))
}

pub async fn list(db: &PgPool) -> Result<Vec<User>, AppError> {
    Ok(repo::list(db).await?)
}

/// Lowercase, 3–32 characters: letters, digits, `.`, `_`, `-`; starts with a letter or digit.
fn normalize_username(raw: &str) -> Result<String, AppError> {
    let name = raw.trim().to_lowercase();
    let len_ok = (3..=32).contains(&name.len());
    let chars_ok = name
        .chars()
        .all(|c| c.is_ascii_lowercase() || c.is_ascii_digit() || matches!(c, '.' | '_' | '-'));
    let start_ok = name
        .chars()
        .next()
        .is_some_and(|c| c.is_ascii_alphanumeric());
    if len_ok && chars_ok && start_ok {
        Ok(name)
    } else {
        Err(AppError::Validation(
            "username must be 3-32 characters: a-z, 0-9, '.', '_' or '-', starting with a letter or digit"
                .into(),
        ))
    }
}

fn valid_days(days: i64) -> Result<i64, AppError> {
    if (1..=3650).contains(&days) {
        Ok(days)
    } else {
        Err(AppError::Validation(
            "days must be between 1 and 3650".into(),
        ))
    }
}

/// Random password like `Xk7mP-q2Wd9-...` from the OS random number generator.
pub fn generate_password() -> Result<String, AppError> {
    // Largest multiple of the alphabet size that fits in a byte; bytes at or
    // above it are skipped so every character is equally likely.
    let limit = (256 / PASSWORD_ALPHABET.len() * PASSWORD_ALPHABET.len()) as u8;
    let wanted = PASSWORD_GROUPS * PASSWORD_GROUP_LEN;
    let mut chars = Vec::with_capacity(wanted);
    let mut buf = [0u8; 64];
    while chars.len() < wanted {
        getrandom::fill(&mut buf)
            .map_err(|e| AppError::Internal(format!("random source failed: {e}")))?;
        for &b in buf.iter().filter(|&&b| b < limit) {
            if chars.len() == wanted {
                break;
            }
            chars.push(PASSWORD_ALPHABET[b as usize % PASSWORD_ALPHABET.len()] as char);
        }
    }
    Ok(chars
        .chunks(PASSWORD_GROUP_LEN)
        .map(|g| g.iter().collect::<String>())
        .collect::<Vec<_>>()
        .join("-"))
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn generated_passwords_have_the_right_shape_and_differ() {
        let a = generate_password().unwrap();
        let b = generate_password().unwrap();
        assert_ne!(a, b);
        let groups: Vec<&str> = a.split('-').collect();
        assert_eq!(groups.len(), PASSWORD_GROUPS);
        for g in groups {
            assert_eq!(g.len(), PASSWORD_GROUP_LEN);
            assert!(g.bytes().all(|c| PASSWORD_ALPHABET.contains(&c)));
        }
    }

    #[test]
    fn normalizes_usernames() {
        assert_eq!(normalize_username("  JoshTri ").unwrap(), "joshtri");
        assert_eq!(normalize_username("jeremy.p_2-x").unwrap(), "jeremy.p_2-x");
        for bad in ["", "ab", "-abc", "has space", "émile", &"a".repeat(33)] {
            assert!(
                normalize_username(bad).is_err(),
                "{bad:?} should be rejected"
            );
        }
    }

    #[test]
    fn checks_days() {
        assert!(valid_days(30).is_ok());
        assert!(valid_days(0).is_err());
        assert!(valid_days(-5).is_err());
    }
}
