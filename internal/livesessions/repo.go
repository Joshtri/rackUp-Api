package livesessions

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/Joshtri/rackup-api/internal/db"
)

// selectSessions selects every LiveSession column, with the name of whoever
// ended the session.
const selectSessions = `
	SELECT s.id, s.created_by, s.state, s.version, s.ended_at, s.ended_by,
	       u.display_name, s.saved_match_id, s.created_at, s.updated_at
	FROM live_sessions s LEFT JOIN users u ON u.id = s.ended_by `

func scanSession(row pgx.Row) (*LiveSession, error) {
	var s LiveSession
	var state []byte
	err := row.Scan(&s.ID, &s.CreatedBy, &state, &s.Version, &s.EndedAt, &s.EndedBy,
		&s.EndedByName, &s.SavedMatchID, &s.CreatedAt, &s.UpdatedAt)
	if db.NoRows(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	s.State = json.RawMessage(state)
	return &s, nil
}

// listActive returns running sessions touched in the last 12 hours, newest
// first. Older ones were most likely left open by accident.
func listActive(ctx context.Context, q db.DBTX) ([]LiveSession, error) {
	rows, err := q.Query(ctx, selectSessions+`
		WHERE s.ended_at IS NULL AND s.updated_at > now() - INTERVAL '12 hours'
		ORDER BY s.updated_at DESC LIMIT 20`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []LiveSession{}
	for rows.Next() {
		s, err := scanSession(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *s)
	}
	return out, rows.Err()
}

func find(ctx context.Context, q db.DBTX, id uuid.UUID) (*LiveSession, error) {
	return scanSession(q.QueryRow(ctx, selectSessions+"WHERE s.id = $1", id))
}

// lockEnded reports whether the session has ended, locking its row until the
// transaction ends. found is false when it doesn't exist.
func lockEnded(ctx context.Context, q db.DBTX, id uuid.UUID) (ended, found bool, err error) {
	err = q.QueryRow(ctx,
		"SELECT ended_at IS NOT NULL FROM live_sessions WHERE id = $1 FOR UPDATE", id).Scan(&ended)
	if db.NoRows(err) {
		return false, false, nil
	}
	return ended, err == nil, err
}

// insert returns false when the id already exists.
func insert(ctx context.Context, q db.DBTX, id, userID uuid.UUID, state json.RawMessage) (bool, error) {
	tag, err := q.Exec(ctx,
		`INSERT INTO live_sessions (id, created_by, state) VALUES ($1, $2, $3)
		 ON CONFLICT (id) DO NOTHING`,
		id, userID, []byte(state))
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() == 1, nil
}

// updateState saves a new state if the session is still at `version` and
// running. Returns false when it isn't (another device changed it first, or it ended).
func updateState(ctx context.Context, q db.DBTX, id uuid.UUID, version int32, state json.RawMessage) (bool, error) {
	tag, err := q.Exec(ctx,
		`UPDATE live_sessions SET state = $3, version = version + 1, updated_at = now()
		 WHERE id = $1 AND version = $2 AND ended_at IS NULL`,
		id, version, []byte(state))
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() == 1, nil
}

func markEnded(ctx context.Context, q db.DBTX, id, userID uuid.UUID, savedMatchID *uuid.UUID) error {
	_, err := q.Exec(ctx,
		`UPDATE live_sessions
		 SET ended_at = now(), ended_by = $2, saved_match_id = $3,
		     version = version + 1, updated_at = now()
		 WHERE id = $1`,
		id, userID, savedMatchID)
	return err
}
