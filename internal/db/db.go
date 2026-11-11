// Package db opens the Postgres pool and runs migrations.
package db

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Joshtri/rackup-api/internal/config"
)

// DBTX is what queries run on: the pool, one connection, or a transaction.
type DBTX interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// NewPool creates the pool. No connection opens until the first query,
// so a cold start doesn't wait on the database.
func NewPool(ctx context.Context, s config.Settings) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(s.DatabaseURL)
	if err != nil {
		return nil, err
	}
	cfg.MaxConns = s.DBMaxConnections
	cfg.MinConns = 0
	cfg.MaxConnIdleTime = 60 * time.Second
	// Neon can take a few seconds to wake a suspended compute.
	cfg.ConnConfig.ConnectTimeout = 10 * time.Second
	return pgxpool.NewWithConfig(ctx, cfg)
}

// IsUniqueViolation reports whether err is a unique constraint violation.
func IsUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

// NoRows reports whether err means the query found nothing.
func NoRows(err error) bool {
	return errors.Is(err, pgx.ErrNoRows)
}
