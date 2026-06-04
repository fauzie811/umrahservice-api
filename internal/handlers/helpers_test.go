package handlers

import (
	"testing"
	"time"
)

func TestNormalizeCode(t *testing.T) {
	cases := map[string]string{
		"/MToy":                              "MToy",
		"https://app.test/luggage-tag/MToy":  "MToy",
		"https://app.test/luggage-tag/MToy/": "MToy",
		"MToy":                               "MToy",
		"/api/luggage-tag/abc==":             "abc==",
	}
	for in, want := range cases {
		if got := normalizeCode(in); got != want {
			t.Errorf("normalizeCode(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestStartOfWeek(t *testing.T) {
	// 2026-06-04 is a Thursday; Monday of that week is 2026-06-01.
	thu := time.Date(2026, 6, 4, 10, 0, 0, 0, time.UTC)
	if got := startOfWeek(thu).Format("2006-01-02"); got != "2026-06-01" {
		t.Fatalf("startOfWeek = %s, want 2026-06-01", got)
	}
	// Sunday should map back to the previous Monday.
	sun := time.Date(2026, 6, 7, 10, 0, 0, 0, time.UTC)
	if got := startOfWeek(sun).Format("2006-01-02"); got != "2026-06-01" {
		t.Fatalf("startOfWeek(sun) = %s, want 2026-06-01", got)
	}
}

func TestParseChecklistTruthy(t *testing.T) {
	cases := map[interface{}]bool{
		true: true, false: false,
		"1": true, "true": true, "0": false, "": false,
		float64(1): true, float64(0): false,
	}
	for in, want := range cases {
		if got := truthy(in); got != want {
			t.Errorf("truthy(%v) = %v, want %v", in, got, want)
		}
	}
}
