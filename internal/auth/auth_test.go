package auth

import (
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

const secret = "test-secret-that-is-at-least-32-chars"

func TestTokenRoundTrip(t *testing.T) {
	id := uuid.New()
	token, err := IssueToken(id, secret, time.Now().Unix()+3600)
	if err != nil {
		t.Fatal(err)
	}
	got, err := VerifyToken(token, secret)
	if err != nil || got != id {
		t.Fatalf("got %v, %v", got, err)
	}
}

func TestRejectsWrongSecretAndExpiredToken(t *testing.T) {
	token, _ := IssueToken(uuid.New(), secret, time.Now().Unix()+3600)
	if _, err := VerifyToken(token, "another-secret-that-is-32-chars-long"); err == nil {
		t.Fatal("wrong secret accepted")
	}
	expired, _ := IssueToken(uuid.New(), secret, 1_000)
	if _, err := VerifyToken(expired, secret); err == nil {
		t.Fatal("expired token accepted")
	}
}

func TestTokenNeverOutlivesTheAccount(t *testing.T) {
	now := time.Now()
	soon := now.Add(3 * 24 * time.Hour)
	if got := tokenExpiry(now, 30, soon); got != soon.Unix() {
		t.Fatalf("got %d, want %d", got, soon.Unix())
	}
	later := now.Add(60 * 24 * time.Hour)
	if got := tokenExpiry(now, 30, later); got != now.Unix()+30*86_400 {
		t.Fatalf("got %d", got)
	}
}

func TestPasswordHashRoundTrip(t *testing.T) {
	hash, err := HashPassword("correct horse")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(hash, "$argon2id$v=19$m=19456,t=2,p=1$") {
		t.Fatalf("unexpected hash format %q", hash)
	}
	if ok, err := VerifyPassword("correct horse", hash); !ok || err != nil {
		t.Fatal("right password rejected", err)
	}
	if ok, _ := VerifyPassword("wrong", hash); ok {
		t.Fatal("wrong password accepted")
	}
	if _, err := VerifyPassword("x", "garbage"); err == nil {
		t.Fatal("garbage hash accepted")
	}
}

// A hash made by the old Rust version (argon2 crate). Accounts created
// before the move to Go must still log in.
func TestVerifiesRustHash(t *testing.T) {
	const rustHash = "$argon2id$v=19$m=19456,t=2,p=1$zHI8sfQWhbilS95UYJyI9g$GhrTXAzbF/BiusW4NsEZ8giBEFYvtzq0AxlVPfA9BUg"
	if ok, err := VerifyPassword("MEUZR-HRqmJ-qVutD-QZ59Y", rustHash); !ok || err != nil {
		t.Fatal("Rust hash rejected", err)
	}
	if ok, _ := VerifyPassword("MEUZR-HRqmJ-qVutD-QZ59X", rustHash); ok {
		t.Fatal("wrong password accepted")
	}
}

func TestGeneratedPasswordsHaveTheRightShapeAndDiffer(t *testing.T) {
	a, _ := GeneratePassword()
	b, _ := GeneratePassword()
	if a == b {
		t.Fatal("passwords repeat")
	}
	groups := strings.Split(a, "-")
	if len(groups) != passwordGroups {
		t.Fatalf("got %d groups", len(groups))
	}
	for _, g := range groups {
		if len(g) != passwordGroupLen {
			t.Fatalf("group %q has the wrong length", g)
		}
		for _, c := range g {
			if !strings.ContainsRune(passwordAlphabet, c) {
				t.Fatalf("unexpected character %q", c)
			}
		}
	}
}

func TestNormalizesUsernames(t *testing.T) {
	if got, _ := normalizeUsername("  JoshTri "); got != "joshtri" {
		t.Fatalf("got %q", got)
	}
	if got, _ := normalizeUsername("jeremy.p_2-x"); got != "jeremy.p_2-x" {
		t.Fatalf("got %q", got)
	}
	for _, bad := range []string{"", "ab", "-abc", "has space", "émile", strings.Repeat("a", 33)} {
		if _, err := normalizeUsername(bad); err == nil {
			t.Errorf("%q should be rejected", bad)
		}
	}
}

func TestChecksDays(t *testing.T) {
	if validDays(30) != nil || validDays(0) == nil || validDays(-5) == nil {
		t.Fatal("validDays is wrong")
	}
}
