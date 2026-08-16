package handlers_test

import (
	"encoding/json"
	"net/http"
	"strconv"
	"testing"

	"github.com/ba-reynolds/vitrilium/internal/testutil"
)

func itoa(i int) string { return strconv.Itoa(i) }

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

	// Duplicate email -> 409
	rec = app.Do(t, testutil.Request{
		Method: http.MethodPost,
		Path:   "/api/v1/auth/register",
		Body:   map[string]string{"username": "tester2", "email": "tester@example.com", "password": "password123"},
	})
	if rec.Code != http.StatusConflict {
		t.Fatalf("duplicate email: expected 409 got %d", rec.Code)
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

	// A follows B
	rec := app.Do(t, testutil.Request{Method: http.MethodPost, Path: "/api/v1/users/feedb/follow", Token: tokenA})
	if rec.Code != http.StatusOK {
		t.Fatalf("follow failed: %d %s", rec.Code, rec.Body.String())
	}

	feed, _ := testutil.Decode[map[string]any](t, app.Do(t, testutil.Request{Method: http.MethodGet, Path: "/api/v1/posts/feed", Token: tokenA}))
	if len(feed["items"].([]any)) != 1 {
		t.Fatalf("home feed should have 1 post from followed user, got %v", feed["items"])
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
