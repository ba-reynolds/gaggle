// Package seedgen generates and applies a realistic, deterministic demo
// dataset (users, posts, engagement, DMs, lists, badges, media) for the
// Gaggle social network. It is the single implementation of the seed
// strategy: the bulk loader (cmd/seed) and the future live-user simulator
// (cmd/simulate) both build on Generate/Apply and the Tick seam.
//
// Design constraints:
//   - Generate is PURE: it returns an in-memory Dataset with NO database
//     writes. It is deterministic for a fixed gofakeit seed + "now".
//   - Apply writes the Dataset through the store layer, honoring the
//     service-layer invariants that matter (hashtag/mention sync).
//   - Re-running Apply is safe: cmd/seed guards on alice@example.com
//     existing, and individual writes are idempotent where the store allows.
package seedgen

import (
	"os"
	"time"

	"github.com/brianvoe/gofakeit/v7"
	"github.com/google/uuid"
)

// Scale targets for a "busy but not pathological" community.
//
// Full 4x scale is used for production/demo; CI sets SEED_SCALE=ci for 2x.
const SeedValue = 20260819 // fixed rng seed; determinism across envs

func isCI() bool { return os.Getenv("CI") == "true" || os.Getenv("SEED_SCALE") == "ci" }

var (
	AnchorUsers     = 8 // alice/bob/charlie/diana/eve/frank/grace/henry
	FakerUsers      = func() int { if isCI() { return 72 }; return 142 }()
	TotalUsers      = AnchorUsers + FakerUsers
	DaysOfHistory   = 28   // posts spread across the last 4 weeks
	TopLevelPosts   = func() int { if isCI() { return 800 }; return 1600 }()
	ReplyPosts      = func() int { if isCI() { return 300 }; return 600 }()
	DMConversations = func() int { if isCI() { return 20 }; return 40 }()
	Lists           = func() int { if isCI() { return 12 }; return 24 }()
	AssignedBadges  = 3    // assigned badges granted to a few users
	MediaPosts      = func() int { if isCI() { return 30 }; return 60 }()

	FollowMin = 8 // per-user following count bounds
	FollowMax = 15

	LikeMin, LikeMax         = 0, 40
	RepostMin, RepostMax     = 0, 10
	BookmarkMin, BookmarkMax = 0, 16
	PollVoteMin, PollVoteMax = 0, 12
)

// GenUser is a to-be-created user. Password holds the PLAINTEXT demo password;
// Apply bcrypt-hashes it before insert (all seed accounts share "password123").
type GenUser struct {
	Username           string
	Email              string
	Password           string
	IsAdmin            bool
	IsPrivate          bool
	DisplayName        string
	Bio                string
	Location           string
	Website            string
	BirthDate          time.Time
	ProfilePictureUUID uuid.UUID
	BannerUUID         uuid.UUID
}

// GenPost is a to-be-created post. Cross references are by index into the
// dataset's slices; -1 means "none". Hashtags/mentions are extracted from
// Content at Apply time.
type GenPost struct {
	AuthorIdx  int
	ParentIdx  int // -1 = top-level
	QuotedIdx  int // -1 = not a quote
	Content    string
	Visibility string // "public" | "followers"
	CreatedAt  time.Time
	MediaIdx   int // -1 = no attached media
	PollIdx    int // -1 = no poll
}

// GenPoll is a poll attached to one post. Votes reference that poll's post.
type GenPoll struct {
	Options []string
	EndsAt  *time.Time
}

// GenPollVote records one vote on a poll (referenced by its post's index).
type GenPollVote struct {
	PostIdx   int
	VoterIdx  int
	OptionIdx int
}

// bookmarkCategoryPool is the shared palette for bookmark categories.
var bookmarkCategoryPool = []struct {
	Name  string
	Color string
}{
	{"Tech", "#0ea5e9"},
	{"Inspiration", "#a855f7"},
	{"Reading List", "#f59e0b"},
	{"Research", "#06b6d4"},
	{"Work", "#6366f1"},
	{"Ideas", "#ec4899"},
	{"Watch Later", "#22c55e"},
}

// GenEngagement is a single like/repost/bookmark of a post by a user.
type GenEngagement struct {
	PostIdx      int
	UserIdx      int
	CategoryName *string // nil = uncategorized (~10% of bookmarks)
}

// GenRelationship is one user_relationships row (follow | block | mute).
type GenRelationship struct {
	FollowerIdx  int
	FollowingIdx int
	Type         string
}

// GenDMMessage is one message in a conversation. SenderIdx is one of the two
// conversation participants.
type GenDMMessage struct {
	SenderIdx int
	Body      string
}

// GenDMConversation is a two-party conversation with ordered messages.
type GenDMConversation struct {
	UserAIdx int
	UserBIdx int
	Messages []GenDMMessage
}

// GenList is a user-created list with members (owner excluded).
type GenList struct {
	OwnerIdx    int
	Name        string
	Description string
	MemberIdxs  []int
}

// GenBadgeGrant is one ASSIGNED badge creation plus a grant to a recipient.
type GenBadgeGrant struct {
	Key          string
	Label        string
	Description  string
	Icon         string
	RecipientIdx int
}

// GenMedia describes a placeholder image to persist. PostIdx >= 0 attaches it
// to that post; PostIdx == -1 is a standalone row referenced by a profile
// (avatar/banner) via the user's UUID fields.
type GenMedia struct {
	UUID     uuid.UUID
	MimeType string
	Filename string
	Width    int
	Height   int
	PostIdx  int
	Position int
	AltText  string
}

// Dataset is the pure in-memory model of the demo community. All cross
// references use dataset-relative indices; Apply resolves them to DB IDs.
type Dataset struct {
	Users                 []GenUser
	Posts                 []GenPost
	Polls                 []GenPoll
	PollVotes             []GenPollVote
	Likes                 []GenEngagement
	Reposts               []GenEngagement
	Bookmarks             []GenEngagement
	BookmarkCategoryNames [][]string `json:"-"` // per-user pool pick (pruned to used only before Apply)
	Relationships         []GenRelationship
	DMConversations       []GenDMConversation
	Lists                 []GenList
	Badges                []GenBadgeGrant
	Media                 []GenMedia

	// UserIDs / PostIDs are populated by Apply: dataset index -> DB row ID.
	UserIDs []int
	PostIDs []int
}

// generator carries the deterministic faker + dataset under construction.
type generator struct {
	f             *gofakeit.Faker
	now           time.Time
	ds            *Dataset
	usedUsernames map[string]bool
	usedEmails    map[string]bool
}

// Generate returns a deterministic dataset describing a busy community.
// Given the same seed and now it returns an identical structure.
func Generate(f *gofakeit.Faker, now time.Time) *Dataset {
	g := &generator{
		f:             f,
		now:           now,
		ds:            &Dataset{},
		usedUsernames: make(map[string]bool),
		usedEmails:    make(map[string]bool),
	}
	g.genUsers()
	g.genBookmarkCategories()
	g.genMedia()
	g.genPosts()
	g.genRelationships()
	g.genEngagement()
	g.genDMs()
	g.genLists()
	g.genBadges()
	return g.ds
}
