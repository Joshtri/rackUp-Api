package auth

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"github.com/Joshtri/rackup-api/internal/httpx"
)

// Tokens are JWTs signed with HS256: {"sub": user id, "iat", "exp"}.
// Same format as the Rust version, so tokens it issued stay valid.

// IssueToken signs a token for userID that expires at exp (epoch seconds).
func IssueToken(userID uuid.UUID, secret string, exp int64) (string, error) {
	claims := jwt.RegisteredClaims{
		Subject:   userID.String(),
		IssuedAt:  jwt.NewNumericDate(time.Now()),
		ExpiresAt: jwt.NewNumericDate(time.Unix(exp, 0)),
	}
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(secret))
	if err != nil {
		return "", fmt.Errorf("token encode failed: %w", err)
	}
	return token, nil
}

// VerifyToken returns the user id of a valid, unexpired token.
func VerifyToken(token, secret string) (uuid.UUID, error) {
	var claims jwt.RegisteredClaims
	_, err := jwt.ParseWithClaims(token, &claims,
		func(*jwt.Token) (any, error) { return []byte(secret), nil },
		jwt.WithValidMethods([]string{"HS256"}),
		jwt.WithExpirationRequired(),
		jwt.WithLeeway(60*time.Second),
	)
	if err != nil {
		return uuid.Nil, httpx.ErrUnauthorized
	}
	id, err := uuid.Parse(claims.Subject)
	if err != nil {
		return uuid.Nil, httpx.ErrUnauthorized
	}
	return id, nil
}

// tokenExpiry is the token lifetime in epoch seconds: ttlDays from now,
// but never past the account's own expiry.
func tokenExpiry(now time.Time, ttlDays int64, accountExpiresAt time.Time) int64 {
	return min(now.Unix()+ttlDays*24*60*60, accountExpiresAt.Unix())
}
