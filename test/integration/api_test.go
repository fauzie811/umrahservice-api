//go:build integration

package integration

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
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

func TestLoginThrottled(t *testing.T) {
	s := setupServer(t)
	body := `{"email":"throttle@example.com","password":"wrong"}`

	last := 0
	for i := 0; i < 6; i++ {
		req := httptest.NewRequest(http.MethodPost, "/api/login", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		s.engine.ServeHTTP(rec, req)
		last = rec.Code

		if i < 5 && rec.Code != http.StatusUnprocessableEntity {
			t.Fatalf("attempt %d = %d, want 422 (body: %s)", i+1, rec.Code, rec.Body.String())
		}
	}

	if last != http.StatusTooManyRequests {
		t.Fatalf("6th attempt = %d, want 429", last)
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

func TestOverview(t *testing.T) {
	s := setupServer(t)

	// Overview is admin/operator-only. Find a user holding one of those roles,
	// skipping if the dev DB has none.
	var userID uint64
	err := s.db.Table("model_has_roles AS mhr").
		Joins("JOIN roles r ON r.id = mhr.role_id").
		Where("mhr.model_type = ? AND r.name IN ?", "user", []string{"Admin", "Admin Operator", "Operator"}).
		Order("mhr.model_id asc").
		Limit(1).
		Pluck("mhr.model_id", &userID).Error
	if err != nil || userID == 0 {
		t.Skip("no admin/operator user in dev DB to authenticate as")
	}
	token := s.seedToken(t, userID)

	rec := s.do(t, http.MethodGet, "/api/overview", token)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/overview = %d, want 200 (body: %s)", rec.Code, rec.Body.String())
	}

	var body struct {
		Data struct {
			Tasks struct {
				Active  *float64 `json:"active"`
				Overdue *float64 `json:"overdue"`
			} `json:"tasks"`
			Incidents struct {
				Active     *float64           `json:"active"`
				BySeverity map[string]float64 `json:"by_severity"`
				ByStatus   map[string]float64 `json:"by_status"`
			} `json:"incidents"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body.Data.Tasks.Active == nil || body.Data.Tasks.Overdue == nil {
		t.Errorf("tasks counts missing: %s", rec.Body.String())
	}
	if body.Data.Incidents.Active == nil {
		t.Errorf("incidents.active missing: %s", rec.Body.String())
	}
	for _, key := range []string{"low", "medium", "high", "critical"} {
		if _, ok := body.Data.Incidents.BySeverity[key]; !ok {
			t.Errorf("incidents.by_severity missing %q", key)
		}
	}
	for _, key := range []string{"open", "in_progress"} {
		if _, ok := body.Data.Incidents.ByStatus[key]; !ok {
			t.Errorf("incidents.by_status missing %q", key)
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
