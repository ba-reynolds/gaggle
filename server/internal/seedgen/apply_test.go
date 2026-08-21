package seedgen

import (
	"context"
	"fmt"
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
		t.Fatalf("ds.UserIDs = %d, want TotalUsers=%d", len(ds.UserIDs), TotalUsers)
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
	wantPrivates := 4
	if isCI() {
		wantPrivates = 3
	}
	if privates != wantPrivates {
		t.Errorf("private users = %d, want %d", privates, wantPrivates)
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
	if diff := maxTS.Sub(minTS); diff > time.Duration(DaysOfHistory)*24*time.Hour {
		t.Errorf("post history spans %v, want <= %d days", diff, DaysOfHistory)
	}

	// Top-level count matches scale.
	var topLevel int
	var zeroParent int
	if err := st.DB.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM posts WHERE parent_id IS NULL`).Scan(&topLevel); err != nil {
		t.Fatalf("count top-level: %v", err)
	}
	if topLevel != TopLevelPosts {
		t.Errorf("top-level posts = %d, want TopLevelPosts=%d", topLevel, TopLevelPosts)
	}
	if err := st.DB.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM posts WHERE parent_id IS NOT NULL`).Scan(&zeroParent); err != nil {
		t.Fatalf("count replies: %v", err)
	}
	if zeroParent != ReplyPosts {
		t.Errorf("replies = %d, want ReplyPosts=%d", zeroParent, ReplyPosts)
	}
	if zeroParent != ReplyPosts {
		t.Errorf("replies = %d, want ReplyPosts=%d", zeroParent, ReplyPosts)
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

	// Build the set of pairs soft-block removes: a block on (A,B) unfollows
	// both A->B and B->A (Twitter soft-block semantics).
	softRemoved := make(map[string]bool)
	for _, r := range ds.Relationships {
		if r.Type != "block" {
			continue
		}
		a := ds.UserIDs[r.FollowerIdx]
		b := ds.UserIDs[r.FollowingIdx]
		softRemoved[fmt.Sprintf("%d:%d", a, b)] = true
		softRemoved[fmt.Sprintf("%d:%d", b, a)] = true
	}

	for _, r := range ds.Relationships {
		// Blocks always land. Follows land unless soft-block removed them.
		if r.Type == "follow" {
			a := ds.UserIDs[r.FollowerIdx]
			b := ds.UserIDs[r.FollowingIdx]
			if softRemoved[fmt.Sprintf("%d:%d", a, b)] {
				continue
			}
		}
		exists, err := st.UserRelationships.Exists(ctx, ds.UserIDs[r.FollowerIdx], ds.UserIDs[r.FollowingIdx], r.Type)
		if err != nil {
			t.Fatalf("Exists(%v): %v", r, err)
		}
		if !exists {
			t.Errorf("relationship %v missing after Apply", r)
		}
	}

	// Invariant: no pair may have a follow where a block exists in either
	// direction — blocking must have soft-unfollowed them.
	var followWithBlock int
	if err := st.DB.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM user_relationships follow
		WHERE follow.relationship_type = 'follow'
		  AND EXISTS (
			SELECT 1 FROM user_relationships blk
			WHERE blk.relationship_type = 'block'
			  AND blk.follower_id IN (follow.follower_id, follow.following_id)
			  AND blk.following_id IN (follow.follower_id, follow.following_id)
			  AND blk.follower_id <> blk.following_id
		  )`).Scan(&followWithBlock); err != nil {
		t.Fatalf("check follow-with-block invariant: %v", err)
	}
	if followWithBlock != 0 {
		t.Errorf("found %d follow rows coexisting with a block; soft-block should have removed them", followWithBlock)
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

func TestApplyBookmarkCategoriesUsedOnly(t *testing.T) {
	st, _, _ := seedEngine(t)
	ctx := context.Background()

	var zeroCount int
	if err := st.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM bookmark_categories c WHERE NOT EXISTS (SELECT 1 FROM post_bookmarks b WHERE b.user_id=c.user_id AND b.category_id=c.category_id)`).Scan(&zeroCount); err != nil {
		t.Fatalf("count empty categories: %v", err)
	}
	if zeroCount != 0 {
		t.Errorf("found %d empty bookmark categories; only-if-used violated", zeroCount)
	}
	var uncategorized, total int
	if err := st.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM post_bookmarks WHERE category_id IS NULL`).Scan(&uncategorized); err != nil {
		t.Fatalf("count uncategorized: %v", err)
	}
	if err := st.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM post_bookmarks`).Scan(&total); err != nil {
		t.Fatalf("count total bookmarks: %v", err)
	}
	if total == 0 {
		t.Fatal("no bookmarks after Apply")
	}
	ratio := float64(uncategorized) / float64(total)
	if ratio < 0.05 || ratio > 0.15 {
		t.Errorf("uncategorized ratio %.2f outside 5-15%% (uncategorized=%d total=%d)", ratio, uncategorized, total)
	}
}

func TestApplyBookmarkedFeedUncategorized(t *testing.T) {
	st, ds, _ := seedEngine(t)
	ctx := context.Background()

	aliceID := ds.UserIDs[0]

	// Uncategorized-only feed.
	uncatFeed, err := st.Posts.GetBookmarkedPostsFeed(ctx, aliceID, nil, true, 100, "")
	if err != nil {
		t.Fatalf("GetBookmarkedPostsFeed uncategorized: %v", err)
	}
	if len(uncatFeed.Items) == 0 {
		t.Fatal("expected at least one uncategorized bookmark for alice")
	}
	// Every returned post must map to a bookmark with category_id IS NULL.
	for _, item := range uncatFeed.Items {
		var catID *int
		if err := st.DB.QueryRowContext(ctx, `SELECT category_id FROM post_bookmarks WHERE user_id=$1 AND post_id=$2`, aliceID, item.Post.ID).Scan(&catID); err != nil {
			t.Fatalf("lookup bookmark for post %d: %v", item.Post.ID, err)
		}
		if catID != nil {
			t.Errorf("uncategorized feed returned post %d with category_id %d", item.Post.ID, *catID)
		}
	}
	// Count from feed should match DB uncategorized count for alice.
	var dbUncat int
	if err := st.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM post_bookmarks WHERE user_id=$1 AND category_id IS NULL`, aliceID).Scan(&dbUncat); err != nil {
		t.Fatalf("count db uncategorized: %v", err)
	}
	// Feed is paginated; just sanity-check it does not return categorized rows.
	// Union: one alice category plus uncategorized.
	var catID int
	var catName string
	if err := st.DB.QueryRowContext(ctx, `SELECT category_id, category_name FROM bookmark_categories WHERE user_id=$1 LIMIT 1`, aliceID).Scan(&catID, &catName); err != nil {
		t.Fatalf("lookup alice category: %v", err)
	}
	unionFeed, err := st.Posts.GetBookmarkedPostsFeed(ctx, aliceID, []int{catID}, true, 200, "")
	if err != nil {
		t.Fatalf("GetBookmarkedPostsFeed union: %v", err)
	}
	if len(unionFeed.Items) == 0 {
		t.Fatal("union feed returned 0 items")
	}
	// Union must contain at least one uncategorized and at least one categorized.
	var hasUncat, hasCat bool
	for _, item := range unionFeed.Items {
		var cid *int
		if err := st.DB.QueryRowContext(ctx, `SELECT category_id FROM post_bookmarks WHERE user_id=$1 AND post_id=$2`, aliceID, item.Post.ID).Scan(&cid); err != nil {
			t.Fatalf("lookup bookmark for union post %d: %v", item.Post.ID, err)
		}
		if cid == nil {
			hasUncat = true
		} else if *cid == catID {
			hasCat = true
		} else {
			t.Errorf("union feed returned post %d with unexpected category_id %d (want %d or NULL)", item.Post.ID, *cid, catID)
		}
	}
	if !hasUncat {
		t.Errorf("union feed has no uncategorized items (category %q alone would show none)", catName)
	}
	if !hasCat {
		t.Errorf("union feed has no items from category %q", catName)
	}
	// Category-only feed must not include uncategorized.
	catOnly, err := st.Posts.GetBookmarkedPostsFeed(ctx, aliceID, []int{catID}, false, 200, "")
	if err != nil {
		t.Fatalf("GetBookmarkedPostsFeed cat-only: %v", err)
	}
	for _, item := range catOnly.Items {
		var cid *int
		if err := st.DB.QueryRowContext(ctx, `SELECT category_id FROM post_bookmarks WHERE user_id=$1 AND post_id=$2`, aliceID, item.Post.ID).Scan(&cid); err != nil {
			t.Fatalf("lookup bookmark for cat-only post %d: %v", item.Post.ID, err)
		}
		if cid == nil {
			t.Errorf("cat-only feed returned uncategorized post %d", item.Post.ID)
		} else if *cid != catID {
			t.Errorf("cat-only feed returned post %d with wrong category %d want %d", item.Post.ID, *cid, catID)
		}
	}
}
