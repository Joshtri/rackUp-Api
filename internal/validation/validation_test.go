package validation

import "testing"

func TestCleansWhitespace(t *testing.T) {
	if got := CleanName("  Budi \t  Santoso "); got != "Budi Santoso" {
		t.Fatalf("got %q", got)
	}
	if got := CleanName("   "); got != "" {
		t.Fatalf("got %q", got)
	}
}

func TestComparesNamesIgnoringCase(t *testing.T) {
	if !SameName("budi", "BUDI") || SameName("budi", "andi") {
		t.Fatal("SameName is wrong")
	}
}

func TestChecksLengthInChars(t *testing.T) {
	if RequireText("name", "", 5) == nil {
		t.Fatal("empty should fail")
	}
	if RequireText("name", "ééééé", 5) != nil {
		t.Fatal("5 chars should pass")
	}
	if RequireText("name", "éééééé", 5) == nil {
		t.Fatal("6 chars should fail")
	}
}
