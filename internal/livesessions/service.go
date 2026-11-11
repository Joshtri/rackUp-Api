package livesessions

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Joshtri/rackup-api/internal/httpx"
	"github.com/Joshtri/rackup-api/internal/matches"
)

// Like the history, live sessions are shared: any logged-in account can
// follow and control any of them.

func listAll(ctx context.Context, pool *pgxpool.Pool) ([]sessionResponse, error) {
	rows, err := listActive(ctx, pool)
	if err != nil {
		return nil, err
	}
	out := make([]sessionResponse, len(rows))
	for i := range rows {
		out[i] = toResponse(&rows[i])
	}
	return out, nil
}

func mustFind(ctx context.Context, pool *pgxpool.Pool, id uuid.UUID) (sessionResponse, error) {
	s, err := find(ctx, pool, id)
	if err != nil {
		return sessionResponse{}, err
	}
	if s == nil {
		return sessionResponse{}, httpx.ErrNotFound
	}
	return toResponse(s), nil
}

// get returns the session, or nil when the caller already has version `since`.
func get(ctx context.Context, pool *pgxpool.Pool, id uuid.UUID, since *int32) (*sessionResponse, error) {
	s, err := mustFind(ctx, pool, id)
	if err != nil {
		return nil, err
	}
	if since != nil && *since == s.Version {
		return nil, nil
	}
	return &s, nil
}

// create creates a session. Repeating it with the same id returns the stored
// one unchanged; created is true when a new row was added.
func create(ctx context.Context, pool *pgxpool.Pool, userID uuid.UUID, req createRequest) (sessionResponse, bool, error) {
	state, err := validateState(req.State)
	if err != nil {
		return sessionResponse{}, false, err
	}
	created, err := insert(ctx, pool, req.ID, userID, state)
	if err != nil {
		return sessionResponse{}, false, err
	}
	s, err := mustFind(ctx, pool, req.ID)
	return s, created, err
}

// update saves a change made on top of req.Version.
// 409 when another device changed the session first or it has ended: the
// client then fetches the latest state and re-applies its tap.
func update(ctx context.Context, pool *pgxpool.Pool, id uuid.UUID, req updateRequest) (sessionResponse, error) {
	state, err := validateState(req.State)
	if err != nil {
		return sessionResponse{}, err
	}
	ok, err := updateState(ctx, pool, id, req.Version, state)
	if err != nil {
		return sessionResponse{}, err
	}
	if !ok {
		current, err := mustFind(ctx, pool, id)
		if err != nil {
			return sessionResponse{}, err
		}
		if current.EndedAt != nil {
			return sessionResponse{}, httpx.Conflict("this session has ended")
		}
		return sessionResponse{}, httpx.Conflict("this session was changed on another device")
	}
	return mustFind(ctx, pool, id)
}

// end ends a session, saving its result in the history when req.Save is set.
// Ending an already ended session changes nothing, so when two phones end
// it at the same time the first one wins and at most one match is saved.
func end(ctx context.Context, pool *pgxpool.Pool, userID, id uuid.UUID, req endRequest) (sessionResponse, *matches.MatchResponse, error) {
	var newMatch *matches.NewMatch
	if req.Save != nil {
		// Same id as the session, so a retried end can't save a second match.
		req.Save.ID = &id
		m, err := matches.ToNewMatch(*req.Save)
		if err != nil {
			return sessionResponse{}, nil, err
		}
		newMatch = &m
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		return sessionResponse{}, nil, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	ended, found, err := lockEnded(ctx, tx, id)
	if err != nil {
		return sessionResponse{}, nil, err
	}
	if !found {
		return sessionResponse{}, nil, httpx.ErrNotFound
	}
	var saved *matches.MatchResponse
	if !ended {
		var savedID *uuid.UUID
		if newMatch != nil {
			m, _, err := matches.CreateIn(ctx, tx, userID, *newMatch)
			if err != nil {
				return sessionResponse{}, nil, err
			}
			saved, savedID = &m, &m.ID
		}
		if err := markEnded(ctx, tx, id, userID, savedID); err != nil {
			return sessionResponse{}, nil, err
		}
	}
	s, err := find(ctx, tx, id)
	if err != nil {
		return sessionResponse{}, nil, err
	}
	if s == nil {
		return sessionResponse{}, nil, httpx.ErrNotFound
	}
	if err := tx.Commit(ctx); err != nil {
		return sessionResponse{}, nil, err
	}
	return toResponse(s), saved, nil
}
