// Package players lists the player names in the matches an account can see
// (for name suggestions).
//
// Both queries walk the visible matches newest first (date, then entry
// time), player A before player B, like `players` in the frontend's useMatches.
package players

import (
	"context"
	"net/http"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Joshtri/rackup-api/internal/auth"
	"github.com/Joshtri/rackup-api/internal/circle"
	"github.com/Joshtri/rackup-api/internal/db"
	"github.com/Joshtri/rackup-api/internal/httpx"
)

// names lists both players of every match the viewer ($1) can see.
var names = `
	SELECT player_a AS player, played_on, created_at, 0 AS side FROM matches m WHERE ` + circle.MatchVisible("m", "$1") + `
	UNION ALL
	SELECT player_b, played_on, created_at, 1 FROM matches m WHERE ` + circle.MatchVisible("m", "$1")

// List returns distinct names, most recently played first.
func List(ctx context.Context, q db.DBTX, viewer uuid.UUID) ([]string, error) {
	rows, err := q.Query(ctx,
		`SELECT player FROM (
		   SELECT player,
		          row_number() OVER (ORDER BY played_on DESC, created_at DESC, side) AS rn
		   FROM (`+names+`) names
		 ) ranked
		 GROUP BY player
		 ORDER BY min(rn)`, viewer)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []string{}
	for rows.Next() {
		var n string
		if err := rows.Scan(&n); err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

// FindSpelling returns the spelling the viewer already uses for name,
// ignoring case. ok is false if the name is new to them.
func FindSpelling(ctx context.Context, q db.DBTX, viewer uuid.UUID, name string) (spelling string, ok bool, err error) {
	err = q.QueryRow(ctx,
		`SELECT player FROM (`+names+`) names
		 WHERE lower(player) = lower($2)
		 ORDER BY played_on DESC, created_at DESC, side
		 LIMIT 1`, viewer, name).Scan(&spelling)
	if db.NoRows(err) {
		return "", false, nil
	}
	return spelling, err == nil, err
}

// Handler serves GET /players: names, most recently played first.
func Handler(pool *pgxpool.Pool) httpx.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		names, err := List(r.Context(), pool, auth.UserID(r.Context()))
		if err != nil {
			return err
		}
		httpx.WriteJSON(w, http.StatusOK, names)
		return nil
	}
}
