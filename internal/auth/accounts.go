package auth

import (
	"context"
	"crypto/rand"
	"fmt"
	"strings"
	"time"

	"github.com/Joshtri/rackup-api/internal/db"
	"github.com/Joshtri/rackup-api/internal/httpx"
	"github.com/Joshtri/rackup-api/internal/validation"
)

// Account management for the admin CLI (`go run ./cmd/admin user ...`).
// There is no sign-up endpoint: every account is created here.

// DefaultAccountDays is how long a new or renewed account works, unless told otherwise.
const DefaultAccountDays = 30

const (
	// No look-alike characters (0/O/o, 1/l/I), so it can be read out or typed on a phone.
	passwordAlphabet = "ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnpqrstuvwxyz23456789"
	// 4 groups of 5 → 20 random characters ≈ 116 bits of entropy.
	passwordGroups   = 4
	passwordGroupLen = 5
)

// CreateAccount creates an account with a generated password.
// Returns the user and the plain password, which is shown once and never stored.
func CreateAccount(ctx context.Context, q db.DBTX, username, displayName string, days int64) (*User, string, error) {
	username, err := normalizeUsername(username)
	if err != nil {
		return nil, "", err
	}
	displayName = validation.CleanName(displayName)
	if err := validation.RequireText("display name", displayName, 50); err != nil {
		return nil, "", err
	}
	if err := validDays(days); err != nil {
		return nil, "", err
	}

	password, err := GeneratePassword()
	if err != nil {
		return nil, "", err
	}
	hash, err := HashPassword(password)
	if err != nil {
		return nil, "", err
	}
	expiresAt := time.Now().Add(time.Duration(days) * 24 * time.Hour)
	user, err := insertUser(ctx, q, username, hash, displayName, expiresAt)
	if db.IsUniqueViolation(err) {
		return nil, "", httpx.Conflict(fmt.Sprintf("username %q is already taken", username))
	}
	if err != nil {
		return nil, "", err
	}
	return user, password, nil
}

// RenewAccount sets the account to expire `days` from now (also revives an expired account).
func RenewAccount(ctx context.Context, q db.DBTX, username string, days int64) (*User, error) {
	if err := validDays(days); err != nil {
		return nil, err
	}
	expiresAt := time.Now().Add(time.Duration(days) * 24 * time.Hour)
	return orNotFound(setExpiry(ctx, q, lower(username), expiresAt))
}

// ExpireAccount ends the account now. Open sessions stop working on their next request.
func ExpireAccount(ctx context.Context, q db.DBTX, username string) (*User, error) {
	return orNotFound(setExpiry(ctx, q, lower(username), time.Now()))
}

// ResetPassword replaces the password with a new generated one, returned in plain text once.
func ResetPassword(ctx context.Context, q db.DBTX, username string) (*User, string, error) {
	password, err := GeneratePassword()
	if err != nil {
		return nil, "", err
	}
	hash, err := HashPassword(password)
	if err != nil {
		return nil, "", err
	}
	user, err := orNotFound(setPassword(ctx, q, lower(username), hash))
	if err != nil {
		return nil, "", err
	}
	return user, password, nil
}

func ListAccounts(ctx context.Context, q db.DBTX) ([]User, error) {
	return listUsers(ctx, q)
}

func orNotFound(u *User, err error) (*User, error) {
	if err != nil {
		return nil, err
	}
	if u == nil {
		return nil, httpx.ErrNotFound
	}
	return u, nil
}

func lower(s string) string { return strings.ToLower(strings.TrimSpace(s)) }

// normalizeUsername: lowercase, 3–32 characters: letters, digits, '.', '_', '-';
// starts with a letter or digit.
func normalizeUsername(raw string) (string, error) {
	name := lower(raw)
	ok := len(name) >= 3 && len(name) <= 32
	for i, c := range name {
		alnum := (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9')
		if !alnum && (i == 0 || (c != '.' && c != '_' && c != '-')) {
			ok = false
		}
	}
	if !ok {
		return "", httpx.Validation("username must be 3-32 characters: a-z, 0-9, '.', '_' or '-', starting with a letter or digit")
	}
	return name, nil
}

func validDays(days int64) error {
	if days < 1 || days > 3650 {
		return httpx.Validation("days must be between 1 and 3650")
	}
	return nil
}

// GeneratePassword returns a random password like `Xk7mP-q2Wd9-...` from the OS random source.
func GeneratePassword() (string, error) {
	// Largest multiple of the alphabet size that fits in a byte; bytes at or
	// above it are skipped so every character is equally likely.
	n := len(passwordAlphabet)
	limit := byte(256 / n * n)
	wanted := passwordGroups * passwordGroupLen
	chars := make([]byte, 0, wanted)
	buf := make([]byte, 64)
	for len(chars) < wanted {
		if _, err := rand.Read(buf); err != nil {
			return "", httpx.Internal(fmt.Errorf("random source failed: %w", err))
		}
		for _, b := range buf {
			if b < limit && len(chars) < wanted {
				chars = append(chars, passwordAlphabet[int(b)%n])
			}
		}
	}
	groups := make([]string, 0, passwordGroups)
	for i := 0; i < wanted; i += passwordGroupLen {
		groups = append(groups, string(chars[i:i+passwordGroupLen]))
	}
	return strings.Join(groups, "-"), nil
}
