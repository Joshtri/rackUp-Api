use sqlx::PgExecutor;

// History is shared, so both queries walk all matches newest first (date,
// then entry time), player A before player B, like `players` in the
// frontend's useMatches.

/// Distinct names, most recently played first.
pub async fn list<'e>(db: impl PgExecutor<'e>) -> Result<Vec<String>, sqlx::Error> {
    sqlx::query_scalar::<_, String>(
        "SELECT player FROM (
           SELECT player,
                  row_number() OVER (ORDER BY played_on DESC, created_at DESC, side) AS rn
           FROM (
             SELECT player_a AS player, played_on, created_at, 0 AS side FROM matches
             UNION ALL
             SELECT player_b, played_on, created_at, 1 FROM matches
           ) names
         ) ranked
         GROUP BY player
         ORDER BY min(rn)",
    )
    .fetch_all(db)
    .await
}

/// The spelling already in use for `name`, ignoring case. `None` if it's new.
pub async fn find_spelling<'e>(
    db: impl PgExecutor<'e>,
    name: &str,
) -> Result<Option<String>, sqlx::Error> {
    sqlx::query_scalar::<_, String>(
        "SELECT player FROM (
           SELECT player_a AS player, played_on, created_at, 0 AS side FROM matches
           UNION ALL
           SELECT player_b, played_on, created_at, 1 FROM matches
         ) names
         WHERE lower(player) = lower($1)
         ORDER BY played_on DESC, created_at DESC, side
         LIMIT 1",
    )
    .bind(name)
    .fetch_optional(db)
    .await
}
