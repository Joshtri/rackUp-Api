// Package app wires every route and middleware into one http.Handler.
package app

import (
	"log/slog"
	"net/http"
	"slices"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Joshtri/rackup-api/internal/auth"
	"github.com/Joshtri/rackup-api/internal/config"
	"github.com/Joshtri/rackup-api/internal/httpx"
	"github.com/Joshtri/rackup-api/internal/livesessions"
	"github.com/Joshtri/rackup-api/internal/matches"
	"github.com/Joshtri/rackup-api/internal/players"
)

// Vercel caps request bodies at 4.5 MB, so a larger limit here would never be reached.
const bodyLimitBytes = 4_500_000

// New builds the whole API.
func New(s config.Settings, pool *pgxpool.Pool) http.Handler {
	a := &auth.Handlers{DB: pool, Settings: s}
	m := &matches.Handlers{DB: pool}
	ls := &livesessions.Handlers{DB: pool}
	private := a.Require

	mux := http.NewServeMux()
	handle := func(pattern string, h httpx.HandlerFunc) { mux.Handle(pattern, h) }

	handle("GET /health", health)
	handle("GET /ready", ready(pool))

	// No sign-up route: accounts are created with `go run ./cmd/admin user create`.
	handle("POST /auth/login", a.Login)
	handle("GET /auth/me", private(a.Me))
	handle("GET /users", private(a.Users))

	handle("GET /live-sessions", private(ls.List))
	handle("POST /live-sessions", private(ls.Create))
	handle("GET /live-sessions/{id}", private(ls.Get))
	handle("PUT /live-sessions/{id}", private(ls.Update))
	handle("POST /live-sessions/{id}/end", private(ls.End))

	handle("GET /matches", private(m.List))
	handle("POST /matches", private(m.Create))
	handle("POST /matches/import", private(m.Import))
	handle("GET /matches/{id}", private(m.Get))
	handle("PUT /matches/{id}", private(m.Update))
	handle("DELETE /matches/{id}", private(m.Delete))

	handle("GET /players", private(players.Handler(pool)))

	handle("/", func(http.ResponseWriter, *http.Request) error { return httpx.ErrNotFound })

	return logRequests(cors(s.CORSOrigins, limitBody(mux)))
}

func limitBody(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, bodyLimitBytes)
		next.ServeHTTP(w, r)
	})
}

// cors lets the frontend (another domain) call the API.
// Auth uses a bearer token, not cookies, so no credentials mode is needed.
func cors(origins []string, next http.Handler) http.Handler {
	anyOrigin := slices.Contains(origins, "*")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		origin := r.Header.Get("Origin")
		switch {
		case anyOrigin:
			h.Set("Access-Control-Allow-Origin", "*")
		case origin != "" && slices.Contains(origins, origin):
			h.Set("Access-Control-Allow-Origin", origin)
		}
		if !anyOrigin {
			h.Add("Vary", "Origin")
		}
		if r.Method == http.MethodOptions && r.Header.Get("Access-Control-Request-Method") != "" {
			h.Add("Vary", "Access-Control-Request-Method, Access-Control-Request-Headers")
			h.Set("Access-Control-Allow-Methods", "GET,POST,PUT,DELETE")
			h.Set("Access-Control-Allow-Headers", "authorization,content-type")
			h.Set("Access-Control-Max-Age", "3600")
			w.WriteHeader(http.StatusOK)
			return
		}
		next.ServeHTTP(w, r)
	})
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (s *statusRecorder) WriteHeader(code int) {
	s.status = code
	s.ResponseWriter.WriteHeader(code)
}

// logRequests logs each request at debug level (LOG_LEVEL=debug).
func logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		slog.Debug("request", "method", r.Method, "path", r.URL.Path,
			"status", rec.status, "ms", time.Since(start).Milliseconds())
	})
}
