package db

import (
	"bytes"
	"context"
	"crypto/sha512"
	"fmt"
	"hash/crc32"
	"io/fs"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/Joshtri/rackup-api/migrations"
)

// Migrations are tracked in sqlx's `_sqlx_migrations` table, in the same
// format, so a database first migrated by the old Rust version (sqlx) is
// picked up as-is and nothing runs twice.

type migration struct {
	version     int64
	description string
	sql         []byte
	checksum    []byte
}

// Migrate applies pending migrations from the embedded `migrations/` folder.
// Use a direct (unpooled) connection: it takes a session-level advisory
// lock, which Neon's pooler (PgBouncer in transaction mode) doesn't support.
// Returns the versions it applied.
func Migrate(ctx context.Context, conn *pgx.Conn) ([]int64, error) {
	files, err := loadMigrations()
	if err != nil {
		return nil, err
	}

	if _, err := conn.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS _sqlx_migrations (
			version BIGINT PRIMARY KEY,
			description TEXT NOT NULL,
			installed_on TIMESTAMPTZ NOT NULL DEFAULT now(),
			success BOOLEAN NOT NULL,
			checksum BYTEA NOT NULL,
			execution_time BIGINT NOT NULL
		)`); err != nil {
		return nil, fmt.Errorf("create migrations table: %w", err)
	}

	lockID, err := lockID(ctx, conn)
	if err != nil {
		return nil, err
	}
	if _, err := conn.Exec(ctx, "SELECT pg_advisory_lock($1)", lockID); err != nil {
		return nil, fmt.Errorf("take migration lock: %w", err)
	}
	defer conn.Exec(context.WithoutCancel(ctx), "SELECT pg_advisory_unlock($1)", lockID) //nolint:errcheck

	applied, err := appliedMigrations(ctx, conn)
	if err != nil {
		return nil, err
	}

	known := map[int64]bool{}
	for _, m := range files {
		known[m.version] = true
	}
	for version, row := range applied {
		if !row.success {
			return nil, fmt.Errorf("migration %d is partially applied; fix it by hand and delete its row from _sqlx_migrations", version)
		}
		if !known[version] {
			return nil, fmt.Errorf("migration %d was applied but its file is missing from migrations/", version)
		}
	}

	var done []int64
	for _, m := range files {
		if row, ok := applied[m.version]; ok {
			if !bytes.Equal(row.checksum, m.checksum) {
				return done, fmt.Errorf("migration %d was changed after it was applied", m.version)
			}
			continue
		}
		if err := apply(ctx, conn, m); err != nil {
			return done, fmt.Errorf("migration %d (%s): %w", m.version, m.description, err)
		}
		done = append(done, m.version)
	}
	return done, nil
}

func apply(ctx context.Context, conn *pgx.Conn, m migration) error {
	start := time.Now()
	tx, err := conn.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	// No arguments, so pgx uses the simple protocol: a file may hold many statements.
	if _, err := tx.Exec(ctx, string(m.sql)); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx,
		`INSERT INTO _sqlx_migrations (version, description, success, checksum, execution_time)
		 VALUES ($1, $2, TRUE, $3, -1)`,
		m.version, m.description, m.checksum); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}
	_, err = conn.Exec(ctx, "UPDATE _sqlx_migrations SET execution_time = $1 WHERE version = $2",
		time.Since(start).Nanoseconds(), m.version)
	return err
}

type appliedRow struct {
	success  bool
	checksum []byte
}

func appliedMigrations(ctx context.Context, conn *pgx.Conn) (map[int64]appliedRow, error) {
	rows, err := conn.Query(ctx, "SELECT version, success, checksum FROM _sqlx_migrations ORDER BY version")
	if err != nil {
		return nil, fmt.Errorf("read applied migrations: %w", err)
	}
	defer rows.Close()
	out := map[int64]appliedRow{}
	for rows.Next() {
		var v int64
		var r appliedRow
		if err := rows.Scan(&v, &r.success, &r.checksum); err != nil {
			return nil, err
		}
		out[v] = r
	}
	return out, rows.Err()
}

// lockID is the same advisory lock id sqlx uses, so the two never migrate at once.
func lockID(ctx context.Context, conn *pgx.Conn) (int64, error) {
	var name string
	if err := conn.QueryRow(ctx, "SELECT current_database()").Scan(&name); err != nil {
		return 0, fmt.Errorf("read database name: %w", err)
	}
	return 0x3d32ad9e * int64(crc32.ChecksumIEEE([]byte(name))), nil
}

// loadMigrations reads files named `<version>_<description>.sql`, oldest first.
func loadMigrations() ([]migration, error) {
	entries, err := fs.ReadDir(migrations.FS, ".")
	if err != nil {
		return nil, err
	}
	var out []migration
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".sql") {
			continue
		}
		base := strings.TrimSuffix(name, ".sql")
		versionPart, desc, ok := strings.Cut(base, "_")
		if !ok {
			return nil, fmt.Errorf("migration file %q: expected <version>_<description>.sql", name)
		}
		version, err := strconv.ParseInt(versionPart, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("migration file %q: bad version", name)
		}
		sql, err := fs.ReadFile(migrations.FS, name)
		if err != nil {
			return nil, err
		}
		sum := sha512.Sum384(sql)
		out = append(out, migration{
			version:     version,
			description: strings.ReplaceAll(desc, "_", " "),
			sql:         sql,
			checksum:    sum[:],
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].version < out[j].version })
	return out, nil
}
