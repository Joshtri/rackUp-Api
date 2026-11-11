package livesessions

import (
	"net/http"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Joshtri/rackup-api/internal/auth"
	"github.com/Joshtri/rackup-api/internal/httpx"
)

// Handlers serves /live-sessions. Every route needs a logged-in user (see auth.Require).
type Handlers struct {
	DB *pgxpool.Pool
}

func now() int64 { return time.Now().UnixMilli() }

func (h *Handlers) List(w http.ResponseWriter, r *http.Request) error {
	sessions, err := listAll(r.Context(), h.DB)
	if err != nil {
		return err
	}
	httpx.WriteJSON(w, http.StatusOK, listResponse{ServerNow: now(), Sessions: sessions})
	return nil
}

// Get supports ?since=<version>: `session` is left out when nothing changed,
// which keeps the once-a-second poll tiny.
func (h *Handlers) Get(w http.ResponseWriter, r *http.Request) error {
	id, err := httpx.PathUUID(r, "id")
	if err != nil {
		return err
	}
	var since *int32
	if q := r.URL.Query(); q.Has("since") {
		v, err := strconv.ParseInt(q.Get("since"), 10, 32)
		if err != nil {
			return httpx.BadRequest(http.StatusBadRequest, "Failed to deserialize query string: since must be an integer")
		}
		n := int32(v)
		since = &n
	}
	s, err := get(r.Context(), h.DB, id, since)
	if err != nil {
		return err
	}
	httpx.WriteJSON(w, http.StatusOK, oneResponse{ServerNow: now(), Session: s})
	return nil
}

// Create answers 201 when created, 200 when the id already existed.
func (h *Handlers) Create(w http.ResponseWriter, r *http.Request) error {
	var req createRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		return err
	}
	s, created, err := create(r.Context(), h.DB, auth.UserID(r.Context()), req)
	if err != nil {
		return err
	}
	status := http.StatusOK
	if created {
		status = http.StatusCreated
	}
	httpx.WriteJSON(w, status, oneResponse{ServerNow: now(), Session: &s})
	return nil
}

func (h *Handlers) Update(w http.ResponseWriter, r *http.Request) error {
	id, err := httpx.PathUUID(r, "id")
	if err != nil {
		return err
	}
	var req updateRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		return err
	}
	s, err := update(r.Context(), h.DB, id, req)
	if err != nil {
		return err
	}
	httpx.WriteJSON(w, http.StatusOK, oneResponse{ServerNow: now(), Session: &s})
	return nil
}

func (h *Handlers) End(w http.ResponseWriter, r *http.Request) error {
	id, err := httpx.PathUUID(r, "id")
	if err != nil {
		return err
	}
	var req endRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		return err
	}
	s, saved, err := end(r.Context(), h.DB, auth.UserID(r.Context()), id, req)
	if err != nil {
		return err
	}
	httpx.WriteJSON(w, http.StatusOK, endResponse{ServerNow: now(), Session: s, Saved: saved})
	return nil
}
