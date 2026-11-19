package matches

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Joshtri/rackup-api/internal/circle"
	"github.com/Joshtri/rackup-api/internal/db"
	"github.com/Joshtri/rackup-api/internal/httpx"
	"github.com/Joshtri/rackup-api/internal/players"
	"github.com/Joshtri/rackup-api/internal/validation"
)

const (
	maxImport = 5_000
	maxShots  = 20_000
	maxRacks  = 2_000
)

// `viewer` / `userID` below is the logged-in account: it decides which
// matches are visible and editable (see package circle), and is recorded as
// the one who saved a new match.

func listAll(ctx context.Context, pool *pgxpool.Pool, viewer uuid.UUID) ([]MatchResponse, error) {
	rows, err := list(ctx, pool, viewer)
	if err != nil {
		return nil, err
	}
	out := make([]MatchResponse, len(rows))
	for i := range rows {
		out[i] = toResponse(&rows[i], viewer)
	}
	return out, nil
}

func get(ctx context.Context, pool *pgxpool.Pool, viewer, id uuid.UUID) (MatchResponse, error) {
	m, err := findVisible(ctx, pool, id, viewer)
	if err != nil {
		return MatchResponse{}, err
	}
	if m == nil {
		return MatchResponse{}, httpx.ErrNotFound
	}
	return toResponse(m, viewer), nil
}

// CreateIn saves an already validated match on q (the pool or a transaction).
// Player names reuse a spelling the user already sees ("budi" → "Budi"),
// like `canonicalName` in the frontend, and are linked to accounts.
//
// If the id already exists, the stored match is returned unchanged (so
// undoing a delete twice is safe). created is true when a new row was added.
func CreateIn(ctx context.Context, q db.DBTX, userID uuid.UUID, m NewMatch) (resp MatchResponse, created bool, err error) {
	if name, ok, err := players.FindSpelling(ctx, q, userID, m.PlayerA); err != nil {
		return resp, false, err
	} else if ok {
		m.PlayerA = name
	}
	if name, ok, err := players.FindSpelling(ctx, q, userID, m.PlayerB); err != nil {
		return resp, false, err
	} else if ok {
		m.PlayerB = name
	}

	row, err := insert(ctx, q, userID, &m)
	if err != nil {
		return resp, false, err
	}
	if row != nil {
		if err := circle.LinkMatches(ctx, q, []uuid.UUID{m.ID}); err != nil {
			return resp, false, err
		}
		if row, err = find(ctx, q, m.ID); err != nil {
			return resp, false, err
		}
		return toResponse(row, userID), true, nil
	}
	existing, err := findVisible(ctx, q, m.ID, userID)
	if err != nil {
		return resp, false, err
	}
	if existing == nil {
		return resp, false, httpx.Conflict("this match id is already in use")
	}
	return toResponse(existing, userID), false, nil
}

// mustEdit returns 404 when the viewer can't see the match and 403 when they
// can see it but didn't record it or play in it.
func mustEdit(m *Match, viewer uuid.UUID) error {
	if m == nil {
		return httpx.ErrNotFound
	}
	if !m.EditableBy(viewer) {
		return httpx.Forbidden("only the players and whoever recorded this match can change it")
	}
	return nil
}

// updateMatch edits a match, like the frontend's `update`:
//   - the shot-clock log can't be edited; it is kept, and flipped if the
//     players swapped sides;
//   - a name typed with different capitalisation is treated as a fix and
//     renamed across all matches.
//   - only the players and whoever recorded it can edit it.
func updateMatch(ctx context.Context, pool *pgxpool.Pool, viewer, id uuid.UUID, fields MatchFields) (MatchResponse, error) {
	data, err := fields.validate()
	if err != nil {
		return MatchResponse{}, err
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		return MatchResponse{}, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	existing, err := findVisibleForUpdate(ctx, tx, id, viewer)
	if err != nil {
		return MatchResponse{}, err
	}
	if err := mustEdit(existing, viewer); err != nil {
		return MatchResponse{}, err
	}
	swapped := validation.SameName(data.PlayerA, existing.PlayerB) &&
		validation.SameName(data.PlayerB, existing.PlayerA)
	log := existing.Log
	if log != nil && swapped {
		flipped := log.Flipped()
		log = &flipped
	}

	if err := update(ctx, tx, id, data, log); err != nil {
		return MatchResponse{}, err
	}
	if err := renamePlayer(ctx, tx, viewer, data.PlayerA); err != nil {
		return MatchResponse{}, err
	}
	if err := renamePlayer(ctx, tx, viewer, data.PlayerB); err != nil {
		return MatchResponse{}, err
	}
	if err := circle.LinkMatches(ctx, tx, []uuid.UUID{id}); err != nil {
		return MatchResponse{}, err
	}
	updated, err := find(ctx, tx, id)
	if err != nil {
		return MatchResponse{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return MatchResponse{}, err
	}
	return toResponse(updated, viewer), nil
}

func remove(ctx context.Context, pool *pgxpool.Pool, viewer, id uuid.UUID) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	existing, err := findVisibleForUpdate(ctx, tx, id, viewer)
	if err != nil {
		return err
	}
	if err := mustEdit(existing, viewer); err != nil {
		return err
	}
	if err := deleteMatch(ctx, tx, id); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// importMatches merges a backup by id: existing ids are skipped, nothing is
// overwritten. The whole import is rejected if any match is invalid.
func importMatches(ctx context.Context, pool *pgxpool.Pool, userID uuid.UUID, req importRequest) (importResponse, error) {
	received := int64(len(req.Matches))
	if received == 0 {
		return importResponse{}, httpx.Validation("matches is empty")
	}
	if received > maxImport {
		return importResponse{}, httpx.Validation(fmt.Sprintf("at most %d matches per import", maxImport))
	}

	seen := map[uuid.UUID]bool{}
	rows := make([]NewMatch, 0, received)
	for i, item := range req.Matches {
		row, err := ToNewMatch(item)
		if err != nil {
			var e *httpx.Error
			if errors.As(err, &e) && e.Code == "validation_error" {
				return importResponse{}, httpx.Validation(fmt.Sprintf("matches[%d]: %s", i, e.Message))
			}
			return importResponse{}, err
		}
		if !seen[row.ID] {
			seen[row.ID] = true
			rows = append(rows, row)
		}
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		return importResponse{}, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck
	ids, err := insertMany(ctx, tx, userID, rows)
	if err != nil {
		return importResponse{}, err
	}
	if err := circle.LinkMatches(ctx, tx, ids); err != nil {
		return importResponse{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return importResponse{}, err
	}
	added := int64(len(ids))
	return importResponse{Added: added, Skipped: received - added}, nil
}

// ToNewMatch validates a create request and turns it into a row to insert.
func ToNewMatch(req CreateMatchRequest) (NewMatch, error) {
	data, err := req.Fields.validate()
	if err != nil {
		return NewMatch{}, err
	}
	if req.Log != nil && (len(req.Log.Shots) > maxShots || len(req.Log.Racks) > maxRacks) {
		return NewMatch{}, httpx.Validation("log is too large")
	}
	createdAt := time.Now()
	if req.CreatedAt != nil {
		createdAt = time.UnixMilli(*req.CreatedAt)
		if y := createdAt.UTC().Year(); y < 1 || y > 9999 {
			return NewMatch{}, httpx.Validation("createdAt is out of range")
		}
	}
	id := uuid.New()
	if req.ID != nil {
		id = *req.ID
	}
	return NewMatch{
		ID:        id,
		PlayedOn:  data.PlayedOn,
		PlayerA:   data.PlayerA,
		PlayerB:   data.PlayerB,
		ScoreA:    data.ScoreA,
		ScoreB:    data.ScoreB,
		Note:      data.Note,
		Log:       req.Log,
		CreatedAt: createdAt,
	}, nil
}
