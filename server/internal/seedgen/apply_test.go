package seedgen

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ba-reynolds/gaggle/internal/store"
	"github.com/ba-reynolds/gaggle/internal/testutil"
	"github.com/google/uuid"
)

// seedEngine builds a store bound to the test DB, generates the fixed dataset,
// and Apply()s it once. Returns the store, dataset, and the shared media dir.
func seedEngine(t *testing.T) (*store.Store, *Dataset, string) {
	t.Helper()
	db := testutil.Database(t)
	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))
	mediaDir := t.TempDir()
	st := store.NewStore(db, log, mediaDir)
	ds := GenerateFixed(fixedNow())
	ctx := context.Background()
	if err := Apply(ctx, st, log, ds, ApplyOptions{MediaDir: mediaDir}); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	return st, ds, mediaDir
}

func TestApplyUsersAndProfiles(t *testing.T) {
	st, ds, _ := seedEngine(t)
	ctx := context.Background()

	if len(ds.UserIDs) != TotalUsers {
		t.Fatalf("ds.UserIDs = %d, want %d", len(ds.UserIDs), TotalUsers)
	}

	for i, u := range ds.Users {
		got, err := st.Users.GetByID(ctx, ds.UserIDs[i])
		if err != nil {
			t.Fatalf("GetByID(%d): %v", i, err)
		}
		if got.Username != u.Username {
			t.Errorf("user %d username = %q, want %q", i, got.Username, u.Username)
		}
	}

	// alice is admin, and profile/private flags landed.
	alice, err := st.Users.GetByUsername(ctx, "alice")
	if err != nil {
		t.Fatalf("GetByUsername(alice): %v", err)
	}
	if !alice.IsAdmin {
		t.Error("alice should be admin")
	}

	var profiles, privates, admins int
	if err := st.DB.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM user_profiles`).Scan(&profiles); err != nil {
		t.Fatalf("count profiles: %v", err)
	}
	if profiles != TotalUsers {
		t.Errorf("profiles = %d, want %d", profiles, TotalUsers)
	}
	if err := st.DB.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM users WHERE is_private = TRUE`).Scan(&privates); err != nil {
		t.Fatalf("count private: %v", err)
	}
	if privates != 3 {
		t.Errorf("private users = %d, want 3", privates)
	}
	if err := st.DB.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM users WHERE is_admin = TRUE`).Scan(&admins); err != nil {
		t.Fatalf("count admins: %v", err)
	}
	if admins != 1 {
		t.Errorf("admins = %d, want 1", admins)
	}
}

func TestApplyPostsBackdatedAndSynced(t *testing.T) {
	st, ds, _ := seedEngine(t)
	ctx := context.Background()

	if len(ds.PostIDs) != len(ds.Posts) {
		t.Fatalf("ds.PostIDs = %d, want %d", len(ds.PostIDs), len(ds.Posts))
	}

	// All posts are backdated within the 28-day window.
	var minTS, maxTS time.Time
	first := true
	for i := range ds.Posts {
		got, err := st.Posts.GetByID(ctx, ds.PostIDs[i])
		if err != nil {
			t.Fatalf("GetByID(post %d): %v", i, err)
		}
		if got.CreatedAt.IsZero() {
			t.Fatalf("post %d has zero created_at", i)
		}
		if first || got.CreatedAt.Before(minTS) {
			minTS = got.CreatedAt
		}
		if first || got.CreatedAt.After(maxTS) {
			maxTS = got.CreatedAt
		}
		first = false
	}
	if diff := maxTS.Sub(minTS); diff > DaysOfHistory*24*time.Hour {
		t.Errorf("post history spans %v, want <= %d days", diff, DaysOfHistory)
	}

	// Top-level count matches.
	var topLevel int
	var zeroParent int
	if err := st.DB.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM posts WHERE parent_id IS NULL`).Scan(&topLevel); err != nil {
		t.Fatalf("count top-level: %v", err)
	}
	if topLevel != TopLevelPosts {
		t.Errorf("top-level posts = %d, want %d", topLevel, TopLevelPosts)
	}
	if err := st.DB.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM posts WHERE parent_id IS NOT NULL`).Scan(&zeroParent); err != nil {
		t.Fatalf("count replies: %v", err)
	}
	if zeroParent != ReplyPosts {
		t.Errorf("replies = %d, want %d", zeroParent, ReplyPosts)
	}

	// Hashtags + mentions synced into the join tables.
	var hashtagRows, mentionRows int
	if err := st.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM post_hashtags`).Scan(&hashtagRows); err != nil {
		t.Fatalf("count post_hashtags: %v", err)
	}
	if hashtagRows == 0 {
		t.Error("no post_hashtags rows synced")
	}
	if err := st.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM post_mentions`).Scan(&mentionRows); err != nil {
		t.Fatalf("count post_mentions: %v", err)
	}
	if mentionRows == 0 {
		t.Error("no post_mentions rows synced")
	}
}

func TestApplyPollsAndEngagement(t *testing.T) {
	st, ds, _ := seedEngine(t)
	ctx := context.Background()

	// Polls exist for every post that carried one, with votes.
	var polls, votes int
	if err := st.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM polls`).Scan(&polls); err != nil {
		t.Fatalf("count polls: %v", err)
	}
	if polls != len(ds.Polls) {
		t.Errorf("polls = %d, want %d", polls, len(ds.Polls))
	}
	if err := st.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM poll_votes`).Scan(&votes); err != nil {
		t.Fatalf("count poll_votes: %v", err)
	}
	if votes != len(ds.PollVotes) {
		t.Errorf("poll_votes = %d, want %d", votes, len(ds.PollVotes))
	}

	// Engagement rows match the dataset counts.
	check := func(table string, expected int) {
		t.Helper()
		var got int
		if err := st.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM `+table).Scan(&got); err != nil {
			t.Fatalf("count %s: %v", table, err)
		}
		if got != expected {
			t.Errorf("%s = %d, want %d", table, got, expected)
		}
	}
	check("post_likes", len(ds.Likes))
	check("post_reposts", len(ds.Reposts))
	check("post_bookmarks", len(ds.Bookmarks))
}

func TestApplyRelationships(t *testing.T) {
	st, ds, _ := seedEngine(t)
	ctx := context.Background()

	for _, r := range ds.Relationships {
		exists, err := st.UserRelationships.Exists(ctx, ds.UserIDs[r.FollowerIdx], ds.UserIDs[r.FollowingIdx], r.Type)
		if err != nil {
			t.Fatalf("Exists(%v): %v", r, err)
		}
		if !exists {
			t.Errorf("relationship %v missing after Apply", r)
		}
	}
}

func TestApplyDMsListsBadges(t *testing.T) {
	st, ds, _ := seedEngine(t)
	ctx := context.Background()

	// Conversations + messages.
	var convs int
	if err := st.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM conversations`).Scan(&convs); err != nil {
		t.Fatalf("count conversations: %v", err)
	}
	if convs != len(ds.DMConversations) {
		t.Errorf("conversations = %d, want %d", convs, len(ds.DMConversations))
	}
	var dms int
	if err := st.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM messages`).Scan(&dms); err != nil {
		t.Fatalf("count messages: %v", err)
	}
	if dms == 0 {
		t.Error("no messages seeded")
	}

	// Lists + members.
	var lists, members int
	if err := st.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM lists`).Scan(&lists); err != nil {
		t.Fatalf("count lists: %v", err)
	}
	if lists != len(ds.Lists) {
		t.Errorf("lists = %d, want %d", lists, len(ds.Lists))
	}
	if err := st.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM list_members`).Scan(&members); err != nil {
		t.Fatalf("count list_members: %v", err)
	}
	if members == 0 {
		t.Error("no list members seeded")
	}

	// Assigned badges catalog + grants.
	var badges int
	if err := st.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM badges WHERE kind = 'assigned'`).Scan(&badges); err != nil {
		t.Fatalf("count badges: %v", err)
	}
	if badges != len(ds.Badges) {
		t.Errorf("assigned badges = %d, want %d", badges, len(ds.Badges))
	}
	for _, b := range ds.Badges {
		userID := ds.UserIDs[b.RecipientIdx]
		got, err := st.Badges.GetBadgesForUsers(ctx, []int{userID})
		if err != nil {
			t.Fatalf("GetBadgesForUsers: %v", err)
		}
		if len(got[userID]) == 0 {
			t.Errorf("user %d holds no badges (want %s)", b.RecipientIdx, b.Key)
		}
	}
}

func TestApplyMedia(t *testing.T) {
	st, ds, mediaDir := seedEngine(t)
	ctx := context.Background()

	// All users have an avatar media row + file.
	for i, u := range ds.Users {
		if u.ProfilePictureUUID == uuid.Nil {
			t.Fatalf("user %d missing avatar uuid", i)
		}
		if _, err := st.Media.GetByID(ctx, u.ProfilePictureUUID); err != nil {
			t.Fatalf("avatar media row for user %d: %v", i, err)
		}
	}

	// Post media linked.
	var links int
	if err := st.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM post_media`).Scan(&links); err != nil {
		t.Fatalf("count post_media: %v", err)
	}
	if links != len(ds.Media) {
		t.Errorf("post_media links = %d, want %d", links, len(ds.Media))
	}

	// Media files exist on disk.
	files, err := os.ReadDir(mediaDir)
	if err != nil {
		t.Fatalf("read media dir: %v", err)
	}
	if len(files) < len(ds.Users)+len(ds.Media) {
		t.Errorf("media files on disk = %d, want >= %d", len(files), len(ds.Users)+len(ds.Media))
	}
	if _, err := os.Stat(filepath.Join(mediaDir, ds.Users[0].ProfilePictureUUID.String())); err != nil {
		t.Errorf("avatar file missing: %v", err)
	}
}
