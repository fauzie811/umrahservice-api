//go:build integration

// Package integration holds tests that run against a live database (the dev DB
// by default). They are gated behind the `integration` build tag so the normal
// `go test ./...` unit run stays fast and DB-free.
//
// Run with:
//
//	go test -tags=integration ./test/integration/...
//
// Configuration comes from the project-root .env (same file the server uses),
// so DB_HOST/DB_DATABASE/DB_USERNAME/DB_PASSWORD point at the dev DB. If the
// database is unreachable, the tests skip rather than fail.
package integration

import (
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"gorm.io/gorm"

	"umrahservice-api/internal/auth"
	"umrahservice-api/internal/broadcast"
	"umrahservice-api/internal/config"
	"umrahservice-api/internal/db"
	"umrahservice-api/internal/handlers"
	"umrahservice-api/internal/laravel"
	"umrahservice-api/internal/models"
	"umrahservice-api/internal/router"
	"umrahservice-api/internal/storage"
)

// testDatabase is the schema integration tests run against by default. It
// mirrors the app schema (owned by Laravel) but holds throwaway data. Override
// per-run with DB_DATABASE=... go test -tags=integration ...
const testDatabase = "umrahservice_app_test"

// server bundles the wired engine plus its dependencies for a test.
type server struct {
	engine *gin.Engine
	db     *gorm.DB
	cfg    *config.Config
}

// setupServer loads the project .env, opens the (dev) database and builds the
// full Gin engine. It skips the test if the database cannot be reached.
func setupServer(t *testing.T) *server {
	t.Helper()
	gin.SetMode(gin.TestMode)

	// Detect an explicit DB_DATABASE override from the real environment before
	// .env is loaded, so a caller can still target another schema per-run.
	dbOverride, hasOverride := os.LookupEnv("DB_DATABASE")

	// Load the project-root .env so DB credentials match the running service.
	// godotenv.Load does not override variables already set in the environment,
	// so CI/explicit env vars still win.
	_ = godotenv.Load(filepath.Join(projectRoot(t), ".env"))

	cfg := config.Load()

	// Never run integration tests against the app DB by default — use the
	// dedicated test schema unless the caller explicitly overrode DB_DATABASE.
	if hasOverride {
		cfg.DB.Database = dbOverride
	} else {
		cfg.DB.Database = testDatabase
	}

	database, err := db.Open(cfg)
	if err != nil {
		t.Skipf("dev DB unreachable (%s@%s:%s/%s): %v",
			cfg.DB.Username, cfg.DB.Host, cfg.DB.Port, cfg.DB.Database, err)
	}
	// Open is lazy; force a real connection so we skip cleanly when the DB is down.
	if sqlDB, err := database.DB(); err != nil || sqlDB.Ping() != nil {
		t.Skipf("dev DB ping failed (%s@%s:%s/%s)", cfg.DB.Username, cfg.DB.Host, cfg.DB.Port, cfg.DB.Database)
	}

	store, err := storage.New(cfg)
	if err != nil {
		t.Fatalf("storage init failed: %v", err)
	}
	b := broadcast.New(cfg)
	lc := laravel.NewClient(cfg.LaravelURL, cfg.InternalSecret)
	h := handlers.New(database, store, cfg, b, lc)

	return &server{
		engine: router.New(database, h),
		db:     database,
		cfg:    cfg,
	}
}

// do issues an in-process request against the engine and returns the recorder.
func (s *server) do(t *testing.T, method, path, token string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	s.engine.ServeHTTP(rec, req)
	return rec
}

// firstUserID returns the id of an existing user to authenticate as, skipping
// the test if the users table is empty.
func (s *server) firstUserID(t *testing.T) uint64 {
	t.Helper()
	var u models.User
	if err := s.db.Order("id asc").First(&u).Error; err != nil {
		t.Skipf("no users in dev DB to authenticate as: %v", err)
	}
	return u.ID
}

// seedToken issues a real Sanctum-compatible token for userID and registers a
// cleanup that deletes the token row so the dev DB is left untouched.
func (s *server) seedToken(t *testing.T, userID uint64) string {
	t.Helper()
	plain, err := auth.CreateToken(s.db, userID, "integration-test")
	if err != nil {
		t.Fatalf("seed token: %v", err)
	}
	t.Cleanup(func() {
		id, _, _ := cutPipe(plain)
		if id != 0 {
			s.db.Delete(&models.PersonalAccessToken{}, id)
		}
	})
	return plain
}

// cutPipe splits "{id}|{secret}" returning the numeric id.
func cutPipe(token string) (uint64, string, bool) {
	for i := 0; i < len(token); i++ {
		if token[i] == '|' {
			var id uint64
			for _, c := range token[:i] {
				if c < '0' || c > '9' {
					return 0, "", false
				}
				id = id*10 + uint64(c-'0')
			}
			return id, token[i+1:], true
		}
	}
	return 0, "", false
}

// projectRoot walks up from this source file to the directory containing go.mod.
func projectRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot determine caller path")
	}
	dir := filepath.Dir(file)
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.mod not found walking up from test file")
		}
		dir = parent
	}
}
