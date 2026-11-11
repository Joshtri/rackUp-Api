package matches

import (
	"encoding/json"
	"strings"
	"testing"
)

const sampleLog = `{
	"shots": [
		{ "side": "A", "ms": 12000, "end": "continue" },
		{ "side": "A", "ms": 8000, "end": "rack" }
	],
	"racks": [
		{ "winner": "A", "startedAt": 1000, "endedAt": 9000, "shotEnd": 2, "final": null }
	],
	"extensions": { "A": 1, "B": 0 },
	"timeouts": { "A": 0, "B": 2 },
	"startedAt": 1000,
	"endedAt": 20000
}`

func parseLog(t *testing.T, s string) MatchLog {
	t.Helper()
	var l MatchLog
	if err := json.Unmarshal([]byte(s), &l); err != nil {
		t.Fatal(err)
	}
	return l
}

func TestReadsFrontendJSON(t *testing.T) {
	l := parseLog(t, sampleLog)
	if len(l.Shots) != 2 || l.Racks[0].Final != nil || l.Extensions.A != 1 {
		t.Fatalf("parsed wrong: %+v", l)
	}
	// Writes back the same keys the frontend expects.
	out, _ := json.Marshal(l)
	var back map[string]any
	_ = json.Unmarshal(out, &back)
	rack := back["racks"].([]any)[0].(map[string]any)
	if rack["shotEnd"].(float64) != 2 || back["timeouts"].(map[string]any)["B"].(float64) != 2 {
		t.Fatalf("wrong output: %s", out)
	}
	if _, ok := rack["final"]; !ok {
		t.Fatalf("final should be sent as null: %s", out)
	}
}

func TestFlipsEverySide(t *testing.T) {
	l := parseLog(t, sampleLog)
	f := l.Flipped()
	for _, s := range f.Shots {
		if s.Side != SideB {
			t.Fatal("shot not flipped")
		}
	}
	if f.Racks[0].Winner != SideB ||
		f.Extensions != (PerSide{A: 0, B: 1}) ||
		f.Timeouts != (PerSide{A: 2, B: 0}) ||
		f.StartedAt != l.StartedAt {
		t.Fatalf("flipped wrong: %+v", f)
	}
	// The original is untouched.
	if l.Shots[0].Side != SideA {
		t.Fatal("original changed")
	}
	back := f.Flipped()
	a, _ := json.Marshal(back)
	b, _ := json.Marshal(l)
	if string(a) != string(b) {
		t.Fatal("flipping twice should give the original")
	}
}

func TestRejectsBadLogs(t *testing.T) {
	for name, bad := range map[string]string{
		"unknown side":  strings.Replace(sampleLog, `"side": "A", "ms": 12000`, `"side": "C", "ms": 12000`, 1),
		"missing shots": strings.Replace(sampleLog, `"shots"`, `"shotz"`, 1),
		"null racks":    `{"shots":[],"racks":null,"extensions":{"A":0,"B":0},"timeouts":{"A":0,"B":0},"startedAt":1,"endedAt":2}`,
		"bad end":       strings.Replace(sampleLog, `"continue"`, `"stop"`, 1),
	} {
		var l MatchLog
		if err := json.Unmarshal([]byte(bad), &l); err == nil {
			t.Errorf("%s: should be rejected", name)
		}
	}
}

func fields(t *testing.T, a, b string) MatchFields {
	t.Helper()
	var f MatchFields
	body, _ := json.Marshal(map[string]any{"date": "2026-10-31", "playerA": a, "playerB": b, "scoreA": 6, "scoreB": 8})
	if err := json.Unmarshal(body, &f); err != nil {
		t.Fatal(err)
	}
	return f
}

func TestCleansNamesAndNote(t *testing.T) {
	f := fields(t, "  Budi  ", "Andi")
	blank := "   "
	f.Note = &blank
	d, err := f.validate()
	if err != nil || d.PlayerA != "Budi" || d.Note != nil {
		t.Fatalf("got %+v, %v", d, err)
	}
}

func TestRejectsBadInput(t *testing.T) {
	if _, err := fields(t, "Budi", "budi").validate(); err == nil {
		t.Error("same names accepted")
	}
	if _, err := fields(t, "", "Andi").validate(); err == nil {
		t.Error("empty name accepted")
	}
	f := fields(t, "Budi", "Andi")
	f.ScoreA = -1
	if _, err := f.validate(); err == nil {
		t.Error("negative score accepted")
	}
}

func TestParsesCreateRequestFromFrontendShape(t *testing.T) {
	var req CreateMatchRequest
	err := json.Unmarshal([]byte(`{
		"id": "6f1c1b2e-6b8a-4c55-9a0e-2f4f3c1d9b7a",
		"date": "2026-10-31", "playerA": "Budi", "playerB": "Andi",
		"scoreA": 6, "scoreB": 8, "note": "seru", "createdAt": 1793433600000
	}`), &req)
	if err != nil {
		t.Fatal(err)
	}
	if req.ID == nil || *req.CreatedAt != 1793433600000 || req.Fields.ScoreB != 8 || req.Log != nil {
		t.Fatalf("parsed wrong: %+v", req)
	}

	for name, bad := range map[string]string{
		"bad date":      `{"date": "31-10-2026", "playerA": "Budi", "playerB": "Andi", "scoreA": 1, "scoreB": 2}`,
		"missing score": `{"date": "2026-10-31", "playerA": "Budi", "playerB": "Andi", "scoreA": 1}`,
		"float score":   `{"date": "2026-10-31", "playerA": "Budi", "playerB": "Andi", "scoreA": 1.5, "scoreB": 2}`,
	} {
		var r CreateMatchRequest
		if err := json.Unmarshal([]byte(bad), &r); err == nil {
			t.Errorf("%s: should be rejected", name)
		}
	}
}

func TestResponseOmitsEmptyNoteAndLog(t *testing.T) {
	out, _ := json.Marshal(MatchResponse{})
	s := string(out)
	if strings.Contains(s, `"note"`) || strings.Contains(s, `"log"`) {
		t.Fatalf("got %s", s)
	}
}
