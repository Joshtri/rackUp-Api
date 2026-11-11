package auth

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/Joshtri/rackup-api/internal/db"
)

const userColumns = "id, username, password_hash, display_name, created_at, expires_at"

func scanUser(row pgx.Row) (*User, error) {
	var u User
	err := row.Scan(&u.ID, &u.Username, &u.PasswordHash, &u.DisplayName, &u.CreatedAt, &u.ExpiresAt)
	if db.NoRows(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func insertUser(ctx context.Context, q db.DBTX, username, passwordHash, displayName string, expiresAt time.Time) (*User, error) {
	return scanUser(q.QueryRow(ctx,
		`INSERT INTO users (id, username, password_hash, display_name, expires_at)
		 VALUES ($1, $2, $3, $4, $5)
		 RETURNING `+userColumns,
		uuid.New(), username, passwordHash, displayName, expiresAt))
}

func findByUsername(ctx context.Context, q db.DBTX, username string) (*User, error) {
	return scanUser(q.QueryRow(ctx, "SELECT "+userColumns+" FROM users WHERE username = $1", username))
}

func findByID(ctx context.Context, q db.DBTX, id uuid.UUID) (*User, error) {
	return scanUser(q.QueryRow(ctx, "SELECT "+userColumns+" FROM users WHERE id = $1", id))
}

// findExpiry reads only the expiry, for the per-request auth check.
// ok is false when the user doesn't exist.
func findExpiry(ctx context.Context, q db.DBTX, id uuid.UUID) (expiresAt time.Time, ok bool, err error) {
	err = q.QueryRow(ctx, "SELECT expires_at FROM users WHERE id = $1", id).Scan(&expiresAt)
	if db.NoRows(err) {
		return time.Time{}, false, nil
	}
	return expiresAt, err == nil, err
}

func listUsers(ctx context.Context, q db.DBTX) ([]User, error) {
	rows, err := q.Query(ctx, "SELECT "+userColumns+" FROM users ORDER BY username")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	users := []User{}
	for rows.Next() {
		u, err := scanUser(rows)
		if err != nil {
			return nil, err
		}
		users = append(users, *u)
	}
	return users, rows.Err()
}

func setExpiry(ctx context.Context, q db.DBTX, username string, expiresAt time.Time) (*User, error) {
	return scanUser(q.QueryRow(ctx,
		"UPDATE users SET expires_at = $2 WHERE username = $1 RETURNING "+userColumns,
		username, expiresAt))
}

func setPassword(ctx context.Context, q db.DBTX, username, passwordHash string) (*User, error) {
	return scanUser(q.QueryRow(ctx,
		"UPDATE users SET password_hash = $2 WHERE username = $1 RETURNING "+userColumns,
		username, passwordHash))
}
