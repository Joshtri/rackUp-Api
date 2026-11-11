package matches

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/Joshtri/rackup-api/internal/httpx"
	"github.com/Joshtri/rackup-api/internal/validation"
)

const (
	nameMax  = 50
	noteMax  = 500
	scoreMax = 999
)

// Date is a calendar day, sent as "YYYY-MM-DD".
type Date struct{ time.Time }

const dateLayout = "2006-01-02"

func (d Date) MarshalJSON() ([]byte, error) {
	return []byte(`"` + d.Format(dateLayout) + `"`), nil
}

func (d *Date) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	t, err := time.Parse(dateLayout, s)
	if err != nil {
		return fmt.Errorf("invalid date %q, expected YYYY-MM-DD", s)
	}
	d.Time = t
	return nil
}

// MatchFields are the editable fields. Body of PUT /matches/{id}.
type MatchFields struct {
	Date    Date    `json:"date"`
	PlayerA string  `json:"playerA"`
	PlayerB string  `json:"playerB"`
	ScoreA  int32   `json:"scoreA"`
	ScoreB  int32   `json:"scoreB"`
	Note    *string `json:"note"`
}

func (f *MatchFields) UnmarshalJSON(b []byte) error {
	if err := httpx.RequireFields(b, "date", "playerA", "playerB", "scoreA", "scoreB"); err != nil {
		return err
	}
	type plain MatchFields
	return json.Unmarshal(b, (*plain)(f))
}

// CreateMatchRequest is the body of POST /matches, and each item of an import.
type CreateMatchRequest struct {
	// Optional client id, so a deleted match can be restored with the same id.
	ID *uuid.UUID
	// Epoch ms. Defaults to now.
	CreatedAt *int64
	// Only for matches played with the shot clock.
	Log    *MatchLog
	Fields MatchFields
}

func (c *CreateMatchRequest) UnmarshalJSON(b []byte) error {
	var extra struct {
		ID        *uuid.UUID `json:"id"`
		CreatedAt *int64     `json:"createdAt"`
		Log       *MatchLog  `json:"log"`
	}
	if err := json.Unmarshal(b, &extra); err != nil {
		return err
	}
	if err := json.Unmarshal(b, &c.Fields); err != nil {
		return err
	}
	c.ID, c.CreatedAt, c.Log = extra.ID, extra.CreatedAt, extra.Log
	return nil
}

type importRequest struct {
	Matches []CreateMatchRequest `json:"matches"`
}

func (r *importRequest) UnmarshalJSON(b []byte) error {
	if err := httpx.RequireFields(b, "matches"); err != nil {
		return err
	}
	type plain importRequest
	return json.Unmarshal(b, (*plain)(r))
}

type importResponse struct {
	// New matches saved.
	Added int64 `json:"added"`
	// Matches whose id already existed.
	Skipped int64 `json:"skipped"`
}

// MatchResponse has the same shape as `Match` in the frontend (src/lib/matches.ts), plus updatedAt.
type MatchResponse struct {
	ID        uuid.UUID `json:"id"`
	Date      Date      `json:"date"`
	PlayerA   string    `json:"playerA"`
	PlayerB   string    `json:"playerB"`
	ScoreA    int32     `json:"scoreA"`
	ScoreB    int32     `json:"scoreB"`
	Note      *string   `json:"note,omitempty"`
	Log       *MatchLog `json:"log,omitempty"`
	CreatedAt int64     `json:"createdAt"`
	UpdatedAt int64     `json:"updatedAt"`
}

func toResponse(m *Match) MatchResponse {
	return MatchResponse{
		ID:        m.ID,
		Date:      Date{m.PlayedOn},
		PlayerA:   m.PlayerA,
		PlayerB:   m.PlayerB,
		ScoreA:    m.ScoreA,
		ScoreB:    m.ScoreB,
		Note:      m.Note,
		Log:       m.Log,
		CreatedAt: m.CreatedAt.UnixMilli(),
		UpdatedAt: m.UpdatedAt.UnixMilli(),
	}
}

// matchData is MatchFields after validation and cleanup.
type matchData struct {
	PlayedOn time.Time
	PlayerA  string
	PlayerB  string
	ScoreA   int32
	ScoreB   int32
	Note     *string
}

func (f MatchFields) validate() (matchData, error) {
	a := validation.CleanName(f.PlayerA)
	b := validation.CleanName(f.PlayerB)
	if err := validation.RequireText("playerA", a, nameMax); err != nil {
		return matchData{}, err
	}
	if err := validation.RequireText("playerB", b, nameMax); err != nil {
		return matchData{}, err
	}
	if validation.SameName(a, b) {
		return matchData{}, httpx.Validation("playerA and playerB must be different")
	}
	for _, s := range []struct {
		field string
		score int32
	}{{"scoreA", f.ScoreA}, {"scoreB", f.ScoreB}} {
		if s.score < 0 || s.score > scoreMax {
			return matchData{}, httpx.Validation(fmt.Sprintf("%s must be between 0 and %d", s.field, scoreMax))
		}
	}
	var note *string
	if f.Note != nil {
		if n := strings.TrimSpace(*f.Note); n != "" {
			if utf8.RuneCountInString(n) > noteMax {
				return matchData{}, httpx.Validation(fmt.Sprintf("note must be at most %d characters", noteMax))
			}
			note = &n
		}
	}
	return matchData{PlayedOn: f.Date.Time, PlayerA: a, PlayerB: b, ScoreA: f.ScoreA, ScoreB: f.ScoreB, Note: note}, nil
}
