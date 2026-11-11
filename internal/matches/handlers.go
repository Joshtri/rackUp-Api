package matches

import (
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Joshtri/rackup-api/internal/auth"
	"github.com/Joshtri/rackup-api/internal/httpx"
)

// Handlers serves /matches. Every route needs a logged-in user (see auth.Require);
// any account can read and change every match.
type Handlers struct {
	DB *pgxpool.Pool
}

func (h *Handlers) List(w http.ResponseWriter, r *http.Request) error {
	out, err := listAll(r.Context(), h.DB)
	if err != nil {
		return err
	}
	httpx.WriteJSON(w, http.StatusOK, out)
	return nil
}

func (h *Handlers) Get(w http.ResponseWriter, r *http.Request) error {
	id, err := httpx.PathUUID(r, "id")
	if err != nil {
		return err
	}
	m, err := get(r.Context(), h.DB, id)
	if err != nil {
		return err
	}
	httpx.WriteJSON(w, http.StatusOK, m)
	return nil
}

// Create answers 201 when created, 200 when the id already existed (restore after undo).
func (h *Handlers) Create(w http.ResponseWriter, r *http.Request) error {
	var req CreateMatchRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		return err
	}
	m, err := ToNewMatch(req)
	if err != nil {
		return err
	}
	resp, created, err := CreateIn(r.Context(), h.DB, auth.UserID(r.Context()), m)
	if err != nil {
		return err
	}
	status := http.StatusOK
	if created {
		status = http.StatusCreated
	}
	httpx.WriteJSON(w, status, resp)
	return nil
}

func (h *Handlers) Update(w http.ResponseWriter, r *http.Request) error {
	id, err := httpx.PathUUID(r, "id")
	if err != nil {
		return err
	}
	var fields MatchFields
	if err := httpx.DecodeJSON(r, &fields); err != nil {
		return err
	}
	m, err := updateMatch(r.Context(), h.DB, id, fields)
	if err != nil {
		return err
	}
	httpx.WriteJSON(w, http.StatusOK, m)
	return nil
}

func (h *Handlers) Delete(w http.ResponseWriter, r *http.Request) error {
	id, err := httpx.PathUUID(r, "id")
	if err != nil {
		return err
	}
	if err := remove(r.Context(), h.DB, id); err != nil {
		return err
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}

func (h *Handlers) Import(w http.ResponseWriter, r *http.Request) error {
	var req importRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		return err
	}
	resp, err := importMatches(r.Context(), h.DB, auth.UserID(r.Context()), req)
	if err != nil {
		return err
	}
	httpx.WriteJSON(w, http.StatusOK, resp)
	return nil
}
