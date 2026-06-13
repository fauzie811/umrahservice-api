package ratelimit

import (
	"testing"
	"time"
)

func TestHitAllowsUpToMaxThenBlocks(t *testing.T) {
	l := New()

	for i := 1; i <= 5; i++ {
		if ok, _ := l.Hit("k", 5, time.Minute); !ok {
			t.Fatalf("hit %d: want allowed, got blocked", i)
		}
	}

	ok, retry := l.Hit("k", 5, time.Minute)
	if ok {
		t.Fatalf("6th hit: want blocked, got allowed")
	}
	if retry <= 0 {
		t.Fatalf("retryAfter = %v, want > 0", retry)
	}
}

func TestHitKeysAreIndependent(t *testing.T) {
	l := New()

	for i := 0; i < 5; i++ {
		l.Hit("a", 5, time.Minute)
	}
	if ok, _ := l.Hit("a", 5, time.Minute); ok {
		t.Fatalf("key a: want blocked after exceeding max")
	}
	if ok, _ := l.Hit("b", 5, time.Minute); !ok {
		t.Fatalf("key b: want allowed (independent of key a)")
	}
}

func TestHitResetsAfterWindow(t *testing.T) {
	l := New()
	window := 20 * time.Millisecond

	for i := 0; i < 5; i++ {
		l.Hit("k", 5, window)
	}
	if ok, _ := l.Hit("k", 5, window); ok {
		t.Fatalf("want blocked before window reset")
	}

	time.Sleep(window + 5*time.Millisecond)

	if ok, _ := l.Hit("k", 5, window); !ok {
		t.Fatalf("want allowed after window reset")
	}
}
