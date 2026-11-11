package matches

import (
	"encoding/json"
	"fmt"

	"github.com/Joshtri/rackup-api/internal/httpx"
)

// The types below mirror `SessionLog` / `MatchLog` in the frontend
// (src/lib/session.ts and src/lib/matches.ts). Keep the JSON shape in sync.

type Side string

const (
	SideA Side = "A"
	SideB Side = "B"
)

func (s Side) Flip() Side {
	if s == SideA {
		return SideB
	}
	return SideA
}

func (s *Side) UnmarshalJSON(b []byte) error {
	v, err := oneOf(b, "A", "B")
	*s = Side(v)
	return err
}

// PerSide is a value per player side, sent as {"A": .., "B": ..}.
type PerSide struct {
	A int64 `json:"A"`
	B int64 `json:"B"`
}

func (p PerSide) Flipped() PerSide { return PerSide{A: p.B, B: p.A} }

func (p *PerSide) UnmarshalJSON(b []byte) error {
	if err := httpx.RequireFields(b, "A", "B"); err != nil {
		return err
	}
	type plain PerSide
	return json.Unmarshal(b, (*plain)(p))
}

type ShotEnd string

func (e *ShotEnd) UnmarshalJSON(b []byte) error {
	v, err := oneOf(b, "continue", "turnover", "rack")
	*e = ShotEnd(v)
	return err
}

type Shot struct {
	Side Side `json:"side"`
	// Shot clock time used, in ms.
	Ms  float64 `json:"ms"`
	End ShotEnd `json:"end"`
}

func (s *Shot) UnmarshalJSON(b []byte) error {
	if err := httpx.RequireFields(b, "side", "ms", "end"); err != nil {
		return err
	}
	type plain Shot
	return json.Unmarshal(b, (*plain)(s))
}

type RackFinal string

func (f *RackFinal) UnmarshalJSON(b []byte) error {
	v, err := oneOf(b, "added", "marked")
	*f = RackFinal(v)
	return err
}

type RackLog struct {
	Winner    Side       `json:"winner"`
	StartedAt int64      `json:"startedAt"`
	EndedAt   int64      `json:"endedAt"`
	ShotEnd   int64      `json:"shotEnd"`
	Final     *RackFinal `json:"final"`
}

func (r *RackLog) UnmarshalJSON(b []byte) error {
	if err := httpx.RequireFields(b, "winner", "startedAt", "endedAt", "shotEnd"); err != nil {
		return err
	}
	type plain RackLog
	return json.Unmarshal(b, (*plain)(r))
}

// MatchLog is the tap log of a live shot-clock session, kept for the stats.
type MatchLog struct {
	Shots      []Shot    `json:"shots"`
	Racks      []RackLog `json:"racks"`
	Extensions PerSide   `json:"extensions"`
	Timeouts   PerSide   `json:"timeouts"`
	StartedAt  int64     `json:"startedAt"`
	EndedAt    int64     `json:"endedAt"`
}

func (l *MatchLog) UnmarshalJSON(b []byte) error {
	if err := httpx.RequireFields(b, "shots", "racks", "extensions", "timeouts", "startedAt", "endedAt"); err != nil {
		return err
	}
	type plain MatchLog
	return json.Unmarshal(b, (*plain)(l))
}

// Flipped is the same log seen from the other side. Used when players A and B swap.
func (l MatchLog) Flipped() MatchLog {
	out := l
	out.Shots = make([]Shot, len(l.Shots))
	for i, s := range l.Shots {
		s.Side = s.Side.Flip()
		out.Shots[i] = s
	}
	out.Racks = make([]RackLog, len(l.Racks))
	for i, r := range l.Racks {
		r.Winner = r.Winner.Flip()
		out.Racks[i] = r
	}
	out.Extensions = l.Extensions.Flipped()
	out.Timeouts = l.Timeouts.Flipped()
	return out
}

// oneOf reads a JSON string that must be one of the allowed values.
func oneOf(b []byte, allowed ...string) (string, error) {
	var v string
	if err := json.Unmarshal(b, &v); err != nil {
		return "", err
	}
	for _, a := range allowed {
		if v == a {
			return v, nil
		}
	}
	return "", fmt.Errorf("unknown variant %q, expected one of %q", v, allowed)
}
