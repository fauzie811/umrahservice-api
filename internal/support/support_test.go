package support

import (
	"testing"
	"time"
)

func TestHashToken(t *testing.T) {
	// sha256("hello") known vector.
	want := "2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824"
	if got := HashToken("hello"); got != want {
		t.Fatalf("HashToken = %s, want %s", got, want)
	}
}

func TestRandomTokenLength(t *testing.T) {
	tok := RandomToken()
	if len(tok) != 40 {
		t.Fatalf("RandomToken length = %d, want 40", len(tok))
	}
	if RandomToken() == RandomToken() {
		t.Fatal("RandomToken should not repeat")
	}
}

func TestISO8601(t *testing.T) {
	tm := time.Date(2026, 6, 4, 13, 29, 0, 0, time.UTC)
	got := ISO8601(&tm)
	if got == nil || *got != "2026-06-04T13:29:00+00:00" {
		t.Fatalf("ISO8601 = %v, want 2026-06-04T13:29:00+00:00", got)
	}
	if ISO8601(nil) != nil {
		t.Fatal("nil time should yield nil")
	}
}
