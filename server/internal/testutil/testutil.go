package testutil

import (
	"bytes"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
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

	"github.com/ba-reynolds/gaggle/internal/api"
	"github.com/ba-reynolds/gaggle/internal/auth"
	"github.com/ba-reynolds/gaggle/internal/service"
	"github.com/ba-reynolds/gaggle/internal/store"
	"github.com/ba-reynolds/gaggle/pkg/config"
)

// testDBName returns a database name unique to the running test binary, so
// `go test ./...` (which runs each package as a separate process, in parallel)
// does not have packages racing to DROP/CREATE the same social_test database.
func testDBName() string {
	sum := sha256.Sum256([]byte(os.Args[0]))
	return fmt.Sprintf("social_test_%x", sum[:4])
}

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
	name := testDBName()
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
		if _, err := admin.Exec("DROP DATABASE IF EXISTS " + name); err != nil {
			dbErr = fmt.Errorf("drop test db: %w", err)
			return
		}
		if _, err := admin.Exec("CREATE DATABASE " + name); err != nil {
			dbErr = fmt.Errorf("create test db: %w", err)
			return
		}

		testDSN := fmt.Sprintf("postgres://%s:%s@%s/%s?sslmode=disable", adminUser, adminPass, adminAddr, name)
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
	DB      *sql.DB
	Router  http.Handler
	Service *service.Service
}

// NewApp builds the application stack without Redis (tests don't depend on it).
func NewApp(t *testing.T, db *sql.DB) *App {
	return NewAppWithCookieSecure(t, db, false)
}

// NewAppWithCookieSecure is like NewApp but lets the test choose the refresh
// cookie's configured Secure fallback (what compose COOKIE_SECURE would set).
func NewAppWithCookieSecure(t *testing.T, db *sql.DB, cookieSecure bool) *App {
	t.Helper()
	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))
	cfg := config.AllConfigs{
		AppConfig: config.AppConfig{
			MediaDir:               t.TempDir(),
			DefaultPaginationLimit: 20,
			MaxPaginationLimit:     100,
			MailIntakeSecret:       "test-intake-secret",
		},
		AuthConfig: config.AuthConfig{
			JWTSecret:                     "test-secret",
			JWTIssuer:                     "gaggle-test",
			JWTAccessTokenExpirationTime:  15 * time.Minute,
			JWTRefreshTokenExpirationTime: 24 * time.Hour,
		},
	}
	authenticator := auth.NewJWTAuthenticator(cfg.AuthConfig)
	store := store.NewStore(db, log, cfg.AppConfig.MediaDir)
	svc := service.NewService(store, log, authenticator, cfg.AppConfig)
	router := api.NewRouter(svc, log, nil, 0, 0, cookieSecure)
	return &App{DB: db, Router: router, Service: svc}
}

// Request is a thin helper for making JSON API calls in tests.
type Request struct {
	Method  string
	Path    string
	Token   string
	Body    any
	RawBody []byte // sent verbatim (e.g. raw MIME); overrides Body
	Cookies []*http.Cookie
	Headers map[string]string
	UA      string
}

// Do performs the request and returns the recorder.
func (a *App) Do(t *testing.T, req Request) *httptest.ResponseRecorder {
	t.Helper()
	var body io.Reader = strings.NewReader("")
	if req.RawBody != nil {
		body = bytes.NewReader(req.RawBody)
	} else if req.Body != nil {
		data, err := json.Marshal(req.Body)
		if err != nil {
			t.Fatalf("marshal request body: %v", err)
		}
		body = strings.NewReader(string(data))
	}

	httpReq := httptest.NewRequest(req.Method, req.Path, body)
	if req.Body != nil {
		httpReq.Header.Set("Content-Type", "application/json")
	}
	if req.Token != "" {
		httpReq.Header.Set("Authorization", "Bearer "+req.Token)
	}
	for k, v := range req.Headers {
		httpReq.Header.Set(k, v)
	}
	if req.UA != "" {
		httpReq.Header.Set("User-Agent", req.UA)
	}
	for _, c := range req.Cookies {
		httpReq.AddCookie(c)
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
