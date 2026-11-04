//! Admin commands, run from a trusted machine with database access:
//!
//! ```text
//! cargo run -- user create <username> [--name "Display Name"] [--days 30]
//! cargo run -- user renew <username> [--days 30]
//! cargo run -- user expire <username>
//! cargo run -- user reset-password <username>
//! cargo run -- user list
//! ```
//!
//! Generated passwords are printed once and only their Argon2id hash is stored.

use chrono::{DateTime, Utc};
use sqlx::PgPool;
use sqlx::postgres::PgPoolOptions;

use crate::entities::User;
use crate::features::auth::accounts::{self, DEFAULT_ACCOUNT_DAYS};

type BoxError = Box<dyn std::error::Error + Send + Sync>;

const USAGE: &str = "usage:
  user create <username> [--name \"Display Name\"] [--days 30]
  user renew <username> [--days 30]
  user expire <username>
  user reset-password <username>
  user list";

/// Runs `user ...` commands. `args` excludes the program name and the word `user`.
pub async fn run_user_command(args: &[String]) -> Result<(), BoxError> {
    let (cmd, rest) = args.split_first().ok_or(USAGE)?;
    let db = connect().await?;
    let result = match cmd.as_str() {
        "create" => {
            let username = positional(rest)?;
            let name = flag(rest, "--name").unwrap_or(username);
            let days = days_flag(rest)?;
            let (user, password) = accounts::create(&db, username, name, days).await?;
            println!("created {}", describe(&user));
            print_password(&password);
            Ok(())
        }
        "renew" => {
            let user = accounts::renew(&db, positional(rest)?, days_flag(rest)?).await?;
            println!("renewed {}", describe(&user));
            Ok(())
        }
        "expire" => {
            let user = accounts::expire(&db, positional(rest)?).await?;
            println!("expired {}", describe(&user));
            Ok(())
        }
        "reset-password" => {
            let (user, password) = accounts::reset_password(&db, positional(rest)?).await?;
            println!("new password for {}", describe(&user));
            print_password(&password);
            Ok(())
        }
        "list" => {
            let users = accounts::list(&db).await?;
            if users.is_empty() {
                println!("no users");
            }
            for user in users {
                println!("{}", describe(&user));
            }
            Ok(())
        }
        _ => Err(USAGE.into()),
    };
    db.close().await;
    result
}

async fn connect() -> Result<PgPool, BoxError> {
    let url = std::env::var("DATABASE_URL_UNPOOLED")
        .or_else(|_| std::env::var("DATABASE_URL"))
        .map_err(|_| "set DATABASE_URL_UNPOOLED (or DATABASE_URL)")?;
    Ok(PgPoolOptions::new().max_connections(1).connect(&url).await?)
}

fn positional(rest: &[String]) -> Result<&str, BoxError> {
    rest.first()
        .filter(|a| !a.starts_with("--"))
        .map(String::as_str)
        .ok_or_else(|| USAGE.into())
}

fn flag<'a>(rest: &'a [String], name: &str) -> Option<&'a str> {
    rest.iter()
        .position(|a| a == name)
        .and_then(|i| rest.get(i + 1))
        .map(String::as_str)
}

fn days_flag(rest: &[String]) -> Result<i64, BoxError> {
    match flag(rest, "--days") {
        Some(v) => Ok(v.parse().map_err(|_| format!("--days: not a number: {v:?}"))?),
        None => Ok(DEFAULT_ACCOUNT_DAYS),
    }
}

fn describe(user: &User) -> String {
    let status = if user.is_expired() { "EXPIRED" } else { "active" };
    format!(
        "{} ({}) - {status}, expires {}",
        user.username,
        user.display_name,
        fmt_time(user.expires_at)
    )
}

fn fmt_time(t: DateTime<Utc>) -> String {
    t.format("%Y-%m-%d %H:%M UTC").to_string()
}

fn print_password(password: &str) {
    println!("password: {password}");
    println!("(shown once, only its hash is stored; hand it over privately)");
}
