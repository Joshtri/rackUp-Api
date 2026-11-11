package auth

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Joshtri/rackup-api/internal/config"
	"github.com/Joshtri/rackup-api/internal/httpx"
)

// Handlers serves /auth/login, /auth/me and /users, and guards the other routes.
// No sign-up route: accounts are created with `go run ./cmd/admin user create`.
type Handlers struct {
	DB       *pgxpool.Pool
	Settings config.Settings
}

func (h *Handlers) Login(w http.ResponseWriter, r *http.Request) error {
	var req loginRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		return err
	}
	username := strings.ToLower(strings.TrimSpace(req.Username))
	user, err := findByUsername(r.Context(), h.DB, username)
	if err != nil {
		return err
	}

	// Hash something even when the username is unknown, so response time
	// doesn't reveal which usernames exist.
	stored := dummyHash()
	if user != nil {
		stored = user.PasswordHash
	}
	valid, err := VerifyPassword(req.Password, stored)
	if err != nil {
		return httpx.Internal(err)
	}
	if user == nil || !valid {
		return httpx.ErrInvalidCredentials
	}
	// Only say "expired" after the password checked out, so the
	// message can't be used to probe for usernames.
	if user.IsExpired() {
		return httpx.ErrAccountExpired
	}

	exp := tokenExpiry(time.Now(), h.Settings.JWTTTLDays, user.ExpiresAt)
	token, err := IssueToken(user.ID, h.Settings.JWTSecret, exp)
	if err != nil {
		return httpx.Internal(err)
	}
	httpx.WriteJSON(w, http.StatusOK, authResponse{
		Token:     token,
		ExpiresAt: exp * 1000,
		User:      toUserResponse(user),
	})
	return nil
}

func (h *Handlers) Me(w http.ResponseWriter, r *http.Request) error {
	user, err := findByID(r.Context(), h.DB, UserID(r.Context()))
	if err != nil {
		return err
	}
	if user == nil {
		// Token is valid but the account is gone.
		return httpx.ErrUnauthorized
	}
	httpx.WriteJSON(w, http.StatusOK, toUserResponse(user))
	return nil
}

// Users lists active accounts, so a session's players can be linked to them.
func (h *Handlers) Users(w http.ResponseWriter, r *http.Request) error {
	users, err := listUsers(r.Context(), h.DB)
	if err != nil {
		return err
	}
	out := []accountResponse{}
	for _, u := range users {
		if !u.IsExpired() {
			out = append(out, accountResponse{ID: u.ID, Username: u.Username, DisplayName: u.DisplayName})
		}
	}
	httpx.WriteJSON(w, http.StatusOK, out)
	return nil
}

type userIDKey struct{}

// Require only lets requests with a valid `Authorization: Bearer <token>` through.
//
// Rejects with 401 `unauthorized` (missing/invalid token, deleted account)
// or 401 `account_expired`. The expiry is read from the database on every
// request, so renewing or ending an account takes effect immediately.
func (h *Handlers) Require(next httpx.HandlerFunc) httpx.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		token, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
		if !ok {
			return httpx.ErrUnauthorized
		}
		id, err := VerifyToken(strings.TrimSpace(token), h.Settings.JWTSecret)
		if err != nil {
			return err
		}
		expiresAt, found, err := findExpiry(r.Context(), h.DB, id)
		if err != nil {
			return err
		}
		if !found {
			return httpx.ErrUnauthorized
		}
		if isExpired(expiresAt) {
			return httpx.ErrAccountExpired
		}
		return next(w, r.WithContext(context.WithValue(r.Context(), userIDKey{}, id)))
	}
}

// UserID is the logged-in user's id. Only valid inside a handler wrapped by Require.
func UserID(ctx context.Context) uuid.UUID {
	id, _ := ctx.Value(userIDKey{}).(uuid.UUID)
	return id
}
