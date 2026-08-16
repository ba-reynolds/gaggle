package testutil

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ba-reynolds/vitrilium/internal/api"
	"github.com/ba-reynolds/vitrilium/internal/auth"
	"github.com/ba-reynolds/vitrilium/internal/service"
	"github.com/ba-reynolds/vitrilium/internal/store"
	"github.com/ba-reynolds/vitrilium/pkg/config"
)

const testDBName = "social_test"

var (
	dbOnce   sync.Once
	dbErr    error
	sharedDB *sql.DB
)

// Database returns a connection pool for the test database, creating it and
// applying all migrations on first use. Reuses the local Postgres instance
// configured via DB_* env vars (defaults: localhost:6969, white/teeth).
func Database(t *testing.T) *sql.DB {
	t.Helper()
	dbOnce.Do(func() {
		adminAddr := envOr("TEST_DB_ADDRESS", "localhost:6969")
		adminUser := envOr("TEST_DB_USER", "white")
		adminPass := envOr("TEST_DB_PASSWORD", "teeth")

		adminDSN := fmt.Sprintf("postgres://%s:%s@%s/postgres?sslmode=disable", adminUser, adminPass, adminAddr)
		admin, err := sql.Open("pgx", adminDSN)
		if err != nil {
			dbErr = fmt.Errorf("open admin connection: %w", err)
			return
		}
		defer admin.Close()

		// Drop + recreate a clean test database.
		if _, err := admin.Exec("DROP DATABASE IF EXISTS " + testDBName); err != nil {
			dbErr = fmt.Errorf("drop test db: %w", err)
			return
		}
		if _, err := admin.Exec("CREATE DATABASE " + testDBName); err != nil {
			dbErr = fmt.Errorf("create test db: %w", err)
			return
		}

		testDSN := fmt.Sprintf("postgres://%s:%s@%s/%s?sslmode=disable", adminUser, adminPass, adminAddr, testDBName)
		db, err := sql.Open("pgx", testDSN)
		if err != nil {
			dbErr = fmt.Errorf("open test db: %w", err)
			return
		}
		if err := db.Ping(); err != nil {
			dbErr = fmt.Errorf("ping test db: %w", err)
			return
		}

		if err := applyMigrations(db); err != nil {
			dbErr = fmt.Errorf("apply migrations: %w", err)
			return
		}
		sharedDB = db
	})
	if dbErr != nil {
		t.Fatalf("failed to set up test database: %v", dbErr)
	}
	t.Cleanup(func() {
		sharedDB.Close()
		dbOnce = sync.Once{}
		sharedDB = nil
		dbErr = nil
	})
	return sharedDB
}

// applyMigrations runs every *.up.sql migration in order on a fresh database.
func applyMigrations(db *sql.DB) error {
	migrationsDir, err := filepath.Abs("../../cmd/migrate/migrations")
	if err != nil {
		return err
	}
	entries, err := os.ReadDir(migrationsDir)
	if err != nil {
		return err
	}
	var files []string
	for _, e := range entries {
		name := e.Name()
		if !e.IsDir() && strings.HasSuffix(name, ".up.sql") {
			files = append(files, name)
		}
	}
	sort.Strings(files)

	for _, name := range files {
		body, err := os.ReadFile(filepath.Join(migrationsDir, name))
		if err != nil {
			return err
		}
		if _, err := db.Exec(string(body)); err != nil {
			return fmt.Errorf("migration %s: %w", name, err)
		}
	}
	return nil
}

// App wires the full backend (store + services + router) against the given DB.
type App struct {
	DB     *sql.DB
	Router http.Handler
}

// NewApp builds the application stack without Redis (tests don't depend on it).
func NewApp(t *testing.T, db *sql.DB) *App {
	t.Helper()
	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))
	cfg := config.AllConfigs{
		AppConfig: config.AppConfig{
			MediaDir:               t.TempDir(),
			DefaultPaginationLimit: 20,
			MaxPaginationLimit:     100,
		},
		AuthConfig: config.AuthConfig{
			JWTSecret:                     "test-secret",
			JWTIssuer:                     "gophersocial-test",
			JWTAccessTokenExpirationTime:  15 * time.Minute,
			JWTRefreshTokenExpirationTime: 24 * time.Hour,
		},
	}
	authenticator := auth.NewJWTAuthenticator(cfg.AuthConfig)
	store := store.NewStore(db, log, cfg.AppConfig.MediaDir)
	svc := service.NewService(store, log, authenticator, cfg.AppConfig)
	router := api.NewRouter(svc, log, nil, 0, 0)
	return &App{DB: db, Router: router}
}

// Request is a thin helper for making JSON API calls in tests.
type Request struct {
	Method string
	Path   string
	Token  string
	Body   any
}

// Do performs the request and returns the recorder.
func (a *App) Do(t *testing.T, req Request) *httptest.ResponseRecorder {
	t.Helper()
	var body *strings.Reader
	if req.Body != nil {
		data, err := json.Marshal(req.Body)
		if err != nil {
			t.Fatalf("marshal request body: %v", err)
		}
		body = strings.NewReader(string(data))
	} else {
		body = strings.NewReader("")
	}

	httpReq := httptest.NewRequest(req.Method, req.Path, body)
	if req.Body != nil {
		httpReq.Header.Set("Content-Type", "application/json")
	}
	if req.Token != "" {
		httpReq.Header.Set("Authorization", "Bearer "+req.Token)
	}

	rec := httptest.NewRecorder()
	a.Router.ServeHTTP(rec, httpReq)
	return rec
}

// Decode parses the envelope {data, error} from a response. It returns the data
// and a pointer to the error object (nil when the response succeeded).
func Decode[T any](t *testing.T, rec *httptest.ResponseRecorder) (T, *map[string]any) {
	t.Helper()
	var env struct {
		Data  T               `json:"data"`
		Error *map[string]any `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("decode response %s: %v (body: %s)", rec.Body.String(), err, rec.Body.String())
	}
	return env.Data, env.Error
}

// RegisterUser creates an account via the API and returns the access token.
func (a *App) RegisterUser(t *testing.T, username, email string) string {
	t.Helper()
	rec := a.Do(t, Request{
		Method: http.MethodPost,
		Path:   "/api/v1/auth/register",
		Body: map[string]string{
			"username": username,
			"email":    email,
			"password": "password123",
		},
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("register %s: status %d body %s", username, rec.Code, rec.Body.String())
	}
	data, _ := Decode[map[string]any](t, rec)
	return data["access_token"].(string)
}

// Login returns an access token for the given identifier.
func (a *App) Login(t *testing.T, identifier string) string {
	t.Helper()
	rec := a.Do(t, Request{
		Method: http.MethodPost,
		Path:   "/api/v1/auth/login",
		Body:   map[string]string{"identifier": identifier, "password": "password123"},
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("login %s: status %d body %s", identifier, rec.Code, rec.Body.String())
	}
	data, _ := Decode[map[string]any](t, rec)
	return data["access_token"].(string)
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
