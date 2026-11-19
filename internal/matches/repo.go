// Package matches is the match history: create, list, edit, delete, import a backup.
//
// An account sees the matches of its circle (itself and its friends) and
// edits the ones it recorded or played in; see package circle.
// `user_id` is written on insert, to remember who recorded it.
package matches

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/Joshtri/rackup-api/internal/circle"
	"github.com/Joshtri/rackup-api/internal/db"
)

// Match is a row of the `matches` table.
type Match struct {
	ID        uuid.UUID
	UserID    uuid.UUID
	PlayedOn  time.Time
	PlayerA   string
	PlayerB   string
	ScoreA    int32
	ScoreB    int32
	Note      *string
	Log       *MatchLog
	CreatedAt time.Time
	UpdatedAt time.Time
	// Accounts linked to the player names, nil for guests.
	PlayerAUser *uuid.UUID
	PlayerBUser *uuid.UUID
}

// EditableBy reports whether the account recorded the match or played in it.
func (m *Match) EditableBy(user uuid.UUID) bool {
	return m.UserID == user ||
		(m.PlayerAUser != nil && *m.PlayerAUser == user) ||
		(m.PlayerBUser != nil && *m.PlayerBUser == user)
}

// NewMatch is a validated match ready to insert.
type NewMatch struct {
	ID        uuid.UUID
	PlayedOn  time.Time
	PlayerA   string
	PlayerB   string
	ScoreA    int32
	ScoreB    int32
	Note      *string
	Log       *MatchLog
	CreatedAt time.Time
}

const matchColumns = "id, user_id, played_on, player_a, player_b, score_a, score_b, note, log, created_at, updated_at, player_a_user, player_b_user"

func scanMatch(row pgx.Row) (*Match, error) {
	var m Match
	var log []byte
	err := row.Scan(&m.ID, &m.UserID, &m.PlayedOn, &m.PlayerA, &m.PlayerB,
		&m.ScoreA, &m.ScoreB, &m.Note, &log, &m.CreatedAt, &m.UpdatedAt, &m.PlayerAUser, &m.PlayerBUser)
	if db.NoRows(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if log != nil {
		m.Log = &MatchLog{}
		if err := json.Unmarshal(log, m.Log); err != nil {
			return nil, err
		}
	}
	return &m, nil
}

// logParam is the jsonb value for a log, or NULL.
func logParam(l *MatchLog) (any, error) {
	if l == nil {
		return nil, nil
	}
	return json.Marshal(l)
}

// list returns the matches the viewer can see, newest first.
func list(ctx context.Context, q db.DBTX, viewer uuid.UUID) ([]Match, error) {
	rows, err := q.Query(ctx, "SELECT "+matchColumns+" FROM matches m WHERE "+circle.MatchVisible("m", "$1")+
		" ORDER BY played_on DESC, created_at DESC", viewer)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Match{}
	for rows.Next() {
		m, err := scanMatch(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *m)
	}
	return out, rows.Err()
}

func find(ctx context.Context, q db.DBTX, id uuid.UUID) (*Match, error) {
	return scanMatch(q.QueryRow(ctx, "SELECT "+matchColumns+" FROM matches WHERE id = $1", id))
}

// findVisible returns the match, or nil when it doesn't exist or the viewer can't see it.
func findVisible(ctx context.Context, q db.DBTX, id, viewer uuid.UUID) (*Match, error) {
	return scanMatch(q.QueryRow(ctx, "SELECT "+matchColumns+" FROM matches m WHERE m.id = $1 AND "+
		circle.MatchVisible("m", "$2"), id, viewer))
}

// findVisibleForUpdate is findVisible, locking the row until the transaction ends.
func findVisibleForUpdate(ctx context.Context, q db.DBTX, id, viewer uuid.UUID) (*Match, error) {
	return scanMatch(q.QueryRow(ctx, "SELECT "+matchColumns+" FROM matches m WHERE m.id = $1 AND "+
		circle.MatchVisible("m", "$2")+" FOR UPDATE", id, viewer))
}

// insert saves a match recorded by userID. Returns nil when the id already exists.
func insert(ctx context.Context, q db.DBTX, userID uuid.UUID, m *NewMatch) (*Match, error) {
	log, err := logParam(m.Log)
	if err != nil {
		return nil, err
	}
	return scanMatch(q.QueryRow(ctx,
		`INSERT INTO matches
		   (id, user_id, played_on, player_a, player_b, score_a, score_b, note, log, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		 ON CONFLICT (id) DO NOTHING
		 RETURNING `+matchColumns,
		m.ID, userID, m.PlayedOn, m.PlayerA, m.PlayerB, m.ScoreA, m.ScoreB, m.Note, log, m.CreatedAt))
}

// importRow is a NewMatch as JSON for jsonb_to_recordset. Field names must
// match the column list in insertMany.
type importRow struct {
	ID        uuid.UUID `json:"id"`
	PlayedOn  Date      `json:"played_on"`
	PlayerA   string    `json:"player_a"`
	PlayerB   string    `json:"player_b"`
	ScoreA    int32     `json:"score_a"`
	ScoreB    int32     `json:"score_b"`
	Note      *string   `json:"note"`
	Log       *MatchLog `json:"log"`
	CreatedAt time.Time `json:"created_at"`
}

// insertMany inserts many matches in one round trip, recorded by userID.
// Existing ids are skipped. Returns the ids that were added.
func insertMany(ctx context.Context, q db.DBTX, userID uuid.UUID, ms []NewMatch) ([]uuid.UUID, error) {
	if len(ms) == 0 {
		return nil, nil
	}
	rows := make([]importRow, len(ms))
	for i, m := range ms {
		rows[i] = importRow{m.ID, Date{m.PlayedOn}, m.PlayerA, m.PlayerB, m.ScoreA, m.ScoreB, m.Note, m.Log, m.CreatedAt}
	}
	data, err := json.Marshal(rows)
	if err != nil {
		return nil, err
	}
	res, err := q.Query(ctx,
		`INSERT INTO matches
		   (id, user_id, played_on, player_a, player_b, score_a, score_b, note, log, created_at)
		 SELECT x.id, $1, x.played_on, x.player_a, x.player_b, x.score_a, x.score_b,
		        x.note, x.log, x.created_at
		 FROM jsonb_to_recordset($2::jsonb) AS x(
		   id UUID, played_on DATE, player_a TEXT, player_b TEXT, score_a INTEGER,
		   score_b INTEGER, note TEXT, log JSONB, created_at TIMESTAMPTZ)
		 ON CONFLICT (id) DO NOTHING
		 RETURNING id`,
		userID, data)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(res, pgx.RowTo[uuid.UUID])
}

func update(ctx context.Context, q db.DBTX, id uuid.UUID, d matchData, l *MatchLog) error {
	log, err := logParam(l)
	if err != nil {
		return err
	}
	_, err = q.Exec(ctx,
		`UPDATE matches
		 SET played_on = $2, player_a = $3, player_b = $4, score_a = $5, score_b = $6,
		     note = $7, log = $8, updated_at = now()
		 WHERE id = $1`,
		id, d.PlayedOn, d.PlayerA, d.PlayerB, d.ScoreA, d.ScoreB, d.Note, log)
	return err
}

// renamePlayer rewrites every spelling of name that differs only in case
// ("budi" → "Budi") across the matches the viewer can see.
func renamePlayer(ctx context.Context, q db.DBTX, viewer uuid.UUID, name string) error {
	_, err := q.Exec(ctx,
		`UPDATE matches
		 SET player_a = CASE WHEN lower(player_a) = lower($1) THEN $1 ELSE player_a END,
		     player_b = CASE WHEN lower(player_b) = lower($1) THEN $1 ELSE player_b END,
		     updated_at = now()
		 WHERE ((lower(player_a) = lower($1) AND player_a <> $1)
		     OR (lower(player_b) = lower($1) AND player_b <> $1))
		   AND `+circle.MatchVisible("matches", "$2"),
		name, viewer)
	return err
}

func deleteMatch(ctx context.Context, q db.DBTX, id uuid.UUID) error {
	_, err := q.Exec(ctx, "DELETE FROM matches WHERE id = $1", id)
	return err
}
