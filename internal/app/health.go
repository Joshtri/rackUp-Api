package app

import (
	"log/slog"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Joshtri/rackup-api/internal/httpx"
)

// health: the process is up. Doesn't touch the database.
func health(w http.ResponseWriter, _ *http.Request) error {
	httpx.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	return nil
}

// ready: the database is reachable.
func ready(pool *pgxpool.Pool) httpx.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		if _, err := pool.Exec(r.Context(), "SELECT 1"); err != nil {
			slog.Warn("database not reachable", "error", err)
			httpx.WriteJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "unavailable"})
			return nil
		}
		httpx.WriteJSON(w, http.StatusOK, map[string]string{"status": "ready"})
		return nil
	}
}
