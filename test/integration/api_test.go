//go:build integration

package integration

import (
	"encoding/json"
	"net/http"
	"testing"
)

func TestHealth(t *testing.T) {
	s := setupServer(t)

	rec := s.do(t, http.MethodGet, "/health", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /health = %d, want 200", rec.Code)
	}

	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body["status"] != "ok" {
		t.Fatalf("status = %v, want ok", body["status"])
	}
}

func TestUserRequiresAuth(t *testing.T) {
	s := setupServer(t)

	rec := s.do(t, http.MethodGet, "/api/user", "")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("GET /api/user (no token) = %d, want 401", rec.Code)
	}

	rec = s.do(t, http.MethodGet, "/api/user", "Bearer 999999|bogus")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("GET /api/user (bad token) = %d, want 401", rec.Code)
	}
}

func TestUserAuthenticated(t *testing.T) {
	s := setupServer(t)
	userID := s.firstUserID(t)
	token := s.seedToken(t, userID)

	rec := s.do(t, http.MethodGet, "/api/user", token)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/user = %d, want 200 (body: %s)", rec.Code, rec.Body.String())
	}

	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	// JSON numbers decode to float64; compare numerically.
	if got, ok := body["id"].(float64); !ok || uint64(got) != userID {
		t.Fatalf("user id = %v, want %d", body["id"], userID)
	}
	for _, key := range []string{"email", "roles", "permissions"} {
		if _, ok := body[key]; !ok {
			t.Errorf("user payload missing %q", key)
		}
	}
}

func TestWalletBalance(t *testing.T) {
	s := setupServer(t)
	token := s.seedToken(t, s.firstUserID(t))

	rec := s.do(t, http.MethodGet, "/api/wallet/balance", token)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/wallet/balance = %d, want 200 (body: %s)", rec.Code, rec.Body.String())
	}

	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	for _, key := range []string{"balance", "total_in", "total_out", "currency"} {
		if _, ok := body[key]; !ok {
			t.Errorf("balance payload missing %q", key)
		}
	}
	if body["currency"] != "SAR" {
		t.Errorf("currency = %v, want SAR", body["currency"])
	}
}
