// Package players lists the player names used in the shared history (for name suggestions).
//
// History is shared, so both queries walk all matches newest first (date,
// then entry time), player A before player B, like `players` in the
// frontend's useMatches.
package players

import (
	"context"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Joshtri/rackup-api/internal/db"
	"github.com/Joshtri/rackup-api/internal/httpx"
)

// List returns distinct names, most recently played first.
func List(ctx context.Context, q db.DBTX) ([]string, error) {
	rows, err := q.Query(ctx,
		`SELECT player FROM (
		   SELECT player,
		          row_number() OVER (ORDER BY played_on DESC, created_at DESC, side) AS rn
		   FROM (
		     SELECT player_a AS player, played_on, created_at, 0 AS side FROM matches
		     UNION ALL
		     SELECT player_b, played_on, created_at, 1 FROM matches
		   ) names
		 ) ranked
		 GROUP BY player
		 ORDER BY min(rn)`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	names := []string{}
	for rows.Next() {
		var n string
		if err := rows.Scan(&n); err != nil {
			return nil, err
		}
		names = append(names, n)
	}
	return names, rows.Err()
}

// FindSpelling returns the spelling already in use for name, ignoring case.
// ok is false if the name is new.
func FindSpelling(ctx context.Context, q db.DBTX, name string) (spelling string, ok bool, err error) {
	err = q.QueryRow(ctx,
		`SELECT player FROM (
		   SELECT player_a AS player, played_on, created_at, 0 AS side FROM matches
		   UNION ALL
		   SELECT player_b, played_on, created_at, 1 FROM matches
		 ) names
		 WHERE lower(player) = lower($1)
		 ORDER BY played_on DESC, created_at DESC, side
		 LIMIT 1`, name).Scan(&spelling)
	if db.NoRows(err) {
		return "", false, nil
	}
	return spelling, err == nil, err
}

// Handler serves GET /players: names, most recently played first.
func Handler(pool *pgxpool.Pool) httpx.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		names, err := List(r.Context(), pool)
		if err != nil {
			return err
		}
		httpx.WriteJSON(w, http.StatusOK, names)
		return nil
	}
}
