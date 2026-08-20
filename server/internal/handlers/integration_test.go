package handlers_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/ba-reynolds/gaggle/internal/testutil"
)

func itoa(i int) string { return strconv.Itoa(i) }

func refreshCookieFrom(rec *httptest.ResponseRecorder) string {
	for _, c := range rec.Result().Cookies() {
		if c.Name == "refresh_token" {
			return c.Value
		}
	}
	return ""
}

func doRefresh(t *testing.T, app *testutil.App, cookieValue string) *httptest.ResponseRecorder {
	t.Helper()
	return doRefreshUA(t, app, cookieValue, "")
}

func doRefreshUA(t *testing.T, app *testutil.App, cookieValue, ua string) *httptest.ResponseRecorder {
	t.Helper()
	return app.Do(t, testutil.Request{
		Method:  http.MethodPost,
		Path:    "/api/v1/auth/refresh-token",
		Cookies: []*http.Cookie{{Name: "refresh_token", Value: cookieValue}},
		UA:      ua,
	})
}

// TestRefreshTokenRotation exercises rotation: each refresh issues a new
// refresh token and revokes the previous one. Replaying an already-rotated
// token from the SAME device/user-agent is the benign signature of concurrent
// tabs (they both held the same cookie while one rotation already landed), so
// the session survives and the family's current token is rotated forward.
func TestRefreshTokenRotation(t *testing.T) {
	app := testutil.NewApp(t, testutil.Database(t))

	const ua = "gaggle-browser/1.0"

	reg := app.Do(t, testutil.Request{
		Method: http.MethodPost,
		Path:   "/api/v1/auth/register",
		UA:     ua,
		Body:   map[string]string{"username": "rotator", "email": "rotator@example.com", "password": "password123"},
	})
	if reg.Code != http.StatusCreated {
		t.Fatalf("register: status %d body %s", reg.Code, reg.Body.String())
	}
	r1 := refreshCookieFrom(reg)
	if r1 == "" {
		t.Fatal("register response missing refresh_token cookie")
	}

	// A refresh rotates the token: the response must hand back a NEW refresh
	// cookie (this is the wiring the old code never did).
	rec := doRefreshUA(t, app, r1, ua)
	if rec.Code != http.StatusOK {
		t.Fatalf("refresh: status %d body %s", rec.Code, rec.Body.String())
	}
	r2 := refreshCookieFrom(rec)
	if r2 == "" || r2 == r1 {
		t.Fatalf("refresh must rotate the refresh token (r1=%q)", r2)
	}

	// The rotated token should also still work (chain keeps extending).
	rec = doRefreshUA(t, app, r2, ua)
	if rec.Code != http.StatusOK {
		t.Fatalf("refresh with r2: status %d body %s", rec.Code, rec.Body.String())
	}
	r3 := refreshCookieFrom(rec)
	if r3 == r2 {
		t.Fatal("second refresh did not rotate")
	}

	// Replaying the already-rotated r1 from the same device/tab context is a
	// benign stale replay (both tabs held the same cookie; one rotation already
	// landed). The session must survive: the family's current token rotates
	// forward instead of the whole family being revoked.
	rec = doRefreshUA(t, app, r1, ua)
	if rec.Code != http.StatusOK {
		t.Fatalf("same-UA replay of rotated token: expected 200 got %d body %s", rec.Code, rec.Body.String())
	}
	current := refreshCookieFrom(rec)
	if current == "" || current == r3 {
		t.Fatal("benign replay did not rotate the current token forward")
	}

	// Even after the benign replay the newest token keeps working, so the user
	// is NOT logged out just because two tabs used the same refresh cookie.
	rec = doRefreshUA(t, app, current, ua)
	if rec.Code != http.StatusOK {
		t.Fatalf("current token after benign replay: expected 200 got %d body %s", rec.Code, rec.Body.String())
	}
}

// TestRefreshTokenReplayFromDifferentDeviceIsTheft keeps the theft signal:
// replaying an already-rotated token from a DIFFERENT user-agent means a
// stolen credential was reused, the whole session family is revoked, and the
// client gets SESSION_EXPIRED.
func TestRefreshTokenReplayFromDifferentDeviceIsTheft(t *testing.T) {
	app := testutil.NewApp(t, testutil.Database(t))

	const goodUA = "gaggle-browser/1.0"
	const thiefUA = "stolen-device/9.0"

	reg := app.Do(t, testutil.Request{
		Method: http.MethodPost,
		Path:   "/api/v1/auth/register",
		UA:     goodUA,
		Body:   map[string]string{"username": "victim", "email": "victim@example.com", "password": "password123"},
	})
	if reg.Code != http.StatusCreated {
		t.Fatalf("register: status %d body %s", reg.Code, reg.Body.String())
	}
	r1 := refreshCookieFrom(reg)
	if r1 == "" {
		t.Fatal("register response missing refresh_token cookie")
	}

	// Get the session to a rotated state: the current token is r2.
	rec := doRefreshUA(t, app, r1, goodUA)
	if rec.Code != http.StatusOK {
		t.Fatalf("refresh: status %d body %s", rec.Code, rec.Body.String())
	}
	r2 := refreshCookieFrom(rec)
	if r2 == "" || r2 == r1 {
		t.Fatal("refresh did not rotate")
	}

	// A different device reuses the now-rotated r1: theft. 401 + SESSION_EXPIRED.
	rec = doRefreshUA(t, app, r1, thiefUA)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("cross-device replay: expected 401 got %d body %s", rec.Code, rec.Body.String())
	}
	_, errObj := testutil.Decode[map[string]any](t, rec)
	if errObj == nil || (*errObj)["code"] != "SESSION_EXPIRED" {
		t.Fatalf("cross-device replay: expected SESSION_EXPIRED got %v", errObj)
	}

	// The whole family is dead, including the newest token r2.
	for name, tok := range map[string]string{"r1": r1, "r2": r2} {
		rec := doRefreshUA(t, app, tok, goodUA)
		if rec.Code == http.StatusOK {
			t.Fatalf("token %s should be dead after theft-revocation, got 200", name)
		}
	}
}

// TestRefreshConcurrentStaleReplayKeepsSession is the multi-tab regression:
// two tabs both hold the pre-rotation cookie and refresh with the SAME
// (already-rotated) token at the same time. Neither may be logged out — the
// benign replay path serializes via FOR UPDATE and the session survives.
func TestRefreshConcurrentStaleReplayKeepsSession(t *testing.T) {
	app := testutil.NewApp(t, testutil.Database(t))
	const ua = "gaggle-browser/1.0"

	reg := app.Do(t, testutil.Request{
		Method: http.MethodPost,
		Path:   "/api/v1/auth/register",
		UA:     ua,
		Body:   map[string]string{"username": "tabs", "email": "tabs@example.com", "password": "password123"},
	})
	if reg.Code != http.StatusCreated {
		t.Fatalf("register: status %d body %s", reg.Code, reg.Body.String())
	}
	r1 := refreshCookieFrom(reg)
	if r1 == "" {
		t.Fatal("register response missing refresh_token cookie")
	}

	// Move the session forward so r1 is stale (already rotated).
	if rec := doRefreshUA(t, app, r1, ua); rec.Code != http.StatusOK {
		t.Fatalf("initial refresh: status %d body %s", rec.Code, rec.Body.String())
	}

	// Both tabs replay the now-rotated r1 simultaneously.
	statuses := make([]int, 2)
	errs := make([]error, 2)
	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			rec := app.Do(t, testutil.Request{
				Method:  http.MethodPost,
				Path:    "/api/v1/auth/refresh-token",
				Cookies: []*http.Cookie{{Name: "refresh_token", Value: r1}},
				UA:      ua,
			})
			statuses[i] = rec.Code
			if rec.Code != http.StatusOK {
				errs[i] = fmt.Errorf("replay %d: status %d body %s", i, rec.Code, rec.Body.String())
			}
		}(i)
	}
	wg.Wait()
	for _, err := range errs {
		if err != nil {
			t.Fatalf("%v", err)
		}
	}

	// The session is still alive for a subsequent replay too.
	if rec := doRefreshUA(t, app, r1, ua); rec.Code != http.StatusOK {
		t.Fatalf("post-concurrency replay: expected 200 got %d body %s", rec.Code, rec.Body.String())
	}
}

// TestRefreshCookieSecureFollowsScheme checks the refresh-token cookie's Secure
// attribute tracks the request's actual scheme (via X-Forwarded-Proto from the
// nginx proxy) instead of blindly following the configured COOKIE_SECURE env.
// This is what lets the production box keep plain-HTTP sessions alive while
// HTTPS clients still get a Secure cookie.
func TestRefreshCookieSecureFollowsScheme(t *testing.T) {
	db := testutil.Database(t)

	seq := 0
	cookieByProto := func(t *testing.T, app *testutil.App, proto string) bool {
		t.Helper()
		seq++
		username := "secure_" + itoa(seq)
		rec := app.Do(t, testutil.Request{
			Method: http.MethodPost,
			Path:   "/api/v1/auth/register",
			Headers: map[string]string{
				"X-Forwarded-Proto": proto,
			},
			Body: map[string]string{"username": username, "email": username + "@example.com", "password": "password123"},
		})
		if rec.Code != http.StatusCreated {
			t.Fatalf("register: status %d body %s", rec.Code, rec.Body.String())
		}
		for _, c := range rec.Result().Cookies() {
			if c.Name == "refresh_token" {
				return c.Secure
			}
		}
		t.Fatal("register response missing refresh_token cookie")
		return false
	}

	// Production box (COOKIE_SECURE=true) behind plain HTTP: the browser must
	// be able to persist the cookie, so Secure must be FALSE over http.
	prod := testutil.NewAppWithCookieSecure(t, db, true)
	if cookieByProto(t, prod, "http") {
		t.Error("COOKIE_SECURE=true + X-Forwarded-Proto http: cookie must NOT be Secure (plain-HTTP sessions would never persist)")
	}
	// ... and TRUE behind https.
	if !cookieByProto(t, prod, "https") {
		t.Error("COOKIE_SECURE=true + X-Forwarded-Proto https: cookie must be Secure")
	}

	// Without the proxy header the configured default is the fallback.
	dev := testutil.NewApp(t, db)
	if cookieByProto(t, dev, "") {
		t.Error("cookieSecure=false + no scheme header: cookie must not be Secure")
	}
	if !cookieByProto(t, prod, "") {
		t.Error("cookieSecure=true + no scheme header: cookie must default to Secure")
	}
}

// TestRefreshTokenRotationChain verifies normal rotation keeps working across
// many refreshes and that stream-style auth (which re-validates using the
// refresh cookie) survives rotation because it checks the session family, not
// the exact token.
func TestRefreshTokenRotationChain(t *testing.T) {
	app := testutil.NewApp(t, testutil.Database(t))
	reg := app.Do(t, testutil.Request{
		Method: http.MethodPost,
		Path:   "/api/v1/auth/register",
		Body:   map[string]string{"username": "chainer", "email": "chainer@example.com", "password": "password123"},
	})
	r := refreshCookieFrom(reg)
	if r == "" {
		t.Fatal("register response missing refresh_token cookie")
	}
	for i := 0; i < 3; i++ {
		rec := doRefresh(t, app, r)
		if rec.Code != http.StatusOK {
			t.Fatalf("refresh %d: status %d body %s", i, rec.Code, rec.Body.String())
		}
		next := refreshCookieFrom(rec)
		if next == "" || next == r {
			t.Fatalf("refresh %d did not rotate", i)
		}
		r = next
	}

	// An older-but-rotated token of a live session still authenticates the
	// realtime stream (family check), so streams don't drop on refresh.
	if _, err := app.Service.Auth.GetUserIDFromRefreshToken(context.Background(), refreshCookieFrom(reg)); err != nil {
		t.Fatalf("stream auth should survive rotation, got %v", err)
	}
}

// TestRefreshTokenLogoutRevokesFamily checks that logout revokes the whole
// session family and that logging out with a stale/already-rotated or garbage
// cookie stays idempotent (200).
func TestRefreshTokenLogoutRevokesFamily(t *testing.T) {
	app := testutil.NewApp(t, testutil.Database(t))
	reg := app.Do(t, testutil.Request{
		Method: http.MethodPost,
		Path:   "/api/v1/auth/register",
		Body:   map[string]string{"username": "logger", "email": "logger@example.com", "password": "password123"},
	})
	r1 := refreshCookieFrom(reg)
	if r1 == "" {
		t.Fatal("register response missing refresh_token cookie")
	}

	// Move the session forward one rotation.
	rec := doRefresh(t, app, r1)
	if rec.Code != http.StatusOK {
		t.Fatalf("refresh: status %d body %s", rec.Code, rec.Body.String())
	}
	r2 := refreshCookieFrom(rec)

	// Logout with the current token (r2) revokes the family: r2 is dead.
	rec = app.Do(t, testutil.Request{
		Method:  http.MethodPost,
		Path:    "/api/v1/auth/logout",
		Cookies: []*http.Cookie{{Name: "refresh_token", Value: r2}},
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("logout: status %d body %s", rec.Code, rec.Body.String())
	}
	if r1 != r2 {
		rec = doRefresh(t, app, r1)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("rotated r1 after logout: expected 401 got %d", rec.Code)
		}
	}
	if rec = doRefresh(t, app, r2); rec.Code != http.StatusUnauthorized {
		t.Fatalf("r2 after logout: expected 401 got %d", rec.Code)
	}

	// Logout with a garbage cookie stays idempotent.
	rec = app.Do(t, testutil.Request{
		Method:  http.MethodPost,
		Path:    "/api/v1/auth/logout",
		Cookies: []*http.Cookie{{Name: "refresh_token", Value: "not-a-real-token"}},
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("logout with garbage cookie: expected 200 got %d body %s", rec.Code, rec.Body.String())
	}
}

// TestRefreshTokenExpiredRejected checks that an expired refresh token yields a
// distinct SESSION_EXPIRED code so the frontend can tell "session over" from a
// generic auth error.
func TestRefreshTokenExpiredRejected(t *testing.T) {
	app := testutil.NewApp(t, testutil.Database(t))
	app.Do(t, testutil.Request{
		Method: http.MethodPost,
		Path:   "/api/v1/auth/register",
		Body:   map[string]string{"username": "expirer", "email": "expirer@example.com", "password": "password123"},
	})

	// Craft a well-signed refresh JWT that is already past its exp claim.
	now := time.Now()
	claims := jwt.MapClaims{
		"sub": 1,
		"exp": now.Add(-1 * time.Hour).Unix(),
		"iat": now.Add(-48 * time.Hour).Unix(),
		"nbf": now.Add(-48 * time.Hour).Unix(),
		"iss": "gaggle-test",
		"aud": "gaggle-test",
		"typ": "refresh",
	}
	expired, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte("test-secret"))
	if err != nil {
		t.Fatalf("sign expired token: %v", err)
	}

	rec := doRefresh(t, app, expired)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expired refresh token: expected 401 got %d body %s", rec.Code, rec.Body.String())
	}
	data, errObj := testutil.Decode[map[string]any](t, rec)
	_ = data
	if errObj == nil || (*errObj)["code"] != "SESSION_EXPIRED" {
		t.Fatalf("expired refresh token: expected SESSION_EXPIRED got %v", errObj)
	}
}

func TestAuthFlow(t *testing.T) {
	app := testutil.NewApp(t, testutil.Database(t))

	// Register
	rec := app.Do(t, testutil.Request{
		Method: http.MethodPost,
		Path:   "/api/v1/auth/register",
		Body:   map[string]string{"username": "tester", "email": "tester@example.com", "password": "password123"},
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("register: status %d body %s", rec.Code, rec.Body.String())
	}
	regData, regErr := testutil.Decode[map[string]any](t, rec)
	if regErr != nil {
		t.Fatalf("register should not error: %v", regErr)
	}
	if regData["access_token"] == nil {
		t.Fatal("register response missing access_token")
	}

	// Duplicate email -> 409 with an informative EMAIL_EXISTS code/message
	rec = app.Do(t, testutil.Request{
		Method: http.MethodPost,
		Path:   "/api/v1/auth/register",
		Body:   map[string]string{"username": "tester2", "email": "tester@example.com", "password": "password123"},
	})
	if rec.Code != http.StatusConflict {
		t.Fatalf("duplicate email: expected 409 got %d", rec.Code)
	}
	_, errObj := testutil.Decode[map[string]any](t, rec)
	if errObj == nil || (*errObj)["code"] != "EMAIL_EXISTS" {
		t.Fatalf("duplicate email: expected EMAIL_EXISTS got %v", errObj)
	}
	if msg, _ := (*errObj)["message"].(string); msg == "" {
		t.Fatalf("duplicate email: expected informative message got %v", errObj)
	}

	// Duplicate username -> 409 with an informative USERNAME_EXISTS code/message
	rec = app.Do(t, testutil.Request{
		Method: http.MethodPost,
		Path:   "/api/v1/auth/register",
		Body:   map[string]string{"username": "tester", "email": "tester2@example.com", "password": "password123"},
	})
	if rec.Code != http.StatusConflict {
		t.Fatalf("duplicate username: expected 409 got %d body %s", rec.Code, rec.Body.String())
	}
	_, errObj = testutil.Decode[map[string]any](t, rec)
	if errObj == nil || (*errObj)["code"] != "USERNAME_EXISTS" {
		t.Fatalf("duplicate username: expected USERNAME_EXISTS got %v", errObj)
	}
	if msg, _ := (*errObj)["message"].(string); msg == "" {
		t.Fatalf("duplicate username: expected informative message got %v", errObj)
	}

	// Login
	loginData, _ := testutil.Decode[map[string]any](t, app.Do(t, testutil.Request{
		Method: http.MethodPost,
		Path:   "/api/v1/auth/login",
		Body:   map[string]string{"identifier": "tester@example.com", "password": "password123"},
	}))
	if loginData["access_token"] == nil {
		t.Fatal("login missing access_token")
	}

	// Wrong password -> 401
	rec = app.Do(t, testutil.Request{
		Method: http.MethodPost,
		Path:   "/api/v1/auth/login",
		Body:   map[string]string{"identifier": "tester@example.com", "password": "wrongpass"},
	})
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("bad login: expected 401 got %d", rec.Code)
	}

	// Unauthenticated request -> 401
	rec = app.Do(t, testutil.Request{Method: http.MethodGet, Path: "/api/v1/users/me"})
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("no token: expected 401 got %d", rec.Code)
	}
}

// TestThreeCharacterUsernameSignIn guards the registration/sign-in validation
// agreement: registration allows a 3-char username (min=3), so sign-in must
// accept the same identifier instead of rejecting it with a different floor.
func TestThreeCharacterUsernameSignIn(t *testing.T) {
	app := testutil.NewApp(t, testutil.Database(t))

	rec := app.Do(t, testutil.Request{
		Method: http.MethodPost,
		Path:   "/api/v1/auth/register",
		Body:   map[string]string{"username": "boo", "email": "boo@example.com", "password": "password123"},
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("register 3-char username: expected 201 got %d body %s", rec.Code, rec.Body.String())
	}

	rec = app.Do(t, testutil.Request{
		Method: http.MethodPost,
		Path:   "/api/v1/auth/login",
		Body:   map[string]string{"identifier": "boo", "password": "password123"},
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("sign in with 3-char username: expected 200 got %d body %s", rec.Code, rec.Body.String())
	}
	data, errObj := testutil.Decode[map[string]any](t, rec)
	if errObj != nil {
		t.Fatalf("sign in with 3-char username: unexpected error %v", errObj)
	}
	if data["access_token"] == nil {
		t.Fatal("sign in with 3-char username: missing access_token")
	}
}

func TestProfileLifecycle(t *testing.T) {
	app := testutil.NewApp(t, testutil.Database(t))
	token := app.RegisterUser(t, "alice", "alice@example.com")

	// GET /users/me returns flat profile shape
	me, _ := testutil.Decode[map[string]any](t, app.Do(t, testutil.Request{Method: http.MethodGet, Path: "/api/v1/users/me", Token: token}))
	if me["username"] != "alice" {
		t.Fatalf("me.username = %v, want alice", me["username"])
	}
	if _, ok := me["display_name"]; !ok {
		t.Fatal("me missing display_name (flat profile shape expected)")
	}
	if me["display_name"] != "alice" {
		t.Fatalf("me.display_name = %v, want alice (fresh accounts default display_name to their username)", me["display_name"])
	}
	if me["followers_count"] == nil {
		t.Fatal("me missing followers_count")
	}

	// PATCH /users/me updates and bumps updated_at
	patchData, _ := testutil.Decode[map[string]any](t, app.Do(t, testutil.Request{
		Method: http.MethodPatch,
		Path:   "/api/v1/users/me",
		Token:  token,
		Body:   map[string]string{"display_name": "Alice Cooper", "bio": "hello", "birth_date": "1990-01-01", "location": "Boston", "website": "https://a.dev"},
	}))
	if patchData["display_name"] != "Alice Cooper" {
		t.Fatalf("patched display_name = %v", patchData["display_name"])
	}

	// GET by username
	prof, _ := testutil.Decode[map[string]any](t, app.Do(t, testutil.Request{Method: http.MethodGet, Path: "/api/v1/users/alice", Token: token}))
	if prof["display_name"] != "Alice Cooper" {
		t.Fatalf("profile display_name = %v", prof["display_name"])
	}
}

func TestSettings(t *testing.T) {
	app := testutil.NewApp(t, testutil.Database(t))
	token := app.RegisterUser(t, "sally", "sally@example.com")

	settings, _ := testutil.Decode[map[string]any](t, app.Do(t, testutil.Request{Method: http.MethodGet, Path: "/api/v1/users/settings", Token: token}))
	if settings["language"] != "en" {
		t.Fatalf("default language = %v, want en", settings["language"])
	}

	// Partial patch merges
	updated, _ := testutil.Decode[map[string]any](t, app.Do(t, testutil.Request{
		Method: http.MethodPatch,
		Path:   "/api/v1/users/settings",
		Token:  token,
		Body:   map[string]string{"language": "es"},
	}))
	if updated["language"] != "es" {
		t.Fatalf("patched language = %v, want es", updated["language"])
	}
}

func TestRegisterSeedsLanguageSetting(t *testing.T) {
	app := testutil.NewApp(t, testutil.Database(t))

	// Registering with a browser language should persist it as the initial setting.
	rec := app.Do(t, testutil.Request{
		Method: http.MethodPost,
		Path:   "/api/v1/auth/register",
		Body: map[string]string{
			"username": "lucia",
			"email":    "lucia@example.com",
			"password": "password123",
			"language": "es",
		},
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("register with language: status %d body %s", rec.Code, rec.Body.String())
	}
	data, _ := testutil.Decode[map[string]any](t, rec)
	token := data["access_token"].(string)

	settings, _ := testutil.Decode[map[string]any](t, app.Do(t, testutil.Request{Method: http.MethodGet, Path: "/api/v1/users/settings", Token: token}))
	if settings["language"] != "es" {
		t.Fatalf("seeded language = %v, want es", settings["language"])
	}

	// Notifications defaults must still be present (JSONB row is fully seeded).
	notifications, _ := settings["notifications"].(map[string]any)
	if notifications["email"] != true {
		t.Fatalf("notifications.email = %v, want true", notifications["email"])
	}
}

func TestPostsAndEngagement(t *testing.T) {
	app := testutil.NewApp(t, testutil.Database(t))
	tokenA := app.RegisterUser(t, "user_a", "a@example.com")
	tokenB := app.RegisterUser(t, "user_b", "b@example.com")

	// A creates a post
	post, _ := testutil.Decode[map[string]any](t, app.Do(t, testutil.Request{
		Method: http.MethodPost,
		Path:   "/api/v1/posts/",
		Token:  tokenA,
		Body:   map[string]string{"content": "hello world"},
	}))
	postID := int(post["id"].(float64))
	postPath := "/api/v1/posts/" + itoa(postID)
	if post["author"].(map[string]any)["username"] != "user_a" {
		t.Fatal("post author mismatch")
	}
	eng := post["engagement"].(map[string]any)
	if eng["like_count"].(float64) != 0 {
		t.Fatal("new post should have 0 likes")
	}

	// B likes it; engagement flags reflect viewer state
	rec := app.Do(t, testutil.Request{Method: http.MethodPost, Path: postPath + "/like", Token: tokenB})
	if rec.Code != http.StatusOK {
		t.Fatalf("like failed: %d %s", rec.Code, rec.Body.String())
	}

	likedByB, _ := testutil.Decode[map[string]any](t, app.Do(t, testutil.Request{Method: http.MethodGet, Path: postPath, Token: tokenB}))
	engB := likedByB["post"].(map[string]any)["engagement"].(map[string]any)
	if engB["is_liked"] != true {
		t.Fatal("viewer B should see is_liked=true")
	}
	if engB["like_count"].(float64) != 1 {
		t.Fatalf("like_count = %v, want 1", engB["like_count"])
	}

	// A (non-liker) sees is_liked=false but count 1
	viewedByA, _ := testutil.Decode[map[string]any](t, app.Do(t, testutil.Request{Method: http.MethodGet, Path: postPath, Token: tokenA}))
	engA := viewedByA["post"].(map[string]any)["engagement"].(map[string]any)
	if engA["is_liked"] != false {
		t.Fatal("viewer A should see is_liked=false")
	}
	if engA["like_count"].(float64) != 1 {
		t.Fatalf("like_count for A = %v, want 1", engA["like_count"])
	}

	// Likers feed
	likers, _ := testutil.Decode[map[string]any](t, app.Do(t, testutil.Request{Method: http.MethodGet, Path: postPath + "/likers", Token: tokenA}))
	if len(likers["items"].([]any)) != 1 {
		t.Fatalf("likers items = %v", likers["items"])
	}

	// B reposts
	rec = app.Do(t, testutil.Request{Method: http.MethodPost, Path: postPath + "/repost", Token: tokenB})
	if rec.Code != http.StatusOK {
		t.Fatalf("repost failed: %d %s", rec.Code, rec.Body.String())
	}
	reposters, _ := testutil.Decode[map[string]any](t, app.Do(t, testutil.Request{Method: http.MethodGet, Path: postPath + "/reposters", Token: tokenA}))
	if len(reposters["items"].([]any)) != 1 {
		t.Fatalf("reposters items = %v", reposters["items"])
	}

	// B quotes the post; quotes feed returns it and quotes_count increments
	quote, _ := testutil.Decode[map[string]any](t, app.Do(t, testutil.Request{
		Method: http.MethodPost,
		Path:   postPath + "/quote",
		Token:  tokenB,
		Body:   map[string]string{"content": "nice post"},
	}))
	_ = quote

	quotes, _ := testutil.Decode[map[string]any](t, app.Do(t, testutil.Request{Method: http.MethodGet, Path: postPath + "/quotes", Token: tokenA}))
	if len(quotes["items"].([]any)) != 1 {
		t.Fatalf("quotes items = %v", quotes["items"])
	}

	quoteView, _ := testutil.Decode[map[string]any](t, app.Do(t, testutil.Request{Method: http.MethodGet, Path: postPath, Token: tokenA}))
	quoteEng := quoteView["post"].(map[string]any)["engagement"].(map[string]any)
	if quoteEng["quote_count"].(float64) != 1 {
		t.Fatalf("quote_count = %v, want 1", quoteEng["quote_count"])
	}

	// Delete the original post
	rec = app.Do(t, testutil.Request{Method: http.MethodDelete, Path: postPath, Token: tokenA})
	if rec.Code != http.StatusOK {
		t.Fatalf("delete failed: %d %s", rec.Code, rec.Body.String())
	}
	rec = app.Do(t, testutil.Request{Method: http.MethodGet, Path: postPath, Token: tokenA})
	if rec.Code != http.StatusNotFound {
		t.Fatalf("deleted post should 404, got %d", rec.Code)
	}
}

func TestNotificationsLifecycle(t *testing.T) {
	app := testutil.NewApp(t, testutil.Database(t))
	tokenA := app.RegisterUser(t, "notify_a", "notify-a@example.com")
	tokenB := app.RegisterUser(t, "notify_b", "notify-b@example.com")

	post, _ := testutil.Decode[map[string]any](t, app.Do(t, testutil.Request{
		Method: http.MethodPost,
		Path:   "/api/v1/posts/",
		Token:  tokenA,
		Body:   map[string]string{"content": "notify me"},
	}))
	postID := int(post["id"].(float64))

	if rec := app.Do(t, testutil.Request{
		Method: http.MethodPost,
		Path:   "/api/v1/posts/" + itoa(postID) + "/like",
		Token:  tokenB,
	}); rec.Code != http.StatusOK {
		t.Fatalf("like failed: %d %s", rec.Code, rec.Body.String())
	}
	app.Do(t, testutil.Request{Method: http.MethodPost, Path: "/api/v1/posts/" + itoa(postID) + "/like", Token: tokenB})

	notifications, _ := testutil.Decode[map[string]any](t, app.Do(t, testutil.Request{
		Method: http.MethodGet,
		Path:   "/api/v1/notifications",
		Token:  tokenA,
	}))
	items := notifications["items"].([]any)
	if len(items) != 1 {
		t.Fatalf("notifications items = %d, want 1", len(items))
	}
	notification := items[0].(map[string]any)
	if notification["type"] != "like" {
		t.Fatalf("notification type = %v, want like", notification["type"])
	}
	if notification["read_at"] != nil {
		t.Fatal("new notification should be unread")
	}

	unread, _ := testutil.Decode[map[string]any](t, app.Do(t, testutil.Request{
		Method: http.MethodGet,
		Path:   "/api/v1/notifications/unread-count",
		Token:  tokenA,
	}))
	if unread["count"].(float64) != 1 {
		t.Fatalf("unread count = %v, want 1", unread["count"])
	}

	notificationID := int(notification["id"].(float64))
	if rec := app.Do(t, testutil.Request{
		Method: http.MethodPost,
		Path:   "/api/v1/notifications/" + itoa(notificationID) + "/read",
		Token:  tokenA,
	}); rec.Code != http.StatusNoContent {
		t.Fatalf("mark notification read: %d %s", rec.Code, rec.Body.String())
	}

	unread, _ = testutil.Decode[map[string]any](t, app.Do(t, testutil.Request{
		Method: http.MethodGet,
		Path:   "/api/v1/notifications/unread-count",
		Token:  tokenA,
	}))
	if unread["count"].(float64) != 0 {
		t.Fatalf("unread count after read = %v, want 0", unread["count"])
	}

	mentionedPost, _ := testutil.Decode[map[string]any](t, app.Do(t, testutil.Request{
		Method: http.MethodPost,
		Path:   "/api/v1/posts/",
		Token:  tokenA,
		Body:   map[string]string{"content": "hello @notify_b"},
	}))
	_ = mentionedPost
	bNotifications, _ := testutil.Decode[map[string]any](t, app.Do(t, testutil.Request{
		Method: http.MethodGet,
		Path:   "/api/v1/notifications",
		Token:  tokenB,
	}))
	if len(bNotifications["items"].([]any)) != 1 {
		t.Fatalf("mention notifications = %d, want 1", len(bNotifications["items"].([]any)))
	}

	if rec := app.Do(t, testutil.Request{
		Method: http.MethodPost,
		Path:   "/api/v1/users/notify_a/block",
		Token:  tokenB,
	}); rec.Code != http.StatusOK {
		t.Fatalf("block failed: %d %s", rec.Code, rec.Body.String())
	}
	blockedPost, _ := testutil.Decode[map[string]any](t, app.Do(t, testutil.Request{
		Method: http.MethodPost,
		Path:   "/api/v1/posts/",
		Token:  tokenB,
		Body:   map[string]string{"content": "blocked test"},
	}))
	blockedPostID := int(blockedPost["id"].(float64))
	app.Do(t, testutil.Request{Method: http.MethodPost, Path: "/api/v1/posts/" + itoa(blockedPostID) + "/like", Token: tokenA})
	bNotifications, _ = testutil.Decode[map[string]any](t, app.Do(t, testutil.Request{
		Method: http.MethodGet,
		Path:   "/api/v1/notifications",
		Token:  tokenB,
	}))
	if len(bNotifications["items"].([]any)) != 1 {
		t.Fatalf("blocked actor notification count = %d, want 1", len(bNotifications["items"].([]any)))
	}
}

func TestSearchHashtagsAndTrends(t *testing.T) {
	app := testutil.NewApp(t, testutil.Database(t))
	token := app.RegisterUser(t, "searcher", "searcher@example.com")
	app.RegisterUser(t, "searchfriend", "searchfriend@example.com")

	post, _ := testutil.Decode[map[string]any](t, app.Do(t, testutil.Request{
		Method: http.MethodPost,
		Path:   "/api/v1/posts/",
		Token:  token,
		Body:   map[string]string{"content": "Building a searchable river boat #Docker #GoLang"},
	}))
	postID := int(post["id"].(float64))

	searchRec := app.Do(t, testutil.Request{
		Method: http.MethodGet,
		Path:   "/api/v1/search?q=searchable&type=posts",
		Token:  token,
	})
	if searchRec.Code != http.StatusOK {
		t.Fatalf("post search status = %d body = %s", searchRec.Code, searchRec.Body.String())
	}
	search, _ := testutil.Decode[map[string]any](t, searchRec)
	if len(search["items"].([]any)) != 1 {
		t.Fatalf("post search items = %d, want 1", len(search["items"].([]any)))
	}

	users, _ := testutil.Decode[map[string]any](t, app.Do(t, testutil.Request{
		Method: http.MethodGet,
		Path:   "/api/v1/search?q=searchfriend&type=users",
		Token:  token,
	}))
	if len(users["items"].([]any)) != 1 {
		t.Fatalf("user search items = %d, want 1", len(users["items"].([]any)))
	}

	hashtag, _ := testutil.Decode[map[string]any](t, app.Do(t, testutil.Request{
		Method: http.MethodGet,
		Path:   "/api/v1/hashtags/docker/posts",
		Token:  token,
	}))
	if len(hashtag["items"].([]any)) != 1 || int(hashtag["items"].([]any)[0].(map[string]any)["id"].(float64)) != postID {
		t.Fatalf("hashtag result did not contain post %d: %v", postID, hashtag["items"])
	}

	trends, _ := testutil.Decode[[]map[string]any](t, app.Do(t, testutil.Request{
		Method: http.MethodGet,
		Path:   "/api/v1/trends",
		Token:  token,
	}))
	if len(trends) == 0 || trends[0]["name"] == nil {
		t.Fatalf("trends = %v, expected hashtag data", trends)
	}
}

func TestMentionsFeed(t *testing.T) {
	app := testutil.NewApp(t, testutil.Database(t))
	tokenA := app.RegisterUser(t, "mention_a", "mention-a@example.com")
	tokenB := app.RegisterUser(t, "mention_b", "mention-b@example.com")

	// Tag mention_b case-insensitively plus a bogus username that must be ignored.
	post, _ := testutil.Decode[map[string]any](t, app.Do(t, testutil.Request{
		Method: http.MethodPost,
		Path:   "/api/v1/posts/",
		Token:  tokenA,
		Body:   map[string]string{"content": "Hello @Mention_B and @ghost"},
	}))
	postID := int(post["id"].(float64))

	retrieved := app.Do(t, testutil.Request{Method: http.MethodGet, Path: "/api/v1/mentions", Token: tokenB})
	if retrieved.Code != http.StatusOK {
		t.Fatalf("mentions status = %d body = %s", retrieved.Code, retrieved.Body.String())
	}
	feed, _ := testutil.Decode[map[string]any](t, retrieved)
	items := feed["items"].([]any)
	if len(items) != 1 || int(items[0].(map[string]any)["id"].(float64)) != postID {
		t.Fatalf("mentions feed did not contain post %d: %v", postID, items)
	}

	empty, _ := testutil.Decode[map[string]any](t, app.Do(t, testutil.Request{Method: http.MethodGet, Path: "/api/v1/mentions", Token: tokenA}))
	if len(empty["items"].([]any)) != 0 {
		t.Fatalf("mention_a's mentions feed = %v, want empty", empty["items"])
	}

	// Mentions inside replies are not surfaced (parity with hashtag feeds, which are top-level only).
	app.Do(t, testutil.Request{
		Method: http.MethodPost,
		Path:   "/api/v1/posts/",
		Token:  tokenA,
		Body: map[string]any{
			"content":   "@mention_b in a reply",
			"parent_id": postID,
		},
	})
	afterReply, _ := testutil.Decode[map[string]any](t, app.Do(t, testutil.Request{Method: http.MethodGet, Path: "/api/v1/mentions", Token: tokenB}))
	if len(afterReply["items"].([]any)) != 1 {
		t.Fatalf("mentions feed after reply = %v, want just the top-level post", afterReply["items"])
	}

	// Removing the mention from an edited post drops it from the feed.
	app.Do(t, testutil.Request{
		Method: http.MethodPatch,
		Path:   "/api/v1/posts/" + itoa(postID) + "/",
		Token:  tokenA,
		Body:   map[string]string{"content": "Hello everyone"},
	})
	afterEdit, _ := testutil.Decode[map[string]any](t, app.Do(t, testutil.Request{Method: http.MethodGet, Path: "/api/v1/mentions", Token: tokenB}))
	if len(afterEdit["items"].([]any)) != 0 {
		t.Fatalf("mentions feed after edit = %v, want empty", afterEdit["items"])
	}
}

func TestSearchSubstringMatch(t *testing.T) {
	app := testutil.NewApp(t, testutil.Database(t))
	token := app.RegisterUser(t, "subsearcher", "subsearcher@example.com")

	post, _ := testutil.Decode[map[string]any](t, app.Do(t, testutil.Request{
		Method: http.MethodPost,
		Path:   "/api/v1/posts/",
		Token:  token,
		Body:   map[string]string{"content": "hey everyone"},
	}))
	postID := int(post["id"].(float64))

	search := func(path string) []any {
		rec, _ := testutil.Decode[map[string]any](t, app.Do(t, testutil.Request{
			Method: http.MethodGet,
			Path:   path,
			Token:  token,
		}))
		items, _ := rec["items"].([]any)
		return items
	}

	// A single letter must match posts that merely contain it (substring, not
	// whole-word full-text matching).
	if items := search("/api/v1/search?q=e&type=posts"); len(items) != 1 || int(items[0].(map[string]any)["id"].(float64)) != postID {
		t.Fatalf("single-letter search items = %v, want post %d", items, postID)
	}

	// Partial words should match too.
	if items := search("/api/v1/search?q=hey%20every&type=posts"); len(items) != 1 || int(items[0].(map[string]any)["id"].(float64)) != postID {
		t.Fatalf("partial word search items = %v, want post %d", items, postID)
	}
}

func TestSearchFilters(t *testing.T) {
	app := testutil.NewApp(t, testutil.Database(t))
	tokenA := app.RegisterUser(t, "filteruser", "filteruser@example.com")
	tokenB := app.RegisterUser(t, "filterother", "filterother@example.com")
	tokenViewer := app.RegisterUser(t, "filterviewer", "filterviewer@example.com")

	createPost := func(token string, body map[string]any) int {
		rec, _ := testutil.Decode[map[string]any](t, app.Do(t, testutil.Request{
			Method: http.MethodPost,
			Path:   "/api/v1/posts/",
			Token:  token,
			Body:   body,
		}))
		return int(rec["id"].(float64))
	}

	postA := createPost(tokenA, map[string]any{"content": "alpha kayak riverboat"})
	postB := createPost(tokenA, map[string]any{"content": "beta kayak riverboat #Golang"})
	postC := createPost(tokenB, map[string]any{"content": "gamma kayak riverboat"})
	postD := createPost(tokenB, map[string]any{"content": "delta kayak riverboat", "parent_id": postA})

	if rec := app.Do(t, testutil.Request{Method: http.MethodPost, Path: "/api/v1/posts/" + itoa(postB) + "/like", Token: tokenViewer}); rec.Code != http.StatusOK {
		t.Fatalf("like postB: status %d body %s", rec.Code, rec.Body.String())
	}

	// Attach media to postB directly (the has_media filter checks post_media rows).
	mediaUUID := "11111111-1111-1111-1111-111111111111"
	if _, err := app.DB.Exec(`INSERT INTO media (media_uuid, mime_type, filename) VALUES ($1, 'image/png', 'b.png')`, mediaUUID); err != nil {
		t.Fatalf("insert media: %v", err)
	}
	if _, err := app.DB.Exec(`INSERT INTO post_media (post_id, media_uuid, position, alt_text) VALUES ($1, $2, 1, '')`, postB, mediaUUID); err != nil {
		t.Fatalf("insert post_media: %v", err)
	}

	search := func(path string) []any {
		rec, _ := testutil.Decode[map[string]any](t, app.Do(t, testutil.Request{
			Method: http.MethodGet,
			Path:   path,
			Token:  tokenViewer,
		}))
		items, _ := rec["items"].([]any)
		return items
	}
	hasID := func(items []any, id int) bool {
		for _, it := range items {
			if int(it.(map[string]any)["id"].(float64)) == id {
				return true
			}
		}
		return false
	}

	if items := search("/api/v1/search?q=kayak&type=posts"); len(items) != 3 || !hasID(items, postA) || !hasID(items, postB) || !hasID(items, postC) || hasID(items, postD) {
		t.Fatalf("base search items = %v, want A,B,C (no replies)", items)
	}
	if items := search("/api/v1/search?q=kayak&type=posts&from=filteruser"); len(items) != 2 || !hasID(items, postA) || !hasID(items, postB) {
		t.Fatalf("from filter items = %v, want A,B", items)
	}
	if items := search("/api/v1/search?q=kayak&type=posts&hashtag=golang"); len(items) != 1 || !hasID(items, postB) {
		t.Fatalf("hashtag filter items = %v, want B", items)
	}
	if items := search("/api/v1/search?q=kayak&type=posts&has_media=true"); len(items) != 1 || !hasID(items, postB) {
		t.Fatalf("has_media filter items = %v, want B", items)
	}
	if items := search("/api/v1/search?q=kayak&type=posts&min_likes=1"); len(items) != 1 || !hasID(items, postB) {
		t.Fatalf("min_likes filter items = %v, want B", items)
	}
	if items := search("/api/v1/search?q=kayak&type=posts&include_replies=true"); len(items) != 4 || !hasID(items, postD) {
		t.Fatalf("include_replies filter items = %v, want A,B,C,D", items)
	}
	if items := search("/api/v1/search?q=kayak&type=posts&from=filterother&min_likes=1"); len(items) != 0 {
		t.Fatalf("combined from+min_likes items = %v, want none", items)
	}

	yesterday := url.QueryEscape(time.Now().UTC().Add(-24 * time.Hour).Format(time.RFC3339))
	tomorrow := url.QueryEscape(time.Now().UTC().Add(24 * time.Hour).Format(time.RFC3339))

	if items := search("/api/v1/search?q=kayak&type=posts&since=" + yesterday); len(items) != 3 {
		t.Fatalf("since filter items = %v, want A,B,C", items)
	}
	if items := search("/api/v1/search?q=kayak&type=posts&until=" + yesterday); len(items) != 0 {
		t.Fatalf("until filter items = %v, want none", items)
	}
	if items := search("/api/v1/search?q=kayak&type=posts&since=" + yesterday + "&until=" + tomorrow); len(items) != 3 {
		t.Fatalf("since+until filter items = %v, want A,B,C", items)
	}

	rec := app.Do(t, testutil.Request{Method: http.MethodGet, Path: "/api/v1/search?q=kayak&type=posts&since=not-a-date", Token: tokenViewer})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("invalid since status = %d, want 400 (body %s)", rec.Code, rec.Body.String())
	}
}

func TestPostPowerFeatures(t *testing.T) {
	app := testutil.NewApp(t, testutil.Database(t))
	tokenA := app.RegisterUser(t, "power_a", "power-a@example.com")
	tokenB := app.RegisterUser(t, "power_b", "power-b@example.com")

	root, _ := testutil.Decode[map[string]any](t, app.Do(t, testutil.Request{
		Method: http.MethodPost,
		Path:   "/api/v1/posts/",
		Token:  tokenA,
		Body:   map[string]string{"content": "original content"},
	}))
	rootID := int(root["id"].(float64))

	edited, _ := testutil.Decode[map[string]any](t, app.Do(t, testutil.Request{
		Method: http.MethodPatch,
		Path:   "/api/v1/posts/" + itoa(rootID),
		Token:  tokenA,
		Body:   map[string]string{"content": "edited content"},
	}))
	if edited["content"] != "edited content" || edited["edited_at"] == nil {
		t.Fatalf("edited post = %v", edited)
	}

	edits, _ := testutil.Decode[map[string]any](t, app.Do(t, testutil.Request{
		Method: http.MethodGet,
		Path:   "/api/v1/posts/" + itoa(rootID) + "/edits",
		Token:  tokenA,
	}))
	if len(edits["items"].([]any)) != 1 {
		t.Fatalf("edit history items = %d, want 1", len(edits["items"].([]any)))
	}

	if rec := app.Do(t, testutil.Request{Method: http.MethodPost, Path: "/api/v1/posts/" + itoa(rootID) + "/pin", Token: tokenA}); rec.Code != http.StatusOK {
		t.Fatalf("pin failed: %d %s", rec.Code, rec.Body.String())
	}
	pinned, _ := testutil.Decode[map[string]any](t, app.Do(t, testutil.Request{Method: http.MethodGet, Path: "/api/v1/users/power_a/pinned", Token: tokenA}))
	if int(pinned["id"].(float64)) != rootID {
		t.Fatalf("pinned post id = %v, want %d", pinned["id"], rootID)
	}

	// one pinned per author: pinning a second post replaces the first
	second, _ := testutil.Decode[map[string]any](t, app.Do(t, testutil.Request{
		Method: http.MethodPost,
		Path:   "/api/v1/posts/",
		Token:  tokenA,
		Body:   map[string]string{"content": "second post"},
	}))
	secondID := int(second["id"].(float64))
	if rec := app.Do(t, testutil.Request{Method: http.MethodPost, Path: "/api/v1/posts/" + itoa(secondID) + "/pin", Token: tokenA}); rec.Code != http.StatusOK {
		t.Fatalf("re-pin failed: %d %s", rec.Code, rec.Body.String())
	}
	pinned, _ = testutil.Decode[map[string]any](t, app.Do(t, testutil.Request{Method: http.MethodGet, Path: "/api/v1/users/power_a/pinned", Token: tokenA}))
	if int(pinned["id"].(float64)) != secondID {
		t.Fatalf("pinned post after re-pin = %v, want %d", pinned["id"], secondID)
	}
	// unpin the second post
	if rec := app.Do(t, testutil.Request{Method: http.MethodDelete, Path: "/api/v1/posts/" + itoa(secondID) + "/pin", Token: tokenA}); rec.Code != http.StatusOK {
		t.Fatalf("unpin failed: %d %s", rec.Code, rec.Body.String())
	}
	if rec := app.Do(t, testutil.Request{Method: http.MethodGet, Path: "/api/v1/users/power_a/pinned", Token: tokenA}); rec.Code != http.StatusNotFound {
		t.Fatalf("pinned after unpin status = %d, want 404", rec.Code)
	}
	// re-pin the root for later delete/pinned assertions
	if rec := app.Do(t, testutil.Request{Method: http.MethodPost, Path: "/api/v1/posts/" + itoa(rootID) + "/pin", Token: tokenA}); rec.Code != http.StatusOK {
		t.Fatalf("re-pin root failed: %d %s", rec.Code, rec.Body.String())
	}

	// non-author edit is forbidden
	if rec := app.Do(t, testutil.Request{
		Method: http.MethodPatch,
		Path:   "/api/v1/posts/" + itoa(rootID),
		Token:  tokenB,
		Body:   map[string]string{"content": "nope"},
	}); rec.Code != http.StatusForbidden {
		t.Fatalf("non-author edit status = %d, want 403", rec.Code)
	}
	// non-author pin is forbidden
	if rec := app.Do(t, testutil.Request{Method: http.MethodPost, Path: "/api/v1/posts/" + itoa(rootID) + "/pin", Token: tokenB}); rec.Code != http.StatusForbidden {
		t.Fatalf("non-author pin status = %d, want 403", rec.Code)
	}

	poll, _ := testutil.Decode[map[string]any](t, app.Do(t, testutil.Request{
		Method: http.MethodPost,
		Path:   "/api/v1/posts/",
		Token:  tokenA,
		Body: map[string]any{
			"content": "pick one",
			"poll":    map[string]any{"question": "Which?", "options": []string{"one", "two"}},
		},
	}))
	pollID := int(poll["id"].(float64))
	if poll["poll"] == nil {
		t.Fatal("created post missing poll")
	}
	postPoll := poll["poll"].(map[string]any)
	if int(postPoll["id"].(float64)) == 0 {
		t.Fatal("created post poll missing id")
	}
	if postPoll["question"] != "pick one" {
		t.Fatalf("poll question = %v, want mirrored post content", postPoll["question"])
	}
	if rec := app.Do(t, testutil.Request{
		Method: http.MethodPost,
		Path:   "/api/v1/posts/" + itoa(pollID) + "/poll/vote",
		Token:  tokenB,
		Body:   map[string]int{"option_id": 1},
	}); rec.Code != http.StatusOK {
		t.Fatalf("poll vote failed: %d %s", rec.Code, rec.Body.String())
	}
	// duplicate vote conflicts
	if rec := app.Do(t, testutil.Request{
		Method: http.MethodPost,
		Path:   "/api/v1/posts/" + itoa(pollID) + "/poll/vote",
		Token:  tokenB,
		Body:   map[string]int{"option_id": 2},
	}); rec.Code != http.StatusConflict {
		t.Fatalf("duplicate poll vote status = %d, want 409", rec.Code)
	}
	// a poll can never be attached to a reply
	if rec := app.Do(t, testutil.Request{
		Method: http.MethodPost,
		Path:   "/api/v1/posts/",
		Token:  tokenB,
		Body: map[string]any{
			"content":   "reply with poll",
			"parent_id": rootID,
			"poll":      map[string]any{"question": "nope?", "options": []string{"a", "b"}},
		},
	}); rec.Code != http.StatusBadRequest {
		t.Fatalf("poll on reply status = %d, want 400", rec.Code)
	}

	// polls are hydrated in feeds: have tokenB follow power_a and read the home feed
	if rec := app.Do(t, testutil.Request{Method: http.MethodPost, Path: "/api/v1/users/power_a/follow", Token: tokenB}); rec.Code != http.StatusOK {
		t.Fatalf("follow failed: %d %s", rec.Code, rec.Body.String())
	}
	feed, _ := testutil.Decode[map[string]any](t, app.Do(t, testutil.Request{Method: http.MethodGet, Path: "/api/v1/posts/feed", Token: tokenB}))
	var feedPollSeen bool
	for _, item := range feed["items"].([]any) {
		if int(item.(map[string]any)["id"].(float64)) == pollID {
			if item.(map[string]any)["poll"] == nil {
				t.Fatalf("home feed poll post missing poll hydration")
			}
			feedPollSeen = true
		}
	}
	if !feedPollSeen {
		t.Fatalf("home feed did not include the poll post")
	}
	// poll also hydrated in the user feed
	userFeed, _ := testutil.Decode[map[string]any](t, app.Do(t, testutil.Request{Method: http.MethodGet, Path: "/api/v1/users/power_a/posts", Token: tokenB}))
	var userPollSeen bool
	for _, item := range userFeed["items"].([]any) {
		if int(item.(map[string]any)["id"].(float64)) == pollID {
			if item.(map[string]any)["poll"] == nil {
				t.Fatalf("user feed poll post missing poll hydration")
			}
			userPollSeen = true
		}
	}
	if !userPollSeen {
		t.Fatalf("user feed did not include the poll post")
	}

	reply, _ := testutil.Decode[map[string]any](t, app.Do(t, testutil.Request{
		Method: http.MethodPost,
		Path:   "/api/v1/posts/",
		Token:  tokenB,
		Body:   map[string]any{"content": "reply", "parent_id": rootID},
	}))
	replyID := int(reply["id"].(float64))
	if rec := app.Do(t, testutil.Request{Method: http.MethodDelete, Path: "/api/v1/posts/" + itoa(rootID), Token: tokenA}); rec.Code != http.StatusOK {
		t.Fatalf("delete root failed: %d %s", rec.Code, rec.Body.String())
	}
	if rec := app.Do(t, testutil.Request{Method: http.MethodGet, Path: "/api/v1/posts/" + itoa(replyID), Token: tokenB}); rec.Code != http.StatusNotFound {
		t.Fatalf("deleted descendant status = %d, want 404", rec.Code)
	}
	// deleted root disappears from pinned
	if rec := app.Do(t, testutil.Request{Method: http.MethodGet, Path: "/api/v1/users/power_a/pinned", Token: tokenA}); rec.Code != http.StatusNotFound {
		t.Fatalf("pinned after delete status = %d, want 404", rec.Code)
	}
	// deleted posts reject edit history and votes
	if rec := app.Do(t, testutil.Request{Method: http.MethodGet, Path: "/api/v1/posts/" + itoa(rootID) + "/edits", Token: tokenA}); rec.Code != http.StatusNotFound {
		t.Fatalf("edit history on deleted post status = %d, want 404", rec.Code)
	}
	if rec := app.Do(t, testutil.Request{
		Method: http.MethodPost,
		Path:   "/api/v1/posts/" + itoa(rootID) + "/poll/vote",
		Token:  tokenB,
		Body:   map[string]int{"option_id": 1},
	}); rec.Code != http.StatusNotFound {
		t.Fatalf("vote on deleted post status = %d, want 404", rec.Code)
	}
	// deleting a non-author post is forbidden
	if rec := app.Do(t, testutil.Request{Method: http.MethodDelete, Path: "/api/v1/posts/" + itoa(pollID), Token: tokenB}); rec.Code != http.StatusForbidden {
		t.Fatalf("non-author delete status = %d, want 403", rec.Code)
	}
}

func TestParentChainAndDescendants(t *testing.T) {
	app := testutil.NewApp(t, testutil.Database(t))
	token := app.RegisterUser(t, "chainuser", "chain@example.com")

	// post 1 -> reply post 2 -> reply post 3
	root, _ := testutil.Decode[map[string]any](t, app.Do(t, testutil.Request{Method: http.MethodPost, Path: "/api/v1/posts/", Token: token, Body: map[string]string{"content": "root"}}))
	rootID := int(root["id"].(float64))
	reply2, _ := testutil.Decode[map[string]any](t, app.Do(t, testutil.Request{Method: http.MethodPost, Path: "/api/v1/posts/", Token: token, Body: map[string]any{"content": "reply 2", "parent_id": rootID}}))
	reply2ID := int(reply2["id"].(float64))
	reply3, _ := testutil.Decode[map[string]any](t, app.Do(t, testutil.Request{Method: http.MethodPost, Path: "/api/v1/posts/", Token: token, Body: map[string]any{"content": "reply 3", "parent_id": reply2ID}}))
	reply3ID := int(reply3["id"].(float64))

	// fetch the middle post (reply2): it has 1 ancestor and 1 descendant
	detail, _ := testutil.Decode[map[string]any](t, app.Do(t, testutil.Request{
		Method: http.MethodGet,
		Path:   "/api/v1/posts/" + itoa(reply2ID) + "?ancestors=true&descendants=true",
		Token:  token,
	}))
	if detail["post"].(map[string]any)["id"].(float64) != float64(reply2ID) {
		t.Fatalf("detail post id = %v", detail["post"])
	}
	ancestors := detail["ancestors"].(map[string]any)["items"].([]any)
	if len(ancestors) != 1 {
		t.Fatalf("ancestors = %d items, want 1", len(ancestors))
	}
	descendants := detail["descendants"].(map[string]any)["items"].([]any)
	if len(descendants) != 1 {
		t.Fatalf("descendants = %d items, want 1", len(descendants))
	}
	_ = root
	_ = reply3ID
}

func TestHomeFeedShowsFollowedUsers(t *testing.T) {
	app := testutil.NewApp(t, testutil.Database(t))
	tokenA := app.RegisterUser(t, "feeda", "feeda@example.com")
	tokenB := app.RegisterUser(t, "feedb", "feedb@example.com")

	// B posts
	app.Do(t, testutil.Request{Method: http.MethodPost, Path: "/api/v1/posts/", Token: tokenB, Body: map[string]string{"content": "from b"}})

	// A posts (their own home feed must include their own post too).
	app.Do(t, testutil.Request{Method: http.MethodPost, Path: "/api/v1/posts/", Token: tokenA, Body: map[string]string{"content": "from a"}})

	// A follows B
	rec := app.Do(t, testutil.Request{Method: http.MethodPost, Path: "/api/v1/users/feedb/follow", Token: tokenA})
	if rec.Code != http.StatusOK {
		t.Fatalf("follow failed: %d %s", rec.Code, rec.Body.String())
	}

	feed, _ := testutil.Decode[map[string]any](t, app.Do(t, testutil.Request{Method: http.MethodGet, Path: "/api/v1/posts/feed", Token: tokenA}))
	items := feed["items"].([]any)
	if len(items) != 2 {
		t.Fatalf("home feed should have 2 posts (own + followed user), got %v", feed["items"])
	}
	// "from a" must be present (author's own post stays in their home feed).
	foundOwn := false
	for _, it := range items {
		if it.(map[string]any)["content"] == "from a" {
			foundOwn = true
		}
	}
	if !foundOwn {
		t.Fatalf("own post missing from home feed: %v", feed["items"])
	}
}

func TestBookmarkCategories(t *testing.T) {
	app := testutil.NewApp(t, testutil.Database(t))
	token := app.RegisterUser(t, "bmuser", "bm@example.com")

	// A "General" category is auto-created on registration.
	cats, _ := testutil.Decode[[]map[string]any](t, app.Do(t, testutil.Request{Method: http.MethodGet, Path: "/api/v1/bookmarks/category", Token: token}))
	if len(cats) != 1 {
		t.Fatalf("expected 1 default category, got %d", len(cats))
	}
	if cats[0]["name"] != "General" {
		t.Fatalf("default category name = %v", cats[0]["name"])
	}

	// Create category (payload uses "name")
	created, _ := testutil.Decode[map[string]any](t, app.Do(t, testutil.Request{
		Method: http.MethodPost,
		Path:   "/api/v1/bookmarks/category",
		Token:  token,
		Body:   map[string]string{"name": "Top picks"},
	}))
	if created["category"].(map[string]any)["name"] != "Top picks" {
		t.Fatalf("created category = %v", created["category"])
	}
}

func TestViewsAreRecorded(t *testing.T) {
	app := testutil.NewApp(t, testutil.Database(t))
	token := app.RegisterUser(t, "viewer", "view@example.com")
	post, _ := testutil.Decode[map[string]any](t, app.Do(t, testutil.Request{Method: http.MethodPost, Path: "/api/v1/posts/", Token: token, Body: map[string]string{"content": "view me"}}))
	postID := int(post["id"].(float64))

	detail, _ := testutil.Decode[map[string]any](t, app.Do(t, testutil.Request{Method: http.MethodGet, Path: "/api/v1/posts/" + itoa(postID), Token: token}))
	eng := detail["post"].(map[string]any)["engagement"].(map[string]any)
	if eng["view_count"].(float64) < 1 {
		t.Fatalf("view_count = %v, want >= 1", eng["view_count"])
	}
}

// TestViewsAreDeduplicatedPerUser: every GET of a post (including the React
// Query refetches that like/bookmark mutations trigger) used to append another
// row to post_views, so interacting with a post bumped its view count. The
// dedup index means one authenticated user only ever counts once per post.
func TestViewsAreDeduplicatedPerUser(t *testing.T) {
	app := testutil.NewApp(t, testutil.Database(t))
	token := app.RegisterUser(t, "viewer2", "view2@example.com")
	otherToken := app.RegisterUser(t, "otherviewer", "otherview@example.com")
	post, _ := testutil.Decode[map[string]any](t, app.Do(t, testutil.Request{Method: http.MethodPost, Path: "/api/v1/posts/", Token: token, Body: map[string]string{"content": "view me dedup"}}))
	postID := int(post["id"].(float64))

	path := "/api/v1/posts/" + itoa(postID)
	detail, _ := testutil.Decode[map[string]any](t, app.Do(t, testutil.Request{Method: http.MethodGet, Path: path, Token: token}))
	first := detail["post"].(map[string]any)["engagement"].(map[string]any)["view_count"].(float64)
	if first < 1 {
		t.Fatalf("view_count = %v, want >= 1", first)
	}

	// A second fetch (what the like/bookmark invalidation refetch does) must
	// NOT inflate the count for the same user.
	detail, _ = testutil.Decode[map[string]any](t, app.Do(t, testutil.Request{Method: http.MethodGet, Path: path, Token: token}))
	second := detail["post"].(map[string]any)["engagement"].(map[string]any)["view_count"].(float64)
	if second != first {
		t.Fatalf("view_count after repeat fetch = %v, want %v (no bump)", second, first)
	}

	// A different user still counts as a new view.
	detail, _ = testutil.Decode[map[string]any](t, app.Do(t, testutil.Request{Method: http.MethodGet, Path: path, Token: otherToken}))
	third := detail["post"].(map[string]any)["engagement"].(map[string]any)["view_count"].(float64)
	if third != first+1 {
		t.Fatalf("view_count after second user = %v, want %v", third, first+1)
	}
}

func TestEnvelopeContract(t *testing.T) {
	app := testutil.NewApp(t, testutil.Database(t))
	token := app.RegisterUser(t, "envuser", "env@example.com")

	// Success responses are wrapped in {data, error:null}
	rec := app.Do(t, testutil.Request{Method: http.MethodGet, Path: "/api/v1/users/me", Token: token})
	var env map[string]json.RawMessage
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("response not valid JSON: %v", err)
	}
	if _, ok := env["data"]; !ok {
		t.Fatalf("success response missing data key: %s", rec.Body.String())
	}
	if errEnv, ok := env["error"]; !ok || string(errEnv) != "null" {
		t.Fatalf("success response error should be null: %s", rec.Body.String())
	}
}

func TestPaginationShape(t *testing.T) {
	app := testutil.NewApp(t, testutil.Database(t))
	token := app.RegisterUser(t, "pager", "pager@example.com")
	for i := 1; i <= 5; i++ {
		app.Do(t, testutil.Request{Method: http.MethodPost, Path: "/api/v1/posts/", Token: token, Body: map[string]string{"content": "post"}})
	}

	feed, _ := testutil.Decode[map[string]any](t, app.Do(t, testutil.Request{Method: http.MethodGet, Path: "/api/v1/users/pager/posts?limit=2", Token: token}))
	if _, ok := feed["items"]; !ok {
		t.Fatalf("feed missing items key (contract says items): %v", feed)
	}
	if len(feed["items"].([]any)) != 2 {
		t.Fatalf("items = %d, want 2", len(feed["items"].([]any)))
	}
	if _, ok := feed["next_cursor"]; !ok {
		t.Fatalf("feed missing next_cursor: %v", feed)
	}
	if feed["has_more"] != true {
		t.Fatalf("feed has_more = %v, want true", feed["has_more"])
	}

	// Second page via cursor
	page2, _ := testutil.Decode[map[string]any](t, app.Do(t, testutil.Request{
		Method: http.MethodGet,
		Path:   "/api/v1/users/pager/posts?limit=2&cursor=" + feed["next_cursor"].(string),
		Token:  token,
	}))
	if len(page2["items"].([]any)) != 2 {
		t.Fatalf("page2 items = %d, want 2", len(page2["items"].([]any)))
	}
}

func TestBadges(t *testing.T) {
	app := testutil.NewApp(t, testutil.Database(t))
	adminToken := app.RegisterUser(t, "badgeadmin", "badgeadmin@example.com")
	userToken := app.RegisterUser(t, "badgeuser", "badgeuser@example.com")

	// Non-admins cannot use the admin API.
	rec := app.Do(t, testutil.Request{Method: http.MethodGet, Path: "/api/v1/admin/badges", Token: userToken})
	if rec.Code != http.StatusForbidden {
		t.Fatalf("non-admin catalog status = %d, want 403", rec.Code)
	}

	// Promote the admin user directly in the DB (register creates non-admins).
	if _, err := app.DB.Exec(`UPDATE users SET is_admin = TRUE WHERE username = 'badgeadmin'`); err != nil {
		t.Fatalf("promote admin: %v", err)
	}

	catalog, _ := testutil.Decode[[]any](t, app.Do(t, testutil.Request{Method: http.MethodGet, Path: "/api/v1/admin/badges", Token: adminToken}))
	if len(catalog) != 4 {
		t.Fatalf("catalog len = %d, want 4 seeded earned badges", len(catalog))
	}

	// Create an assigned badge.
	created, _ := testutil.Decode[map[string]any](t, app.Do(t, testutil.Request{
		Method: http.MethodPost,
		Path:   "/api/v1/admin/badges",
		Token:  adminToken,
		Body: map[string]string{
			"key":         "staff",
			"label":       "Staff",
			"description": "Gaggle staff member",
			"icon":        "ShieldCheck",
		},
	}))
	createdData := created
	badgeID := int(createdData["id"].(float64))

	// Duplicate key -> 409.
	rec = app.Do(t, testutil.Request{
		Method: http.MethodPost,
		Path:   "/api/v1/admin/badges",
		Token:  adminToken,
		Body: map[string]string{
			"key":         "staff",
			"label":       "Staff 2",
			"description": "dup",
			"icon":        "ShieldCheck",
		},
	})
	if rec.Code != http.StatusConflict {
		t.Fatalf("duplicate badge key status = %d, want 409", rec.Code)
	}

	// Grant to the regular user, then confirm it shows on their profile.
	if rec := app.Do(t, testutil.Request{Method: http.MethodPost, Path: "/api/v1/admin/users/badgeuser/badges/" + itoa(badgeID), Token: adminToken}); rec.Code != http.StatusOK {
		t.Fatalf("grant badge: %d %s", rec.Code, rec.Body.String())
	}
	profile, _ := testutil.Decode[map[string]any](t, app.Do(t, testutil.Request{Method: http.MethodGet, Path: "/api/v1/users/badgeuser", Token: userToken}))
	badges := profile["badges"].([]any)
	found := false
	for _, b := range badges {
		if b.(map[string]any)["key"] == "staff" {
			found = true
		}
	}
	if !found {
		t.Fatalf("granted badge missing from profile: %v", badges)
	}

	// Duplicate grant -> 409.
	if rec := app.Do(t, testutil.Request{Method: http.MethodPost, Path: "/api/v1/admin/users/badgeuser/badges/" + itoa(badgeID), Token: adminToken}); rec.Code != http.StatusConflict {
		t.Fatalf("duplicate badge grant status = %d, want 409", rec.Code)
	}

	// Badge still in use cannot be deleted -> 409.
	if rec := app.Do(t, testutil.Request{Method: http.MethodDelete, Path: "/api/v1/admin/badges/" + itoa(badgeID), Token: adminToken}); rec.Code != http.StatusConflict {
		t.Fatalf("delete in-use badge status = %d, want 409", rec.Code)
	}

	// Revoke then delete succeeds.
	if rec := app.Do(t, testutil.Request{Method: http.MethodDelete, Path: "/api/v1/admin/users/badgeuser/badges/" + itoa(badgeID), Token: adminToken}); rec.Code != http.StatusOK {
		t.Fatalf("revoke badge: %d %s", rec.Code, rec.Body.String())
	}
	if rec := app.Do(t, testutil.Request{Method: http.MethodDelete, Path: "/api/v1/admin/badges/" + itoa(badgeID), Token: adminToken}); rec.Code != http.StatusOK {
		t.Fatalf("delete badge: %d %s", rec.Code, rec.Body.String())
	}

	// Revoke again -> 404.
	if rec := app.Do(t, testutil.Request{Method: http.MethodDelete, Path: "/api/v1/admin/users/badgeuser/badges/" + itoa(badgeID), Token: adminToken}); rec.Code != http.StatusNotFound {
		t.Fatalf("revoke removed badge status = %d, want 404", rec.Code)
	}

	// Earned badge: give the user >100 top-level posts and check prolific_poster.
	for i := 0; i < 101; i++ {
		app.Do(t, testutil.Request{Method: http.MethodPost, Path: "/api/v1/posts/", Token: userToken, Body: map[string]string{"content": "badge post"}})
	}
	profile, _ = testutil.Decode[map[string]any](t, app.Do(t, testutil.Request{Method: http.MethodGet, Path: "/api/v1/users/badgeuser", Token: userToken}))
	badges = profile["badges"].([]any)
	found = false
	for _, b := range badges {
		if b.(map[string]any)["key"] == "prolific_poster" {
			found = true
		}
	}
	if !found {
		t.Fatalf("earned prolific_poster missing from profile: %v", badges)
	}
}

func TestLists(t *testing.T) {
	app := testutil.NewApp(t, testutil.Database(t))
	ownerToken := app.RegisterUser(t, "listowner", "listowner@example.com")
	app.RegisterUser(t, "listmember", "listmember@example.com")
	otherToken := app.RegisterUser(t, "listother", "listother@example.com")

	// Create a list.
	created, _ := testutil.Decode[map[string]any](t, app.Do(t, testutil.Request{
		Method: http.MethodPost,
		Path:   "/api/v1/lists",
		Token:  ownerToken,
		Body:   map[string]string{"name": "Go people", "description": "folks who write Go"},
	}))
	listID := int(created["id"].(float64))
	if created["name"] != "Go people" {
		t.Fatalf("created list = %v", created)
	}

	// Duplicate name -> 409.
	if rec := app.Do(t, testutil.Request{Method: http.MethodPost, Path: "/api/v1/lists", Token: ownerToken, Body: map[string]string{"name": "Go people"}}); rec.Code != http.StatusConflict {
		t.Fatalf("duplicate list name status = %d, want 409", rec.Code)
	}

	// Update the list (owner).
	updated, _ := testutil.Decode[map[string]any](t, app.Do(t, testutil.Request{Method: http.MethodPatch, Path: "/api/v1/lists/" + itoa(listID), Token: ownerToken, Body: map[string]string{"name": "Gagglers", "description": "updated description"}}))
	if updated["name"] != "Gagglers" || updated["description"] != "updated description" {
		t.Fatalf("updated list = %v", updated)
	}
	// Non-owner cannot update -> 403.
	if rec := app.Do(t, testutil.Request{Method: http.MethodPatch, Path: "/api/v1/lists/" + itoa(listID), Token: otherToken, Body: map[string]string{"name": "Hijacked"}}); rec.Code != http.StatusForbidden {
		t.Fatalf("non-owner update status = %d, want 403", rec.Code)
	}
	// Invalid payload (empty name) -> 400.
	if rec := app.Do(t, testutil.Request{Method: http.MethodPatch, Path: "/api/v1/lists/" + itoa(listID), Token: ownerToken, Body: map[string]string{"name": ""}}); rec.Code != http.StatusBadRequest {
		t.Fatalf("empty-name update status = %d, want 400", rec.Code)
	}
	// Updating to a duplicate name -> 409.
	if rec := app.Do(t, testutil.Request{Method: http.MethodPost, Path: "/api/v1/lists", Token: ownerToken, Body: map[string]string{"name": "Other list"}}); rec.Code != http.StatusCreated {
		t.Fatalf("create second list: %d %s", rec.Code, rec.Body.String())
	}
	if rec := app.Do(t, testutil.Request{Method: http.MethodPatch, Path: "/api/v1/lists/" + itoa(listID), Token: ownerToken, Body: map[string]string{"name": "Other list"}}); rec.Code != http.StatusConflict {
		t.Fatalf("duplicate-name update status = %d, want 409", rec.Code)
	}

	// Add a member.
	if rec := app.Do(t, testutil.Request{Method: http.MethodPost, Path: "/api/v1/lists/" + itoa(listID) + "/members/listmember", Token: ownerToken}); rec.Code != http.StatusOK {
		t.Fatalf("add member: %d %s", rec.Code, rec.Body.String())
	}
	// Non-owner cannot add members.
	if rec := app.Do(t, testutil.Request{Method: http.MethodPost, Path: "/api/v1/lists/" + itoa(listID) + "/members/listother", Token: otherToken}); rec.Code != http.StatusForbidden {
		t.Fatalf("non-owner add member status = %d, want 403", rec.Code)
	}
	// Adding yourself to your own list -> 400.
	if rec := app.Do(t, testutil.Request{Method: http.MethodPost, Path: "/api/v1/lists/" + itoa(listID) + "/members/listowner", Token: ownerToken}); rec.Code != http.StatusBadRequest {
		t.Fatalf("self-add status = %d, want 400", rec.Code)
	}
	// Duplicate member -> 409.
	if rec := app.Do(t, testutil.Request{Method: http.MethodPost, Path: "/api/v1/lists/" + itoa(listID) + "/members/listmember", Token: ownerToken}); rec.Code != http.StatusConflict {
		t.Fatalf("duplicate member status = %d, want 409", rec.Code)
	}

	// Members are public to any viewer.
	members, _ := testutil.Decode[map[string]any](t, app.Do(t, testutil.Request{Method: http.MethodGet, Path: "/api/v1/lists/" + itoa(listID) + "/members", Token: otherToken}))
	memberNames := []string{}
	for _, m := range members["items"].([]any) {
		memberNames = append(memberNames, m.(map[string]any)["username"].(string))
	}
	if len(memberNames) != 1 || memberNames[0] != "listmember" {
		t.Fatalf("members = %v, want [listmember]", memberNames)
	}

	// The list feed contains the member's top-level post, but not replies.
	if _, err := app.DB.Exec(`INSERT INTO posts (author_id, content) VALUES ((SELECT user_id FROM users WHERE username='listmember'), 'hello from member')`); err != nil {
		t.Fatalf("insert member post: %v", err)
	}
	if _, err := app.DB.Exec(`INSERT INTO posts (author_id, content, parent_id) VALUES ((SELECT user_id FROM users WHERE username='listmember'), 'a reply', (SELECT post_id FROM posts WHERE content='hello from member'))`); err != nil {
		t.Fatalf("insert member reply: %v", err)
	}
	feed, _ := testutil.Decode[map[string]any](t, app.Do(t, testutil.Request{Method: http.MethodGet, Path: "/api/v1/lists/" + itoa(listID) + "/feed", Token: otherToken}))
	if len(feed["items"].([]any)) != 1 {
		t.Fatalf("list feed items = %d, want 1 (top-level only)", len(feed["items"].([]any)))
	}

	// List appears in the user's lists (owner + public profile).
	myLists, _ := testutil.Decode[[]any](t, app.Do(t, testutil.Request{Method: http.MethodGet, Path: "/api/v1/lists", Token: ownerToken}))
	if len(myLists) != 2 {
		t.Fatalf("my lists = %d, want 2", len(myLists))
	}
	profileLists, _ := testutil.Decode[[]any](t, app.Do(t, testutil.Request{Method: http.MethodGet, Path: "/api/v1/users/listowner/lists", Token: otherToken}))
	if len(profileLists) != 2 {
		t.Fatalf("profile lists = %d, want 2", len(profileLists))
	}

	// Non-owner cannot delete; owner can.
	if rec := app.Do(t, testutil.Request{Method: http.MethodDelete, Path: "/api/v1/lists/" + itoa(listID), Token: otherToken}); rec.Code != http.StatusForbidden {
		t.Fatalf("non-owner delete status = %d, want 403", rec.Code)
	}
	if rec := app.Do(t, testutil.Request{Method: http.MethodDelete, Path: "/api/v1/lists/" + itoa(listID), Token: ownerToken}); rec.Code != http.StatusOK {
		t.Fatalf("owner delete: %d %s", rec.Code, rec.Body.String())
	}
	if rec := app.Do(t, testutil.Request{Method: http.MethodGet, Path: "/api/v1/lists/" + itoa(listID), Token: ownerToken}); rec.Code != http.StatusNotFound {
		t.Fatalf("deleted list status = %d, want 404", rec.Code)
	}
}

func TestSuggestedUsers(t *testing.T) {
	app := testutil.NewApp(t, testutil.Database(t))
	viewerToken := app.RegisterUser(t, "sugviewer", "sugviewer@example.com")
	app.RegisterUser(t, "sugbig", "sugbig@example.com")
	app.RegisterUser(t, "sugfollowed", "sugfollowed@example.com")

	// Give "sugbig" a follower head start via the profile counter column.
	if _, err := app.DB.Exec(`UPDATE user_profiles SET followers_count = 5000 WHERE user_id = (SELECT user_id FROM users WHERE username='sugbig')`); err != nil {
		t.Fatalf("bump follower count: %v", err)
	}

	// Follow "sugfollowed" so it should be excluded.
	if rec := app.Do(t, testutil.Request{Method: http.MethodPost, Path: "/api/v1/users/sugfollowed/follow", Token: viewerToken}); rec.Code != http.StatusOK {
		t.Fatalf("follow: %d %s", rec.Code, rec.Body.String())
	}

	suggested, _ := testutil.Decode[map[string]any](t, app.Do(t, testutil.Request{Method: http.MethodGet, Path: "/api/v1/users/suggested", Token: viewerToken}))
	items := suggested["items"].([]any)
	if len(items) == 0 {
		t.Fatalf("suggested empty")
	}
	// "sugfollowed" must not appear (viewer follows them).
	for _, u := range items {
		uname := u.(map[string]any)["username"].(string)
		if uname == "sugfollowed" || uname == "sugviewer" {
			t.Fatalf("suggested contains excluded user %s", uname)
		}
	}
	// The viewer's own suggested badge field is hydrated.
	if _, ok := items[0].(map[string]any)["badges"]; !ok {
		t.Fatalf("suggested item missing badges field")
	}
}

func TestDMs(t *testing.T) {
	app := testutil.NewApp(t, testutil.Database(t))
	aliceToken := app.RegisterUser(t, "dmalice", "dmalice@example.com")
	bobToken := app.RegisterUser(t, "dmbob", "dmbob@example.com")
	carolToken := app.RegisterUser(t, "dmcarol", "dmcarol@example.com")

	// Self-messaging is rejected.
	if rec := app.Do(t, testutil.Request{
		Method: http.MethodPost,
		Path:   "/api/v1/dms/dmalice",
		Token:  aliceToken,
		Body:   map[string]string{"body": "hello self"},
	}); rec.Code != http.StatusBadRequest {
		t.Fatalf("self DM status = %d, want 400", rec.Code)
	}

	// Send. Conversation is created on first contact.
	var conversationID int
	{
		sent, _ := testutil.Decode[map[string]any](t, app.Do(t, testutil.Request{
			Method: http.MethodPost,
			Path:   "/api/v1/dms/dmbob",
			Token:  aliceToken,
			Body:   map[string]string{"body": "hey bob"},
		}))
		if sent["conversation_id"] == nil {
			t.Fatalf("sent message missing conversation_id: %v", sent)
		}
		conversationID = int(sent["conversation_id"].(float64))
	}

	// Reusing the same pair must not create a second conversation.
	{
		sent, _ := testutil.Decode[map[string]any](t, app.Do(t, testutil.Request{
			Method: http.MethodPost,
			Path:   "/api/v1/dms/dmalice",
			Token:  bobToken,
			Body:   map[string]string{"body": "hi alice"},
		}))
		if int(sent["conversation_id"].(float64)) != conversationID {
			t.Fatalf("conversation id changed on reuse: %v", conversationID)
		}
	}

	// Alice has an unread count of 1 (bob's message); her inbox should list it.
	{
		count, _ := testutil.Decode[map[string]any](t, app.Do(t, testutil.Request{Method: http.MethodGet, Path: "/api/v1/dms/unread-count", Token: aliceToken}))
		if int(count["unread_count"].(float64)) != 1 {
			t.Fatalf("alice unread = %v, want 1", count["unread_count"])
		}
		inbox, _ := testutil.Decode[map[string]any](t, app.Do(t, testutil.Request{Method: http.MethodGet, Path: "/api/v1/dms/conversations", Token: aliceToken}))
		items := inbox["items"].([]any)
		if len(items) != 1 {
			t.Fatalf("alice inbox len = %d, want 1", len(items))
		}
		conv := items[0].(map[string]any)
		if conv["other_participant"].(map[string]any)["username"] != "dmbob" {
			t.Fatalf("inbox other_participant = %v", conv["other_participant"])
		}
	}

	// Non-participant cannot read the conversation -> 404.
	if rec := app.Do(t, testutil.Request{Method: http.MethodGet, Path: "/api/v1/dms/conversations/" + itoa(conversationID) + "/messages", Token: carolToken}); rec.Code != http.StatusNotFound {
		t.Fatalf("non-participant messages status = %d, want 404", rec.Code)
	}

	// Carol is blocked by Alice (Alice -> Carol block); Alice cannot DM Carol.
	if rec := app.Do(t, testutil.Request{Method: http.MethodPost, Path: "/api/v1/users/dmcarol/block", Token: aliceToken}); rec.Code != http.StatusOK {
		t.Fatalf("block carol failed: %d %s", rec.Code, rec.Body.String())
	}
	if rec := app.Do(t, testutil.Request{
		Method: http.MethodPost,
		Path:   "/api/v1/dms/dmcarol",
		Token:  aliceToken,
		Body:   map[string]string{"body": "hey carol"},
	}); rec.Code != http.StatusForbidden {
		t.Fatalf("DM to blocked user status = %d, want 403", rec.Code)
	}
	// The reverse direction (Carol -> Alice) must also be suppressed.
	if rec := app.Do(t, testutil.Request{
		Method: http.MethodPost,
		Path:   "/api/v1/dms/dmalice",
		Token:  carolToken,
		Body:   map[string]string{"body": "hey alice"},
	}); rec.Code != http.StatusForbidden {
		t.Fatalf("DM from blocked user status = %d, want 403", rec.Code)
	}

	// Mark-read drops Alice's unread count to 0.
	if rec := app.Do(t, testutil.Request{Method: http.MethodPost, Path: "/api/v1/dms/conversations/" + itoa(conversationID) + "/read", Token: aliceToken}); rec.Code != http.StatusOK {
		t.Fatalf("mark read failed: %d %s", rec.Code, rec.Body.String())
	}
	count, _ := testutil.Decode[map[string]any](t, app.Do(t, testutil.Request{Method: http.MethodGet, Path: "/api/v1/dms/unread-count", Token: aliceToken}))
	if int(count["unread_count"].(float64)) != 0 {
		t.Fatalf("alice unread after mark-read = %v, want 0", count["unread_count"])
	}

	// Body validation: empty body -> 400.
	if rec := app.Do(t, testutil.Request{
		Method: http.MethodPost,
		Path:   "/api/v1/dms/dmbob",
		Token:  aliceToken,
		Body:   map[string]string{"body": "   "},
	}); rec.Code != http.StatusBadRequest {
		t.Fatalf("blank body status = %d, want 400", rec.Code)
	}

	// A single message list round-trip after several sends (pagination is covered
	// by the cursor in older tests; here assert at least the messages are there).
	messages, _ := testutil.Decode[map[string]any](t, app.Do(t, testutil.Request{Method: http.MethodGet, Path: "/api/v1/dms/conversations/" + itoa(conversationID) + "/messages", Token: aliceToken}))
	if len(messages["items"].([]any)) != 2 {
		t.Fatalf("alice message count = %d, want 2", len(messages["items"].([]any)))
	}
}

func TestProfileRelationshipManageAndFeeds(t *testing.T) {
	app := testutil.NewApp(t, testutil.Database(t))
	aliceToken := app.RegisterUser(t, "relalice", "relalice@example.com")
	bobToken := app.RegisterUser(t, "relbob", "relbob@example.com")

	// Alice creates a top-level post and a reply to it.
	root, _ := testutil.Decode[map[string]any](t, app.Do(t, testutil.Request{
		Method: http.MethodPost,
		Path:   "/api/v1/posts/",
		Token:  aliceToken,
		Body:   map[string]string{"content": "top level"},
	}))
	app.Do(t, testutil.Request{
		Method: http.MethodPost,
		Path:   "/api/v1/posts/",
		Token:  aliceToken,
		Body:   map[string]any{"content": "a reply", "parent_id": int(root["id"].(float64))},
	})

	profileOf := func() map[string]any {
		d, _ := testutil.Decode[map[string]any](t, app.Do(t, testutil.Request{Method: http.MethodGet, Path: "/api/v1/users/relalice", Token: bobToken}))
		return d
	}

	// Bob follows Alice -> profile reflects is_following.
	if rec := app.Do(t, testutil.Request{Method: http.MethodPost, Path: "/api/v1/users/relalice/follow", Token: bobToken}); rec.Code != http.StatusOK {
		t.Fatalf("follow failed: %d %s", rec.Code, rec.Body.String())
	}
	if p := profileOf(); p["is_following"] != true {
		t.Fatalf("profile after follow: is_following = %v, want true", p["is_following"])
	}

	// Muting coexists with following and flips is_muted.
	if rec := app.Do(t, testutil.Request{Method: http.MethodPost, Path: "/api/v1/users/relalice/mute", Token: bobToken}); rec.Code != http.StatusOK {
		t.Fatalf("mute failed: %d %s", rec.Code, rec.Body.String())
	}
	if p := profileOf(); p["is_following"] != true || p["is_muted"] != true {
		t.Fatalf("profile after mute: is_following=%v is_muted=%v, want true/true", p["is_following"], p["is_muted"])
	}

	// Muting twice is idempotent.
	if rec := app.Do(t, testutil.Request{Method: http.MethodPost, Path: "/api/v1/users/relalice/mute", Token: bobToken}); rec.Code != http.StatusOK {
		t.Fatalf("re-mute failed: %d %s", rec.Code, rec.Body.String())
	}

	// While muted, alice's likes must not notify bob.
	bobPostOne, _ := testutil.Decode[map[string]any](t, app.Do(t, testutil.Request{Method: http.MethodPost, Path: "/api/v1/posts/", Token: bobToken, Body: map[string]string{"content": "bob post one"}}))
	if rec := app.Do(t, testutil.Request{Method: http.MethodPost, Path: "/api/v1/posts/" + itoa(int(bobPostOne["id"].(float64))) + "/like", Token: aliceToken}); rec.Code != http.StatusOK {
		t.Fatalf("alice like while muted failed: %d %s", rec.Code, rec.Body.String())
	}
	bobNotifs, _ := testutil.Decode[map[string]any](t, app.Do(t, testutil.Request{Method: http.MethodGet, Path: "/api/v1/notifications", Token: bobToken}))
	if len(bobNotifs["items"].([]any)) != 0 {
		t.Fatalf("bob notifications while alice muted = %v, want none", bobNotifs["items"])
	}

	// Unmute clears is_muted but keeps is_following.
	if rec := app.Do(t, testutil.Request{Method: http.MethodDelete, Path: "/api/v1/users/relalice/mute", Token: bobToken}); rec.Code != http.StatusNoContent {
		t.Fatalf("unmute failed: %d %s", rec.Code, rec.Body.String())
	}
	if p := profileOf(); p["is_following"] != true || p["is_muted"] == true {
		t.Fatalf("profile after unmute: is_following=%v is_muted=%v, want true/false", p["is_following"], p["is_muted"])
	}

	// After unmute a like from alice does notify bob.
	bobPostTwo, _ := testutil.Decode[map[string]any](t, app.Do(t, testutil.Request{Method: http.MethodPost, Path: "/api/v1/posts/", Token: bobToken, Body: map[string]string{"content": "bob post two"}}))
	if rec := app.Do(t, testutil.Request{Method: http.MethodPost, Path: "/api/v1/posts/" + itoa(int(bobPostTwo["id"].(float64))) + "/like", Token: aliceToken}); rec.Code != http.StatusOK {
		t.Fatalf("alice like after unmute failed: %d %s", rec.Code, rec.Body.String())
	}
	bobNotifsAfter, _ := testutil.Decode[map[string]any](t, app.Do(t, testutil.Request{Method: http.MethodGet, Path: "/api/v1/notifications", Token: bobToken}))
	afterItems := bobNotifsAfter["items"].([]any)
	if len(afterItems) != 1 {
		t.Fatalf("bob notifications after unmute = %d, want 1", len(afterItems))
	}
	if afterItems[0].(map[string]any)["actor"].(map[string]any)["username"] != "relalice" {
		t.Fatalf("bob notification actor = %v, want relalice", afterItems[0])
	}

	// Following list uses the flat `items` shape and carries viewer status.
	following, _ := testutil.Decode[map[string]any](t, app.Do(t, testutil.Request{Method: http.MethodGet, Path: "/api/v1/users/relbob/following", Token: bobToken}))
	followingItems := following["items"].([]any)
	if len(followingItems) != 1 {
		t.Fatalf("bob following count = %d, want 1", len(followingItems))
	}
	f := followingItems[0].(map[string]any)
	if f["username"] != "relalice" || f["is_following"] != true {
		t.Fatalf("following item = %v", f)
	}
	if f["display_name"] == nil {
		t.Fatalf("following item missing flat profile fields: %v", f)
	}

	// Followers list matches too (bob is alice's only follower).
	followers, _ := testutil.Decode[map[string]any](t, app.Do(t, testutil.Request{Method: http.MethodGet, Path: "/api/v1/users/relalice/followers", Token: bobToken}))
	followersItems := followers["items"].([]any)
	if len(followersItems) != 1 || followersItems[0].(map[string]any)["username"] != "relbob" {
		t.Fatalf("alice followers = %v", followers["items"])
	}

	// Blocking removes the follow and exposes is_blocked.
	if rec := app.Do(t, testutil.Request{Method: http.MethodPost, Path: "/api/v1/users/relalice/block", Token: bobToken}); rec.Code != http.StatusOK {
		t.Fatalf("block failed: %d %s", rec.Code, rec.Body.String())
	}
	if p := profileOf(); p["is_blocked"] != true || p["is_following"] == true {
		t.Fatalf("profile after block: is_blocked=%v is_following=%v, want true/false", p["is_blocked"], p["is_following"])
	}

	// Unblock clears it.
	if rec := app.Do(t, testutil.Request{Method: http.MethodDelete, Path: "/api/v1/users/relalice/block", Token: bobToken}); rec.Code != http.StatusNoContent {
		t.Fatalf("unblock failed: %d %s", rec.Code, rec.Body.String())
	}
	if p := profileOf(); p["is_blocked"] == true {
		t.Fatalf("profile after unblock still blocked: %v", p["is_blocked"])
	}

	// Replies feed contains only replies.
	replies, _ := testutil.Decode[map[string]any](t, app.Do(t, testutil.Request{Method: http.MethodGet, Path: "/api/v1/users/relalice/replies", Token: bobToken}))
	replyItems := replies["items"].([]any)
	if len(replyItems) != 1 || replyItems[0].(map[string]any)["content"] != "a reply" {
		t.Fatalf("replies feed = %v", replies["items"])
	}

	// Default user feed contains only the top-level post.
	posts, _ := testutil.Decode[map[string]any](t, app.Do(t, testutil.Request{Method: http.MethodGet, Path: "/api/v1/users/relalice/posts", Token: bobToken}))
	postItems := posts["items"].([]any)
	if len(postItems) != 1 || postItems[0].(map[string]any)["id"].(float64) != root["id"].(float64) {
		t.Fatalf("user feed = %v", posts["items"])
	}

	// Media feed: attach media to the top-level post directly, then assert the
	// media endpoint only returns that post.
	postID := int(root["id"].(float64))
	var mediaUUID string
	if err := app.DB.QueryRow(`INSERT INTO media (media_uuid, mime_type, filename) VALUES (gen_random_uuid(), 'image/png', 'pic.png') RETURNING media_uuid::text`).Scan(&mediaUUID); err != nil {
		t.Fatalf("insert media: %v", err)
	}
	if _, err := app.DB.Exec(`INSERT INTO post_media (post_id, media_uuid, position, alt_text) VALUES ($1, $2::uuid, 1, '')`, postID, mediaUUID); err != nil {
		t.Fatalf("link media to post: %v", err)
	}

	mediaFeed, _ := testutil.Decode[map[string]any](t, app.Do(t, testutil.Request{Method: http.MethodGet, Path: "/api/v1/users/relalice/media", Token: bobToken}))
	mediaItems := mediaFeed["items"].([]any)
	if len(mediaItems) != 1 || int(mediaItems[0].(map[string]any)["id"].(float64)) != postID {
		t.Fatalf("media feed = %v", mediaFeed["items"])
	}
	if len(mediaItems[0].(map[string]any)["media"].([]any)) != 1 {
		t.Fatalf("media feed item missing media attachments: %v", mediaItems[0])
	}
}

// TestPostVisibility enforces the per-post visibility rule across reads and
// engagement writes: public posts are open to everyone, followers-only posts
// are restricted to the author + their followers, and mentions-only posts are
// restricted to the author + the users @mentioned in the content.
func TestPostVisibility(t *testing.T) {
	app := testutil.NewApp(t, testutil.Database(t))
	alice := app.RegisterUser(t, "visalice", "visalice@example.com")
	bob := app.RegisterUser(t, "visbob", "visbob@example.com")
	carol := app.RegisterUser(t, "viscarol", "viscarol@example.com")

	if rec := app.Do(t, testutil.Request{Method: http.MethodPost, Path: "/api/v1/users/visalice/follow", Token: bob}); rec.Code != http.StatusOK {
		t.Fatalf("bob follow alice: %d %s", rec.Code, rec.Body.String())
	}

	create := func(token string, body map[string]any) int {
		rec := app.Do(t, testutil.Request{Method: http.MethodPost, Path: "/api/v1/posts/", Token: alice, Body: body})
		if rec.Code != http.StatusCreated {
			t.Fatalf("create post %v: %d %s", body, rec.Code, rec.Body.String())
		}
		data, _ := testutil.Decode[map[string]any](t, rec)
		return int(data["id"].(float64))
	}

	publicID := create(alice, map[string]any{"content": "public hello"})
	followersID := create(alice, map[string]any{"content": "followers only", "visibility": "followers"})
	mentionsID := create(alice, map[string]any{"content": "@visbob hi", "visibility": "mentions"})

	// A mentions-only post that mentions nobody is rejected.
	if rec := app.Do(t, testutil.Request{Method: http.MethodPost, Path: "/api/v1/posts/", Token: alice, Body: map[string]any{"content": "nobody", "visibility": "mentions"}}); rec.Code != http.StatusBadRequest {
		t.Fatalf("mentions-only with no mention: expected 400 got %d %s", rec.Code, rec.Body.String())
	}
	// An unknown visibility value is rejected.
	if rec := app.Do(t, testutil.Request{Method: http.MethodPost, Path: "/api/v1/posts/", Token: alice, Body: map[string]any{"content": "bad", "visibility": "secret"}}); rec.Code != http.StatusBadRequest {
		t.Fatalf("invalid visibility: expected 400 got %d %s", rec.Code, rec.Body.String())
	}

	getCode := func(token string, postID int) int {
		rec := app.Do(t, testutil.Request{Method: http.MethodGet, Path: "/api/v1/posts/" + itoa(postID), Token: token})
		return rec.Code
	}

	// The author sees everything.
	for _, id := range []int{publicID, followersID, mentionsID} {
		if code := getCode(alice, id); code != http.StatusOK {
			t.Fatalf("author should see post %d, got %d", id, code)
		}
	}

	// Bob follows alice and is mentioned: sees everything.
	for _, id := range []int{publicID, followersID, mentionsID} {
		if code := getCode(bob, id); code != http.StatusOK {
			t.Fatalf("follower (and mentioned) should see post %d, got %d", id, code)
		}
	}

	// Carol is neither a follower nor mentioned: only the public post.
	if code := getCode(carol, publicID); code != http.StatusOK {
		t.Fatalf("stranger should see public post, got %d", code)
	}
	if code := getCode(carol, followersID); code != http.StatusNotFound {
		t.Fatalf("stranger must not see followers-only post, got %d", code)
	}
	if code := getCode(carol, mentionsID); code != http.StatusNotFound {
		t.Fatalf("stranger must not see mentions-only post, got %d", code)
	}

	// Carol's view of alice's profile feed excludes the restricted posts.
	rec := app.Do(t, testutil.Request{Method: http.MethodGet, Path: "/api/v1/users/visalice/posts", Token: carol})
	feed, _ := testutil.Decode[map[string]any](t, rec)
	items := feed["items"].([]any)
	if len(items) != 1 || int(items[0].(map[string]any)["id"].(float64)) != publicID {
		t.Fatalf("stranger user feed should contain only the public post, got %v", feed["items"])
	}

	// Carol cannot like a post she cannot read.
	if rec := app.Do(t, testutil.Request{Method: http.MethodPost, Path: "/api/v1/posts/" + itoa(followersID) + "/like", Token: carol}); rec.Code != http.StatusNotFound {
		t.Fatalf("stranger like on followers-only post: expected 404 got %d %s", rec.Code, rec.Body.String())
	}
	// Bob's like succeeds (he can read the post).
	if rec := app.Do(t, testutil.Request{Method: http.MethodPost, Path: "/api/v1/posts/" + itoa(followersID) + "/like", Token: bob}); rec.Code != http.StatusOK {
		t.Fatalf("follower like: %d %s", rec.Code, rec.Body.String())
	}
}

// TestAccountPrivacy enforces the account-level profileVisibility: private
// accounts keep a public profile shell (username/bio/counters) but only expose
// their posts and content to followers. Toggling back to public restores
// access. Both "private" and "friends" map to followers-only.
func TestAccountPrivacy(t *testing.T) {
	app := testutil.NewApp(t, testutil.Database(t))
	alice := app.RegisterUser(t, "priv_alice", "priv_alice@example.com")
	bob := app.RegisterUser(t, "priv_bob", "priv_bob@example.com")
	carol := app.RegisterUser(t, "priv_carol", "priv_carol@example.com")

	// Make alice's account private via the settings endpoint.
	patch := app.Do(t, testutil.Request{Method: http.MethodPatch, Path: "/api/v1/users/settings", Token: alice, Body: map[string]any{
		"privacy": map[string]any{"profileVisibility": "private"},
	}})
	if patch.Code != http.StatusOK {
		t.Fatalf("set private: %d %s", patch.Code, patch.Body.String())
	}

	// alice posts a public post.
	rec := app.Do(t, testutil.Request{Method: http.MethodPost, Path: "/api/v1/posts/", Token: alice, Body: map[string]string{"content": "private alice post"}})
	if rec.Code != http.StatusCreated {
		t.Fatalf("create post: %d %s", rec.Code, rec.Body.String())
	}
	data, _ := testutil.Decode[map[string]any](t, rec)
	postID := int(data["id"].(float64))

	// Profile shell is still visible to a stranger, but flags is_private.
	rec = app.Do(t, testutil.Request{Method: http.MethodGet, Path: "/api/v1/users/priv_alice", Token: carol})
	profile, _ := testutil.Decode[map[string]any](t, rec)
	if isPrivate, _ := profile["is_private"].(bool); !isPrivate {
		t.Fatalf("profile should report is_private=true, got %v", profile)
	}

	// Stranger: private posts and feeds are hidden (as if the post doesn't exist).
	if code := app.Do(t, testutil.Request{Method: http.MethodGet, Path: "/api/v1/posts/" + itoa(postID), Token: carol}).Code; code != http.StatusNotFound {
		t.Fatalf("stranger single post: expected 404 got %d", code)
	}
	rec = app.Do(t, testutil.Request{Method: http.MethodGet, Path: "/api/v1/users/priv_alice/posts", Token: carol})
	feed, _ := testutil.Decode[map[string]any](t, rec)
	if items := feed["items"].([]any); len(items) != 0 {
		t.Fatalf("stranger profile feed should be empty, got %v", items)
	}

	// Follower: sees the content.
	if rec := app.Do(t, testutil.Request{Method: http.MethodPost, Path: "/api/v1/users/priv_alice/follow", Token: bob}); rec.Code != http.StatusOK {
		t.Fatalf("bob follow alice: %d %s", rec.Code, rec.Body.String())
	}
	if code := app.Do(t, testutil.Request{Method: http.MethodGet, Path: "/api/v1/posts/" + itoa(postID), Token: bob}).Code; code != http.StatusOK {
		t.Fatalf("follower single post: expected 200 got %d", code)
	}
	rec = app.Do(t, testutil.Request{Method: http.MethodGet, Path: "/api/v1/users/priv_alice/posts", Token: bob})
	feed, _ = testutil.Decode[map[string]any](t, rec)
	if items := feed["items"].([]any); len(items) != 1 {
		t.Fatalf("follower profile feed should have 1 post, got %v", items)
	}

	// "friends" also means followers-only.
	rec = app.Do(t, testutil.Request{Method: http.MethodPatch, Path: "/api/v1/users/settings", Token: alice, Body: map[string]any{
		"privacy": map[string]any{"profileVisibility": "friends"},
	}})
	if rec.Code != http.StatusOK {
		t.Fatalf("set friends: %d %s", rec.Code, rec.Body.String())
	}
	if code := app.Do(t, testutil.Request{Method: http.MethodGet, Path: "/api/v1/posts/" + itoa(postID), Token: carol}).Code; code != http.StatusNotFound {
		t.Fatalf("friends-visibility hides content from strangers: expected 404 got %d", code)
	}

	// Back to public: the stranger regains access.
	rec = app.Do(t, testutil.Request{Method: http.MethodPatch, Path: "/api/v1/users/settings", Token: alice, Body: map[string]any{
		"privacy": map[string]any{"profileVisibility": "public"},
	}})
	if rec.Code != http.StatusOK {
		t.Fatalf("set public: %d %s", rec.Code, rec.Body.String())
	}
	if code := app.Do(t, testutil.Request{Method: http.MethodGet, Path: "/api/v1/posts/" + itoa(postID), Token: carol}).Code; code != http.StatusOK {
		t.Fatalf("after public, stranger should see post: expected 200 got %d", code)
	}
}

// TestProfileUpdateAllowsEmptyOptionalFields: PATCH /users/me used to reject
// empty/short bio, location, and website (they carried `required`/`min` tags
// while the DB defaults them to ” for new accounts), so a fresh user could
// never save a profile edit or clear those fields.
func TestProfileUpdateAllowsEmptyOptionalFields(t *testing.T) {
	app := testutil.NewApp(t, testutil.Database(t))
	token := app.RegisterUser(t, "alicee", "alicee@example.com")

	// All optional fields empty / unset defaults -> 200 (used to 400).
	rec := app.Do(t, testutil.Request{
		Method: http.MethodPatch,
		Path:   "/api/v1/users/me",
		Token:  token,
		Body:   map[string]string{"display_name": "Alice", "bio": "", "location": "", "website": "", "birth_date": ""},
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("profile update with empty optional fields: expected 200 got %d body %s", rec.Code, rec.Body.String())
	}

	// Short values are allowed too (no min floor anymore).
	if rec := app.Do(t, testutil.Request{
		Method: http.MethodPatch,
		Path:   "/api/v1/users/me",
		Token:  token,
		Body:   map[string]string{"display_name": "Al", "bio": "hi", "location": "NY", "website": "x.io", "birth_date": ""},
	}); rec.Code != http.StatusOK {
		t.Fatalf("profile update with short values: expected 200 got %d body %s", rec.Code, rec.Body.String())
	}

	// Clearing a previously-set value persists.
	rec = app.Do(t, testutil.Request{
		Method: http.MethodPatch,
		Path:   "/api/v1/users/me",
		Token:  token,
		Body:   map[string]string{"display_name": "Alice", "bio": "", "location": "NY", "website": "", "birth_date": ""},
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("clear website: expected 200 got %d body %s", rec.Code, rec.Body.String())
	}
	me, _ := testutil.Decode[map[string]any](t, rec)
	if me["website"] != "" {
		t.Fatalf("cleared website should persist as '', got %v", me["website"])
	}
}

// TestUsernameCharsetEnforced: the signup UI constrains usernames to
// [a-zA-Z0-9_] but the API previously didn't, so direct callers could create
// usernames that break mention parsing and profile URLs.
func TestUsernameCharsetEnforced(t *testing.T) {
	app := testutil.NewApp(t, testutil.Database(t))

	for _, bad := range []string{"foo bar", "foo-bar", "foo@bar"} {
		if rec := app.Do(t, testutil.Request{
			Method: http.MethodPost,
			Path:   "/api/v1/auth/register",
			Body:   map[string]string{"username": bad, "email": "x" + strings.ReplaceAll(bad, "@", "_at_") + "@example.com", "password": "password123"},
		}); rec.Code != http.StatusBadRequest {
			t.Fatalf("register username %q: expected 400 got %d body %s", bad, rec.Code, rec.Body.String())
		}
	}

	if rec := app.Do(t, testutil.Request{
		Method: http.MethodPost,
		Path:   "/api/v1/auth/register",
		Body:   map[string]string{"username": "foo_bar", "email": "foo.bar@example.com", "password": "password123"},
	}); rec.Code != http.StatusCreated {
		t.Fatalf("register valid underscored username: expected 201 got %d body %s", rec.Code, rec.Body.String())
	}
}

// TestPostContentLengthRejected: posts.content is VARCHAR(280); the API used to
// rely on Postgres to reject over-length content, which surfaced as a 500.
func TestPostContentLengthRejected(t *testing.T) {
	app := testutil.NewApp(t, testutil.Database(t))
	token := app.RegisterUser(t, "ppp", "ppp@example.com")

	// Exactly 280 chars is fine.
	rec := app.Do(t, testutil.Request{
		Method: http.MethodPost,
		Path:   "/api/v1/posts/",
		Token:  token,
		Body:   map[string]string{"content": strings.Repeat("a", 280)},
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("280-char post: expected 201 got %d body %s", rec.Code, rec.Body.String())
	}
	postData, _ := testutil.Decode[map[string]any](t, rec)
	postID := int(postData["id"].(float64))

	// 281 chars -> 400, not 500.
	if rec := app.Do(t, testutil.Request{
		Method: http.MethodPost,
		Path:   "/api/v1/posts/",
		Token:  token,
		Body:   map[string]string{"content": strings.Repeat("a", 281)},
	}); rec.Code != http.StatusBadRequest {
		t.Fatalf("281-char post: expected 400 got %d body %s", rec.Code, rec.Body.String())
	}

	// Update to an over-length content -> 400 too.
	if rec := app.Do(t, testutil.Request{
		Method: http.MethodPatch,
		Path:   "/api/v1/posts/" + itoa(postID),
		Token:  token,
		Body:   map[string]string{"content": strings.Repeat("b", 281)},
	}); rec.Code != http.StatusBadRequest {
		t.Fatalf("update to 281-char content: expected 400 got %d body %s", rec.Code, rec.Body.String())
	}
}

// TestPollQuestionLengthRejected: polls.question is VARCHAR(140) but the
// question was never length-checked, so an over-long question hit the DB and
// returned 500.
func TestPollQuestionLengthRejected(t *testing.T) {
	app := testutil.NewApp(t, testutil.Database(t))
	token := app.RegisterUser(t, "ppq", "ppq@example.com")

	rec := app.Do(t, testutil.Request{
		Method: http.MethodPost,
		Path:   "/api/v1/posts/",
		Token:  token,
		Body: map[string]any{
			"content": strings.Repeat("q", 141),
			"poll":    map[string]any{"options": []string{"a", "b"}},
		},
	})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("141-char poll question: expected 400 got %d body %s", rec.Code, rec.Body.String())
	}

	if rec := app.Do(t, testutil.Request{
		Method: http.MethodPost,
		Path:   "/api/v1/posts/",
		Token:  token,
		Body: map[string]any{
			"content": strings.Repeat("p", 140),
			"poll":    map[string]any{"options": []string{"a", "b"}},
		},
	}); rec.Code != http.StatusCreated {
		t.Fatalf("140-char poll question: expected 201 got %d body %s", rec.Code, rec.Body.String())
	}
}

// TestNewsAttachmentLifecycle covers the news link attachment on posts:
// create with news round-trips the OpenGraph metadata, feeds hydrate it, and
// posts without news simply omit the field.
func TestNewsAttachmentLifecycle(t *testing.T) {
	app := testutil.NewApp(t, testutil.Database(t))
	token := app.RegisterUser(t, "news_a", "news_a@example.com")
	follower := app.RegisterUser(t, "news_b", "news_b@example.com")
	followRec := app.Do(t, testutil.Request{
		Method: http.MethodPost,
		Path:   "/api/v1/users/" + "news_a" + "/follow",
		Token:  follower,
	})
	if followRec.Code != http.StatusOK {
		t.Fatalf("follow failed: %d %s", followRec.Code, followRec.Body.String())
	}

	news := map[string]any{
		"url":       "https://news.example.com/rescue",
		"title":     "Firefighters rescue kitten from a tree",
		"image_url": "https://news.example.com/firetruck.jpg",
		"site_name": "Daily News",
	}

	rec := app.Do(t, testutil.Request{
		Method: http.MethodPost,
		Path:   "/api/v1/posts/",
		Token:  token,
		Body: map[string]any{
			"content": "breaking: check the article",
			"media":   []any{},
			"news":    news,
		},
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("create news post: expected 201 got %d body %s", rec.Code, rec.Body.String())
	}
	created, _ := testutil.Decode[map[string]any](t, rec)
	postID := int(created["id"].(float64))

	// Single-post read hydrates news.
	got := app.Do(t, testutil.Request{Method: http.MethodGet, Path: "/api/v1/posts/" + itoa(postID), Token: token})
	postData, _ := testutil.Decode[map[string]any](t, got)
	detailed := postData["post"].(map[string]any)
	if detailed["news"] == nil {
		t.Fatalf("created post missing news attachment: %s", got.Body.String())
	}
	gotNews := detailed["news"].(map[string]any)
	if gotNews["title"] != news["title"] {
		t.Fatalf("news title = %v, want %v", gotNews["title"], news["title"])
	}
	if gotNews["image_url"] != news["image_url"] {
		t.Fatalf("news image_url = %v, want %v", gotNews["image_url"], news["image_url"])
	}
	if gotNews["site_name"] != news["site_name"] {
		t.Fatalf("news site_name = %v, want %v", gotNews["site_name"], news["site_name"])
	}

	// Home feed hydrates news on the follower side.
	feedRec := app.Do(t, testutil.Request{Method: http.MethodGet, Path: "/api/v1/posts/feed", Token: follower})
	feed, _ := testutil.Decode[map[string]any](t, feedRec)
	found := false
	for _, item := range feed["items"].([]any) {
		post := item.(map[string]any)
		if int(post["id"].(float64)) != postID {
			continue
		}
		found = true
		if post["news"] == nil {
			t.Fatalf("home feed post %d missing news hydration", postID)
		}
	}
	if !found {
		t.Fatalf("home feed did not include the news post")
	}

	// User feed hydrates news too.
	userFeed := app.Do(t, testutil.Request{Method: http.MethodGet, Path: "/api/v1/users/news_a/posts", Token: token})
	feedData, _ := testutil.Decode[map[string]any](t, userFeed)
	foundUser := false
	for _, item := range feedData["items"].([]any) {
		post := item.(map[string]any)
		if int(post["id"].(float64)) != postID {
			continue
		}
		foundUser = true
		if post["news"] == nil {
			t.Fatalf("user feed post %d missing news hydration", postID)
		}
	}
	if !foundUser {
		t.Fatalf("user feed did not include the news post")
	}

	// A plain post has no news field at all.
	plain := app.Do(t, testutil.Request{
		Method: http.MethodPost,
		Path:   "/api/v1/posts/",
		Token:  token,
		Body:   map[string]any{"content": "no news here", "media": []any{}},
	})
	plainData, _ := testutil.Decode[map[string]any](t, plain)
	if _, present := plainData["news"]; present {
		t.Fatalf("plain post should not carry a news field")
	}

	// Search hydration includes news.
	searchRec := app.Do(t, testutil.Request{
		Method: http.MethodGet,
		Path:   "/api/v1/search?type=posts&q=breaking",
		Token:  token,
	})
	searchData, _ := testutil.Decode[map[string]any](t, searchRec)
	for _, item := range searchData["items"].([]any) {
		post := item.(map[string]any)
		if int(post["id"].(float64)) != postID {
			continue
		}
		if post["news"] == nil {
			t.Fatalf("search result post %d missing news hydration", postID)
		}
	}

	// News is rejected on replies (mirrors the poll rule).
	replyRec := app.Do(t, testutil.Request{
		Method: http.MethodPost,
		Path:   "/api/v1/posts/",
		Token:  token,
		Body: map[string]any{
			"content":   "a reply with news",
			"parent_id": postID,
			"news":      news,
		},
	})
	if replyRec.Code != http.StatusBadRequest {
		t.Fatalf("news on a reply: expected 400 got %d body %s", replyRec.Code, replyRec.Body.String())
	}
}
