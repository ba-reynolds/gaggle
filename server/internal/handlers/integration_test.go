package handlers_test

import (
	"encoding/json"
	"net/http"
	"strconv"
	"testing"

	"github.com/ba-reynolds/gophersocial/internal/testutil"
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
			"description": "GopherSocial staff member",
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
	if len(myLists) != 1 {
		t.Fatalf("my lists = %d, want 1", len(myLists))
	}
	profileLists, _ := testutil.Decode[[]any](t, app.Do(t, testutil.Request{Method: http.MethodGet, Path: "/api/v1/users/listowner/lists", Token: otherToken}))
	if len(profileLists) != 1 {
		t.Fatalf("profile lists = %d, want 1", len(profileLists))
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
