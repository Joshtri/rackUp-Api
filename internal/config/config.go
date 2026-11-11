// Package config reads the app settings from environment variables.
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Settings holds the app settings. It contains secrets, so never log it.
type Settings struct {
	// Neon pooled connection string (host contains "-pooler").
	DatabaseURL      string
	DBMaxConnections int32
	JWTSecret        string
	JWTTTLDays       int64
	// Allowed browser origins. "*" allows any.
	CORSOrigins []string
	// Vercel sets PORT itself.
	Port int
}

// FromEnv reads and checks the settings.
func FromEnv() (Settings, error) {
	var s Settings
	var err error

	if s.JWTSecret, err = required("JWT_SECRET"); err != nil {
		return s, err
	}
	if len(s.JWTSecret) < 32 {
		return s, invalid("JWT_SECRET", "must be at least 32 characters")
	}
	if s.JWTTTLDays, err = parseInt("JWT_TTL_DAYS", 30, 64); err != nil {
		return s, err
	}
	if s.JWTTTLDays < 1 {
		return s, invalid("JWT_TTL_DAYS", "must be 1 or more")
	}
	if s.DatabaseURL, err = required("DATABASE_URL"); err != nil {
		return s, err
	}
	maxConns, err := parseInt("DB_MAX_CONNECTIONS", 5, 32)
	if err != nil {
		return s, err
	}
	if maxConns < 1 {
		return s, invalid("DB_MAX_CONNECTIONS", "must be 1 or more")
	}
	s.DBMaxConnections = int32(maxConns)

	origins := os.Getenv("CORS_ORIGINS")
	if origins == "" {
		origins = "http://localhost:5173"
	}
	s.CORSOrigins = ParseOrigins(origins)

	port, err := parseInt("PORT", 8080, 64)
	if err != nil {
		return s, err
	}
	if port < 0 || port > 65535 {
		return s, invalid("PORT", fmt.Sprintf("could not parse %q", os.Getenv("PORT")))
	}
	s.Port = int(port)
	return s, nil
}

// ParseOrigins splits a comma-separated list and drops trailing slashes.
func ParseOrigins(raw string) []string {
	var out []string
	for _, o := range strings.Split(raw, ",") {
		o = strings.TrimRight(strings.TrimSpace(o), "/")
		if o != "" {
			out = append(out, o)
		}
	}
	return out
}

func required(key string) (string, error) {
	v := os.Getenv(key)
	if strings.TrimSpace(v) == "" {
		return "", fmt.Errorf("missing environment variable %s", key)
	}
	return v, nil
}

func parseInt(key string, def int64, bits int) (int64, error) {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return def, nil
	}
	n, err := strconv.ParseInt(v, 10, bits)
	if err != nil {
		return 0, invalid(key, fmt.Sprintf("could not parse %q", v))
	}
	return n, nil
}

func invalid(key, msg string) error {
	return fmt.Errorf("invalid environment variable %s: %s", key, msg)
}
