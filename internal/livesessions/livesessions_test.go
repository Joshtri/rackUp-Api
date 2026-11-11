package livesessions

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestAcceptsASessionState(t *testing.T) {
	out, err := validateState(json.RawMessage(`{ "playerA": "Budi", "playerB": "Andi", "clock": { "running": false } }`))
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != `{"playerA":"Budi","playerB":"Andi","clock":{"running":false}}` {
		t.Fatalf("not compacted: %s", out)
	}
}

func TestRejectsOtherShapes(t *testing.T) {
	for _, bad := range []string{
		`[]`,
		`null`,
		`{ "playerA": "Budi", "clock": {} }`,
		`{ "playerA": "Budi", "playerB": 3, "clock": {} }`,
		`{ "playerA": "Budi", "playerB": "Andi" }`,
	} {
		if _, err := validateState(json.RawMessage(bad)); err == nil {
			t.Errorf("%s should be rejected", bad)
		}
	}
}

func TestRejectsHugeState(t *testing.T) {
	big := strings.Repeat("x", stateMaxBytes)
	state := `{"playerA":"Budi","playerB":"Andi","clock":{},"pad":"` + big + `"}`
	if _, err := validateState(json.RawMessage(state)); err == nil {
		t.Fatal("huge state accepted")
	}
}

func TestEndRequestReadsMatchOrNull(t *testing.T) {
	var discard, empty, save endRequest
	if err := json.Unmarshal([]byte(`{ "match": null }`), &discard); err != nil || discard.Save != nil {
		t.Fatal("null match", err)
	}
	if err := json.Unmarshal([]byte(`{}`), &empty); err != nil || empty.Save != nil {
		t.Fatal("empty body", err)
	}
	err := json.Unmarshal([]byte(`{
		"match": { "date": "2026-11-04", "playerA": "Budi", "playerB": "Andi", "scoreA": 3, "scoreB": 1 }
	}`), &save)
	if err != nil || save.Save == nil || save.Save.Fields.ScoreA != 3 {
		t.Fatal("match", err)
	}
}

func TestRequestsNeedTheirFields(t *testing.T) {
	var c createRequest
	if err := json.Unmarshal([]byte(`{"state": {}}`), &c); err == nil {
		t.Error("create without id accepted")
	}
	var u updateRequest
	if err := json.Unmarshal([]byte(`{"state": {}}`), &u); err == nil {
		t.Error("update without version accepted")
	}
}
