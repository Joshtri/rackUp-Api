use std::env;
use std::str::FromStr;

/// App settings, read from environment variables.
/// No `Debug` derive on purpose: it holds secrets.
#[derive(Clone)]
pub struct Settings {
    /// Neon pooled connection string (host contains `-pooler`).
    pub database_url: String,
    pub db_max_connections: u32,
    pub jwt_secret: String,
    pub jwt_ttl_days: i64,
    /// Allowed browser origins. `*` allows any.
    pub cors_origins: Vec<String>,
    /// Only used by the local server; Vercel picks its own port.
    pub port: u16,
}

#[derive(Debug, thiserror::Error)]
pub enum ConfigError {
    #[error("missing environment variable {0}")]
    Missing(&'static str),
    #[error("invalid environment variable {0}: {1}")]
    Invalid(&'static str, String),
}

impl Settings {
    pub fn from_env() -> Result<Self, ConfigError> {
        let jwt_secret = required("JWT_SECRET")?;
        if jwt_secret.len() < 32 {
            return Err(ConfigError::Invalid(
                "JWT_SECRET",
                "must be at least 32 characters".into(),
            ));
        }
        let jwt_ttl_days = parse_or("JWT_TTL_DAYS", 30)?;
        if jwt_ttl_days < 1 {
            return Err(ConfigError::Invalid(
                "JWT_TTL_DAYS",
                "must be 1 or more".into(),
            ));
        }

        Ok(Self {
            database_url: required("DATABASE_URL")?,
            db_max_connections: parse_or("DB_MAX_CONNECTIONS", 5)?,
            jwt_secret,
            jwt_ttl_days,
            cors_origins: parse_origins(
                &env::var("CORS_ORIGINS").unwrap_or_else(|_| "http://localhost:5173".into()),
            ),
            port: parse_or("PORT", 8080)?,
        })
    }
}

fn required(key: &'static str) -> Result<String, ConfigError> {
    env::var(key)
        .ok()
        .filter(|v| !v.trim().is_empty())
        .ok_or(ConfigError::Missing(key))
}

fn parse_or<T: FromStr>(key: &'static str, default: T) -> Result<T, ConfigError> {
    match env::var(key) {
        Ok(v) if !v.trim().is_empty() => v
            .trim()
            .parse()
            .map_err(|_| ConfigError::Invalid(key, format!("could not parse {v:?}"))),
        _ => Ok(default),
    }
}

fn parse_origins(raw: &str) -> Vec<String> {
    raw.split(',')
        .map(|o| o.trim().trim_end_matches('/').to_string())
        .filter(|o| !o.is_empty())
        .collect()
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn origins_are_trimmed() {
        assert_eq!(
            parse_origins(" http://localhost:5173/ ,https://rackup.app,, "),
            vec!["http://localhost:5173", "https://rackup.app"]
        );
    }
}
