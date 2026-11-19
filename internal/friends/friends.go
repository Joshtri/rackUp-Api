// Package friends handles invitations between accounts.
//
// An account invites another by username. Once the invitation is accepted
// the two are friends and see each other's matches and live sessions (see
// package circle). Every endpoint answers with the caller's full overview,
// so the app can redraw its friends screen from any response.
package friends

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Joshtri/rackup-api/internal/auth"
	"github.com/Joshtri/rackup-api/internal/circle"
	"github.com/Joshtri/rackup-api/internal/db"
	"github.com/Joshtri/rackup-api/internal/httpx"
)

var (
	errUserNotFound   = &httpx.Error{Status: http.StatusNotFound, Code: "user_not_found", Message: "no active account has this username"}
	errAlreadyFriends = &httpx.Error{Status: http.StatusConflict, Code: "already_friends", Message: "you are already friends"}
	errSelf           = &httpx.Error{Status: http.StatusUnprocessableEntity, Code: "validation_error", Message: "you can't invite yourself"}
)

// Account is another account as the app shows it.
type Account struct {
	ID          uuid.UUID `json:"id"`
	Username    string    `json:"username"`
	DisplayName string    `json:"displayName"`
}

type Friend struct {
	Account
	// Epoch ms when the friendship started.
	Since int64 `json:"since"`
}

type Request struct {
	ID uuid.UUID `json:"id"`
	// The other side: who sent it (incoming) or who it was sent to (outgoing).
	User      Account `json:"user"`
	CreatedAt int64   `json:"createdAt"`
}

// Overview is the body of every /friends response.
type Overview struct {
	Friends  []Friend  `json:"friends"`
	Incoming []Request `json:"incoming"`
	Outgoing []Request `json:"outgoing"`
}

type inviteRequest struct {
	Username string `json:"username"`
}

func (r *inviteRequest) UnmarshalJSON(b []byte) error {
	if err := httpx.RequireFields(b, "username"); err != nil {
		return err
	}
	type plain inviteRequest
	return json.Unmarshal(b, (*plain)(r))
}

// Handlers serves /friends. Every route needs a logged-in user (see auth.Require).
type Handlers struct {
	DB *pgxpool.Pool
}

func (h *Handlers) reply(w http.ResponseWriter, r *http.Request) error {
	o, err := overview(r.Context(), h.DB, auth.UserID(r.Context()))
	if err != nil {
		return err
	}
	httpx.WriteJSON(w, http.StatusOK, o)
	return nil
}

// List serves GET /friends.
func (h *Handlers) List(w http.ResponseWriter, r *http.Request) error {
	return h.reply(w, r)
}

// Invite serves POST /friends/requests {username}. If that account already
// invited the caller, they become friends right away. Repeating it is harmless.
func (h *Handlers) Invite(w http.ResponseWriter, r *http.Request) error {
	var req inviteRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		return err
	}
	username := strings.ToLower(strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(req.Username), "@")))
	if username == "" {
		return httpx.Validation("username is required")
	}
	if err := invite(r.Context(), h.DB, auth.UserID(r.Context()), username); err != nil {
		return err
	}
	return h.reply(w, r)
}

// Accept serves POST /friends/requests/{id}/accept (only for the invited account).
func (h *Handlers) Accept(w http.ResponseWriter, r *http.Request) error {
	id, err := httpx.PathUUID(r, "id")
	if err != nil {
		return err
	}
	if err := accept(r.Context(), h.DB, auth.UserID(r.Context()), id); err != nil {
		return err
	}
	return h.reply(w, r)
}

// DeleteRequest serves DELETE /friends/requests/{id}: decline an incoming
// invitation or cancel an outgoing one.
func (h *Handlers) DeleteRequest(w http.ResponseWriter, r *http.Request) error {
	id, err := httpx.PathUUID(r, "id")
	if err != nil {
		return err
	}
	tag, err := h.DB.Exec(r.Context(),
		"DELETE FROM friend_requests WHERE id = $1 AND (from_user = $2 OR to_user = $2)",
		id, auth.UserID(r.Context()))
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return httpx.ErrNotFound
	}
	return h.reply(w, r)
}

// Remove serves DELETE /friends/{userId}: end a friendship, for both sides.
// Matches both played in stay visible to both (they're linked as players).
func (h *Handlers) Remove(w http.ResponseWriter, r *http.Request) error {
	other, err := httpx.PathUUID(r, "userId")
	if err != nil {
		return err
	}
	me := auth.UserID(r.Context())
	tag, err := h.DB.Exec(r.Context(),
		`DELETE FROM friendships
		 WHERE (user_id = $1 AND friend_id = $2) OR (user_id = $2 AND friend_id = $1)`,
		me, other)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return httpx.ErrNotFound
	}
	return h.reply(w, r)
}

func invite(ctx context.Context, pool *pgxpool.Pool, me uuid.UUID, username string) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	var target uuid.UUID
	err = tx.QueryRow(ctx,
		"SELECT id FROM users WHERE username = $1 AND expires_at > now()", username).Scan(&target)
	if db.NoRows(err) {
		return errUserNotFound
	}
	if err != nil {
		return err
	}
	if target == me {
		return errSelf
	}

	var friends bool
	if err := tx.QueryRow(ctx,
		"SELECT EXISTS (SELECT 1 FROM friendships WHERE user_id = $1 AND friend_id = $2)",
		me, target).Scan(&friends); err != nil {
		return err
	}
	if friends {
		return errAlreadyFriends
	}

	// They invited me first: inviting back means yes.
	var theirs bool
	if err := tx.QueryRow(ctx,
		"SELECT EXISTS (SELECT 1 FROM friend_requests WHERE from_user = $1 AND to_user = $2)",
		target, me).Scan(&theirs); err != nil {
		return err
	}
	if theirs {
		if err := befriend(ctx, tx, me, target); err != nil {
			return err
		}
	} else if _, err := tx.Exec(ctx,
		`INSERT INTO friend_requests (id, from_user, to_user) VALUES ($1, $2, $3)
		 ON CONFLICT (from_user, to_user) DO NOTHING`,
		uuid.New(), me, target); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func accept(ctx context.Context, pool *pgxpool.Pool, me, requestID uuid.UUID) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	var from uuid.UUID
	err = tx.QueryRow(ctx,
		"SELECT from_user FROM friend_requests WHERE id = $1 AND to_user = $2 FOR UPDATE",
		requestID, me).Scan(&from)
	if db.NoRows(err) {
		return httpx.ErrNotFound
	}
	if err != nil {
		return err
	}
	if err := befriend(ctx, tx, me, from); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// befriend makes a and b friends, drops the invitations between them, and
// links the names in their matches to each other's accounts.
func befriend(ctx context.Context, tx pgx.Tx, a, b uuid.UUID) error {
	if _, err := tx.Exec(ctx,
		`INSERT INTO friendships (user_id, friend_id) VALUES ($1, $2), ($2, $1)
		 ON CONFLICT DO NOTHING`, a, b); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx,
		`DELETE FROM friend_requests
		 WHERE (from_user = $1 AND to_user = $2) OR (from_user = $2 AND to_user = $1)`,
		a, b); err != nil {
		return err
	}
	return circle.LinkRecordedBy(ctx, tx, a, b)
}

func overview(ctx context.Context, q db.DBTX, me uuid.UUID) (Overview, error) {
	o := Overview{Friends: []Friend{}, Incoming: []Request{}, Outgoing: []Request{}}

	rows, err := q.Query(ctx,
		`SELECT u.id, u.username, u.display_name, f.created_at
		 FROM friendships f JOIN users u ON u.id = f.friend_id
		 WHERE f.user_id = $1
		 ORDER BY lower(u.display_name)`, me)
	if err != nil {
		return o, err
	}
	for rows.Next() {
		var f Friend
		var since time.Time
		if err := rows.Scan(&f.ID, &f.Username, &f.DisplayName, &since); err != nil {
			rows.Close()
			return o, err
		}
		f.Since = since.UnixMilli()
		o.Friends = append(o.Friends, f)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return o, err
	}

	rows, err = q.Query(ctx,
		`SELECT r.id, r.to_user = $1, u.id, u.username, u.display_name, r.created_at
		 FROM friend_requests r
		 JOIN users u ON u.id = CASE WHEN r.to_user = $1 THEN r.from_user ELSE r.to_user END
		 WHERE r.from_user = $1 OR r.to_user = $1
		 ORDER BY r.created_at DESC`, me)
	if err != nil {
		return o, err
	}
	defer rows.Close()
	for rows.Next() {
		var req Request
		var incoming bool
		var at time.Time
		if err := rows.Scan(&req.ID, &incoming, &req.User.ID, &req.User.Username, &req.User.DisplayName, &at); err != nil {
			return o, err
		}
		req.CreatedAt = at.UnixMilli()
		if incoming {
			o.Incoming = append(o.Incoming, req)
		} else {
			o.Outgoing = append(o.Outgoing, req)
		}
	}
	return o, rows.Err()
}
