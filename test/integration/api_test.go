//go:build integration

package integration

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
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

func TestCsrfCookie(t *testing.T) {
	s := setupServer(t)

	rec := s.do(t, http.MethodGet, "/sanctum/csrf-cookie", "")
	if rec.Code != http.StatusNoContent {
		t.Fatalf("GET /sanctum/csrf-cookie = %d, want 204", rec.Code)
	}

	var xsrf *http.Cookie
	for _, ck := range rec.Result().Cookies() {
		if ck.Name == "XSRF-TOKEN" {
			xsrf = ck
		}
	}
	if xsrf == nil {
		t.Fatalf("XSRF-TOKEN cookie not set")
	}
	if xsrf.Value == "" {
		t.Fatalf("XSRF-TOKEN cookie is empty")
	}
	if xsrf.HttpOnly {
		t.Fatalf("XSRF-TOKEN cookie must be readable by JS (HttpOnly=false)")
	}
}

func TestCORSPreflight(t *testing.T) {
	s := setupServer(t)

	req := httptest.NewRequest(http.MethodOptions, "/api/login", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	req.Header.Set("Access-Control-Request-Method", "POST")
	rec := httptest.NewRecorder()
	s.engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("OPTIONS /api/login = %d, want 204", rec.Code)
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "http://localhost:3000" {
		t.Fatalf("Allow-Origin = %q, want reflected origin", got)
	}
	if got := rec.Header().Get("Access-Control-Allow-Credentials"); got != "true" {
		t.Fatalf("Allow-Credentials = %q, want true", got)
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
