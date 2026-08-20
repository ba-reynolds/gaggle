package seedgen

import (
	"reflect"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/brianvoe/gofakeit/v7"
)

func fixedNow() time.Time {
	return time.Date(2026, 8, 20, 12, 0, 0, 0, time.UTC)
}

// gDataset builds a dataset from the standard fixed seed for assertions.
func gDataset() *Dataset {
	return Generate(gofakeit.New(SeedValue), fixedNow())
}

func TestGenerateDeterminism(t *testing.T) {
	a := Generate(gofakeit.New(SeedValue), fixedNow())
	b := Generate(gofakeit.New(SeedValue), fixedNow())
	if !reflect.DeepEqual(a, b) {
		t.Fatalf("Generate is not deterministic for the same seed+now")
	}
}

func TestGenerateScale(t *testing.T) {
	ds := gDataset()
	if len(ds.Users) != 150 {
		t.Errorf("users = %d, want 150", len(ds.Users))
	}
	if len(ds.Users) != TotalUsers {
		t.Errorf("users = %d, want TotalUsers=%d", len(ds.Users), TotalUsers)
	}
	var top int
	for _, p := range ds.Posts {
		if p.ParentIdx == -1 {
			top++
		}
	}
	if top != 1600 {
		t.Errorf("top-level = %d, want 1600", top)
	}
	if len(ds.Posts) != 2200 {
		t.Errorf("posts = %d, want 2200 (1600+600)", len(ds.Posts))
	}
	if len(ds.Posts) != TopLevelPosts+ReplyPosts {
		t.Errorf("posts = %d, want TopLevelPosts+ReplyPosts=%d", len(ds.Posts), TopLevelPosts+ReplyPosts)
	}
	if len(ds.Likes) == 0 || len(ds.Reposts) == 0 || len(ds.Bookmarks) == 0 {
		t.Errorf("expected engagement rows, got likes=%d reposts=%d bookmarks=%d",
			len(ds.Likes), len(ds.Reposts), len(ds.Bookmarks))
	}
	if len(ds.Relationships) == 0 {
		t.Error("expected relationships")
	}
	// Lists/DMs/Media also at 4x scale
	if len(ds.DMConversations) != DMConversations {
		t.Errorf("dm conversations = %d, want %d", len(ds.DMConversations), DMConversations)
	}
	if len(ds.Lists) != Lists {
		t.Errorf("lists = %d, want %d", len(ds.Lists), Lists)
	}
	if len(ds.Media) != MediaPosts {
		t.Errorf("media = %d, want %d", len(ds.Media), MediaPosts)
	}
}

func TestGenerateUserUniquenessAndConstraints(t *testing.T) {
	ds := gDataset()
	seen := make(map[string]bool, len(ds.Users))
	for i, u := range ds.Users {
		if u.Username == "" || utf8.RuneCountInString(u.Username) > 16 {
			t.Errorf("user %d username %q violates length/empty constraint", i, u.Username)
		}
		if seen[u.Username] {
			t.Errorf("duplicate username %q", u.Username)
		}
		seen[u.Username] = true
		if strings.ToLower(u.Email) == "" {
			t.Errorf("user %d missing email", i)
		}
		if u.Password == "" {
			t.Errorf("user %d missing password", i)
		}
	}
	// alice must be first (guard anchor) and admin.
	if ds.Users[0].Username != "alice" || !ds.Users[0].IsAdmin {
		t.Errorf("alice must be the first anchor user and admin")
	}
}

func TestGeneratePostConstraints(t *testing.T) {
	ds := gDataset()
	for i, p := range ds.Posts {
		if utf8.RuneCountInString(p.Content) > 280 {
			t.Errorf("post %d content > 280 runes", i)
		}
		if p.AuthorIdx < 0 || p.AuthorIdx >= len(ds.Users) {
			t.Fatalf("post %d invalid author idx %d", i, p.AuthorIdx)
		}
		if p.ParentIdx >= i {
			t.Errorf("post %d parent idx %d must reference an earlier post", i, p.ParentIdx)
		}
		if (p.ParentIdx == -1) != isTopLevel(p) {
			t.Errorf("post %d parent/visibility mismatch", i)
		}
		// Posts must be within the 28-day history window (allow small slack).
		age := fixedNow().Sub(p.CreatedAt)
		if age < 0 || age > (DaysOfHistory*24*time.Hour)+time.Minute {
			t.Errorf("post %d created_at %v outside 28-day window", i, p.CreatedAt)
		}
	}
	// Top-level count matches the target exactly.
	var topLevel int
	for _, p := range ds.Posts {
		if p.ParentIdx == -1 {
			topLevel++
		}
	}
	if topLevel != TopLevelPosts {
		t.Errorf("top-level posts = %d, want %d", topLevel, TopLevelPosts)
	}
	// Some posts carry hashtags / mentions in their content.
	var hasHashtag, hasMention bool
	for _, p := range ds.Posts {
		if strings.Contains(p.Content, "#") {
			hasHashtag = true
		}
		if strings.Contains(p.Content, "@") {
			hasMention = true
		}
	}
	if !hasHashtag {
		t.Error("no hashtag-bearing posts generated (wanted ~40%)")
	}
	if !hasMention {
		t.Error("no mentioning posts generated (wanted ~15%)")
	}
}

func isTopLevel(p GenPost) bool { return p.ParentIdx == -1 }

func TestGenerateRelationships(t *testing.T) {
	ds := gDataset()
	var follows, blocks, mutes int
	for _, r := range ds.Relationships {
		if r.FollowerIdx == r.FollowingIdx {
			t.Errorf("self-relationship follower=%d following=%d", r.FollowerIdx, r.FollowingIdx)
		}
		switch r.Type {
		case "follow":
			follows++
		case "block":
			blocks++
		case "mute":
			mutes++
		default:
			t.Errorf("unexpected relationship type %q", r.Type)
		}
	}
	if follows < TotalUsers*FollowMin {
		t.Errorf("follows = %d, want >= %d", follows, TotalUsers*FollowMin)
	}
	if blocks < 2 || mutes < 2 {
		t.Errorf("expected >=2 blocks and >=2 mutes, got %d/%d", blocks, mutes)
	}
}

func TestGenerateMediaAndBadges(t *testing.T) {
	ds := gDataset()
	if len(ds.Media) < MediaPosts {
		t.Errorf("media specs = %d, want >= %d", len(ds.Media), MediaPosts)
	}
	// Every post media spec must map to a post index within range.
	for i, m := range ds.Media {
		if m.PostIdx >= 0 && m.PostIdx >= len(ds.Posts) {
			t.Errorf("media %d maps to invalid post idx %d", i, m.PostIdx)
		}
	}
	if len(ds.Badges) != AssignedBadges {
		t.Errorf("badges = %d, want %d", len(ds.Badges), AssignedBadges)
	}
	if len(ds.Polls) == 0 || len(ds.PollVotes) == 0 {
		t.Errorf("expected polls and poll votes, got polls=%d votes=%d",
			len(ds.Polls), len(ds.PollVotes))
	}
	for i, v := range ds.PollVotes {
		if v.PostIdx >= TopLevelPosts {
			t.Errorf("poll vote %d references non-top-level post idx %d", i, v.PostIdx)
		}
		poll := ds.Posts[v.PostIdx]
		if poll.PollIdx < 0 || poll.PollIdx >= len(ds.Polls) {
			t.Errorf("poll vote %d references post without a valid poll", i)
		}
	}

	// Private users exist and are distinct.
	var privates int
	for _, u := range ds.Users {
		if u.IsPrivate {
			privates++
		}
	}
	if privates < 3 {
		t.Errorf("private users = %d, want >= 3", privates)
	}
}

func TestGenerateDMAndLists(t *testing.T) {
	ds := gDataset()
	if len(ds.DMConversations) != DMConversations {
		t.Errorf("dm conversations = %d, want %d", len(ds.DMConversations), DMConversations)
	}
	if len(ds.DMConversations) < 40 {
		t.Errorf("dm conversations = %d, want >= 40", len(ds.DMConversations))
	}
	for i, c := range ds.DMConversations {
		if c.UserAIdx == c.UserBIdx {
			t.Errorf("dm %d self-conversation", i)
		}
		if c.UserAIdx < 0 || c.UserAIdx >= len(ds.Users) || c.UserBIdx < 0 || c.UserBIdx >= len(ds.Users) {
			t.Errorf("dm %d invalid participant indices", i)
		}
		for _, m := range c.Messages {
			if m.SenderIdx != c.UserAIdx && m.SenderIdx != c.UserBIdx {
				t.Errorf("dm %d sender %d is not a participant", i, m.SenderIdx)
			}
			if utf8.RuneCountInString(m.Body) > 2000 {
				t.Errorf("dm %d message exceeds 2000 runes", i)
			}
		}
	}
	if len(ds.Lists) != Lists {
		t.Errorf("lists = %d, want %d", len(ds.Lists), Lists)
	}
	if len(ds.Lists) < 24 {
		t.Errorf("lists = %d, want >= 24", len(ds.Lists))
	}
	for i, l := range ds.Lists {
		for _, m := range l.MemberIdxs {
			if m == l.OwnerIdx {
				t.Errorf("list %d contains its owner as a member", i)
			}
			if m < 0 || m >= len(ds.Users) {
				t.Errorf("list %d invalid member idx %d", i, m)
			}
		}
	}
	if len(ds.Media) != MediaPosts {
		t.Errorf("media = %d, want %d", len(ds.Media), MediaPosts)
	}
	if len(ds.Media) < 60 {
		t.Errorf("media = %d, want >= 60", len(ds.Media))
	}
}

func TestGenerateBookmarkCategoriesUsedOnly(t *testing.T) {
	ds := gDataset()
	// Every retained category name must appear in bookmarks for that user.
	for uIdx, names := range ds.BookmarkCategoryNames {
		if len(names) == 0 {
			continue
		}
		countByName := make(map[string]int)
		for _, bm := range ds.Bookmarks {
			if bm.UserIdx == uIdx && bm.CategoryName != nil {
				countByName[*bm.CategoryName]++
			}
		}
		for _, name := range names {
			if countByName[name] == 0 {
				t.Errorf("user %d category %q retained but has 0 bookmarks (only-if-used violated)", uIdx, name)
			}
		}
		// No bookmark should reference a category name not in the retained list.
		allowed := make(map[string]bool, len(names))
		for _, n := range names {
			allowed[n] = true
		}
		for _, bm := range ds.Bookmarks {
			if bm.UserIdx == uIdx && bm.CategoryName != nil && !allowed[*bm.CategoryName] {
				t.Errorf("user %d bookmark references pruned category %q", uIdx, *bm.CategoryName)
			}
		}
	}
	// Users with no bookmarks must have no categories.
	for uIdx := range ds.Users {
		hasBM := false
		for _, bm := range ds.Bookmarks {
			if bm.UserIdx == uIdx {
				hasBM = true
				break
			}
		}
		if !hasBM && len(ds.BookmarkCategoryNames[uIdx]) != 0 {
			t.Errorf("user %d has no bookmarks but retained %d categories", uIdx, len(ds.BookmarkCategoryNames[uIdx]))
		}
	}
	// Uncategorized ratio ~10% (allow 5-15%).
	if len(ds.Bookmarks) == 0 {
		t.Fatal("no bookmarks generated")
	}
	var uncategorized int
	for _, bm := range ds.Bookmarks {
		if bm.CategoryName == nil {
			uncategorized++
		}
	}
	ratio := float64(uncategorized) / float64(len(ds.Bookmarks))
	if ratio < 0.05 || ratio > 0.15 {
		t.Errorf("uncategorized ratio %.2f outside 5-15%% (uncategorized=%d total=%d)", ratio, uncategorized, len(ds.Bookmarks))
	}
}
