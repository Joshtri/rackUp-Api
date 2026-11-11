// Package auth handles log in, the current user, the auth middleware,
// and admin account management (there is no public sign-up).
package auth

import (
	"time"

	"github.com/google/uuid"
)

// User is a row of the `users` table.
type User struct {
	ID uuid.UUID
	// Lowercase, used to log in.
	Username string
	// Argon2id PHC string. Never the plain password.
	PasswordHash string
	DisplayName  string
	CreatedAt    time.Time
	// The account stops working at this moment.
	ExpiresAt time.Time
}

func (u *User) IsExpired() bool { return isExpired(u.ExpiresAt) }

func isExpired(expiresAt time.Time) bool { return !expiresAt.After(time.Now()) }

// UserResponse is the user as sent to the client. Times are epoch ms, like Date.now().
type UserResponse struct {
	ID          uuid.UUID `json:"id"`
	Username    string    `json:"username"`
	DisplayName string    `json:"displayName"`
	CreatedAt   int64     `json:"createdAt"`
	// When the account stops working.
	ExpiresAt int64 `json:"expiresAt"`
}

func toUserResponse(u *User) UserResponse {
	return UserResponse{
		ID:          u.ID,
		Username:    u.Username,
		DisplayName: u.DisplayName,
		CreatedAt:   u.CreatedAt.UnixMilli(),
		ExpiresAt:   u.ExpiresAt.UnixMilli(),
	}
}

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type authResponse struct {
	Token string `json:"token"`
	// Epoch ms when the token stops working. Never later than the account's own expiresAt.
	ExpiresAt int64        `json:"expiresAt"`
	User      UserResponse `json:"user"`
}

// accountResponse is another account as seen by a logged-in user. Item of GET /users.
type accountResponse struct {
	ID          uuid.UUID `json:"id"`
	Username    string    `json:"username"`
	DisplayName string    `json:"displayName"`
}
