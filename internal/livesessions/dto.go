// Package livesessions holds live shot-clock sessions shared between devices.
//
// The server stores the session state as the frontend sends it and only
// guards the order of changes: every update must name the `version` it was
// based on, so two phones tapping at once can't overwrite each other.
// Phones poll GET /live-sessions/{id}?since=<version> to follow along.
package livesessions

import (
	"bytes"
	"encoding/json"
	"time"

	"github.com/google/uuid"

	"github.com/Joshtri/rackup-api/internal/httpx"
	"github.com/Joshtri/rackup-api/internal/matches"
)

// A session's state is the whole tap log, so it can grow during a long
// evening; this is far above what a real session reaches.
const stateMaxBytes = 2_000_000

// LiveSession is a row of `live_sessions`, plus the name of whoever ended it.
type LiveSession struct {
	ID        uuid.UUID
	CreatedBy *uuid.UUID
	// The frontend's `Session` object (src/lib/session.ts), stored as-is.
	State json.RawMessage
	// Goes up by one on every change.
	Version      int32
	EndedAt      *time.Time
	EndedBy      *uuid.UUID
	EndedByName  *string
	SavedMatchID *uuid.UUID
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// createRequest is the body of POST /live-sessions. The client picks the id,
// so a retried create is harmless.
type createRequest struct {
	ID    uuid.UUID       `json:"id"`
	State json.RawMessage `json:"state"`
}

func (c *createRequest) UnmarshalJSON(b []byte) error {
	if err := httpx.RequireFields(b, "id"); err != nil {
		return err
	}
	if err := httpx.RequirePresent(b, "state"); err != nil {
		return err
	}
	type plain createRequest
	return json.Unmarshal(b, (*plain)(c))
}

// updateRequest is the body of PUT /live-sessions/{id}.
type updateRequest struct {
	// The version this change was based on. A different current version → 409.
	Version int32           `json:"version"`
	State   json.RawMessage `json:"state"`
}

func (u *updateRequest) UnmarshalJSON(b []byte) error {
	if err := httpx.RequireFields(b, "version"); err != nil {
		return err
	}
	if err := httpx.RequirePresent(b, "state"); err != nil {
		return err
	}
	type plain updateRequest
	return json.Unmarshal(b, (*plain)(u))
}

// endRequest is the body of POST /live-sessions/{id}/end.
type endRequest struct {
	// The result to save in the history, or null to discard the session.
	// Its id is replaced by the session id, so ending twice saves one match.
	Save *matches.CreateMatchRequest `json:"match"`
}

type sessionResponse struct {
	ID        uuid.UUID       `json:"id"`
	State     json.RawMessage `json:"state"`
	Version   int32           `json:"version"`
	CreatedBy *uuid.UUID      `json:"createdBy"`
	// Epoch ms, only once the session has ended.
	EndedAt     *int64     `json:"endedAt"`
	EndedBy     *uuid.UUID `json:"endedBy"`
	EndedByName *string    `json:"endedByName"`
	// The match saved when the session ended, if it was saved.
	SavedMatchID *uuid.UUID `json:"savedMatchId"`
	CreatedAt    int64      `json:"createdAt"`
	UpdatedAt    int64      `json:"updatedAt"`
}

func toResponse(s *LiveSession) sessionResponse {
	var endedAt *int64
	if s.EndedAt != nil {
		ms := s.EndedAt.UnixMilli()
		endedAt = &ms
	}
	return sessionResponse{
		ID:           s.ID,
		State:        s.State,
		Version:      s.Version,
		CreatedBy:    s.CreatedBy,
		EndedAt:      endedAt,
		EndedBy:      s.EndedBy,
		EndedByName:  s.EndedByName,
		SavedMatchID: s.SavedMatchID,
		CreatedAt:    s.CreatedAt.UnixMilli(),
		UpdatedAt:    s.UpdatedAt.UnixMilli(),
	}
}

// Every response carries the server's clock, so all phones can count the shot
// clock down from the same time even when their own clocks differ.

type oneResponse struct {
	ServerNow int64            `json:"serverNow"`
	Session   *sessionResponse `json:"session,omitempty"`
}

type listResponse struct {
	ServerNow int64             `json:"serverNow"`
	Sessions  []sessionResponse `json:"sessions"`
}

type endResponse struct {
	ServerNow int64                  `json:"serverNow"`
	Session   sessionResponse        `json:"session"`
	Saved     *matches.MatchResponse `json:"match,omitempty"`
}

// validateState checks the state looks like a frontend `Session` and stays
// within size. Returns it compacted, ready to store.
func validateState(state json.RawMessage) (json.RawMessage, error) {
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(state, &obj); err != nil || obj == nil {
		return nil, httpx.Validation("state must be an object")
	}
	for _, key := range []string{"playerA", "playerB"} {
		if !isKind(obj[key], '"') {
			return nil, httpx.Validation("state." + key + " must be a string")
		}
	}
	if !isKind(obj["clock"], '{') {
		return nil, httpx.Validation("state.clock must be an object")
	}
	var buf bytes.Buffer
	if err := json.Compact(&buf, state); err != nil || buf.Len() > stateMaxBytes {
		return nil, httpx.Validation("state is too large")
	}
	return buf.Bytes(), nil
}

// isKind reports whether a JSON value starts with the given byte
// ('"' for a string, '{' for an object).
func isKind(v json.RawMessage, first byte) bool {
	v = bytes.TrimSpace(v)
	return len(v) > 0 && v[0] == first
}
