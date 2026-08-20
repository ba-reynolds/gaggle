package seedgen

import (
	"fmt"
	"strings"
	"time"

	"github.com/brianvoe/gofakeit/v7"
	"github.com/google/uuid"
)

// Anchor users — stable, deterministic, used for login/testing. Kept in sync
// with the old cmd/seed anchors so existing demos never break.
var anchorUsers = []GenUser{
	{
		Username: "alice", Email: "alice@example.com", DisplayName: "Alice Johnson",
		Bio: "Software engineer and coffee enthusiast ☕", Location: "San Francisco, CA",
		Website: "https://alice.dev", IsAdmin: true,
	},
	{
		Username: "bob", Email: "bob@example.com", DisplayName: "Bob Smith",
		Bio: "Digital artist creating amazing visuals 🎨", Location: "New York, NY",
		Website: "https://bobart.com",
	},
	{
		Username: "charlie", Email: "charlie@example.com", DisplayName: "Charlie Brown",
		Bio: "Travel blogger sharing adventures around the world 🌍", Location: "London, UK",
		Website: "https://charlie-travels.com",
	},
	{
		Username: "diana", Email: "diana@example.com", DisplayName: "Diana Prince",
		Bio: "Fitness coach helping people reach their goals 💪", Location: "Tokyo, Japan",
		Website: "https://diana-fitness.com",
	},
	{
		Username: "eve", Email: "eve@example.com", DisplayName: "Eve Wilson",
		Bio: "Food lover and amateur chef 👨‍🍳", Location: "Paris, France",
		Website: "https://eve-cooks.com", IsPrivate: true,
	},
	{
		Username: "frank", Email: "frank@example.com", DisplayName: "Frank Miller",
		Bio: "Music producer and DJ 🎵", Location: "Sydney, Australia",
		Website: "https://frank-music.com",
	},
	{
		Username: "grace", Email: "grace@example.com", DisplayName: "Grace Kelly",
		Bio: "Photographer capturing life's beautiful moments 📸", Location: "Toronto, Canada",
		Website: "https://grace-photos.com",
	},
	{
		Username: "henry", Email: "henry@example.com", DisplayName: "Henry Ford",
		Bio: "Bookworm and literary critic 📚", Location: "Berlin, Germany",
		Website: "https://henry-books.com",
	},
}

// Hashtag vocabulary for seeding rich tag feeds (/trends). Fixed pool keeps
// the generated universe coherent (a few well-worn tags across many posts).
var hashtagPool = []string{
	"programming", "golang", "devlife", "webdev", "opensource",
	"design", "photography", "travel", "foodie", "fitness",
	"music", "art", "books", "coffee", "startup", "nature",
}

// Post topic seeds — short factual openers that read like real micro-posts
// once faker detail is appended.
var postTopics = []string{
	"Just shipped", "Finally got", "Hot take:",
	"Today I learned", "Weekend project:", "Reminder that",
	"Unpopular opinion:", "Small win today:", "Long time coming:",
	"Experimenting with", "Deep dive into", "Quick thread on",
	"Milestone unlocked:", "One thing I wish I knew earlier:",
	"Production incident post-mortem:", "Shoutout to",
}

// Reply openers for replies.
var replyOpeners = []string{
	"Great point —", "Totally agree.", "Interesting;", "Hadn't thought of that.",
	"+1.", "Thanks for sharing.", "This is the way.", "Couldn't have said it better.",
}

// cleanUsername sanitizes a raw faker username to the DB's constraints:
// [a-z0-9_], <= 16 chars, non-empty, unique across the dataset.
func cleanUsername(raw string, g *generator) string {
	s := sanitizeForUsername(raw)
	if s == "" || len([]rune(s)) > 16 {
		s = sanitizeForUsername(g.f.Username())
	}
	base := s
	for i := 1; g.usedUsernames[s]; i++ {
		suffix := fmt.Sprintf("%d", i)
		maxLen := 16 - len(suffix)
		runes := []rune(base)
		if len(runes) > maxLen {
			runes = runes[:maxLen]
		}
		s = string(runes) + suffix
	}
	g.usedUsernames[s] = true
	return s
}

// sanitizeForUsername keeps only [a-z0-9_] characters, lowercased.
func sanitizeForUsername(raw string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(raw) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// uniqueEmail returns a case-insensitively unique email address.
func uniqueEmail(g *generator) string {
	for {
		e := g.f.Email()
		key := strings.ToLower(e)
		if !g.usedEmails[key] {
			g.usedEmails[key] = true
			return e
		}
	}
}

// genUsers seeds anchors + faker users.
func (g *generator) genUsers() {
	g.usedUsernames = make(map[string]bool, TotalUsers)
	g.usedEmails = make(map[string]bool, TotalUsers)

	for _, a := range anchorUsers {
		a.Password = "password123"
		g.usedUsernames[a.Username] = true
		g.usedEmails[strings.ToLower(a.Email)] = true
		g.ds.Users = append(g.ds.Users, a)
	}

	for i := 0; i < FakerUsers; i++ {
		user := GenUser{
			Username:    cleanUsername(g.f.Username(), g),
			Email:       uniqueEmail(g),
			Password:    "password123",
			DisplayName: strings.TrimSpace(g.f.FirstName() + " " + g.f.LastName()),
			Bio:         g.genBio(),
			Location:    fmt.Sprintf("%s, %s", g.f.City(), g.f.StateAbr()),
			Website:     "https://" + g.f.DomainName(),
			BirthDate:   g.f.DateRange(g.now.AddDate(-60, 0, 0), g.now.AddDate(-18, 0, 0)),
		}
		// Two faker users also private (plus eve above) = 3 total.
		if i == 7 || i == 19 {
			user.IsPrivate = true
		}
		g.ds.Users = append(g.ds.Users, user)
	}

	// Anchor avatars/banners + every faker user gets an avatar.
	for i := range g.ds.Users {
		g.ds.Users[i].ProfilePictureUUID = g.mediaUUID(fmt.Sprintf("avatar-%d", i))
		if i < 4 {
			g.ds.Users[i].BannerUUID = g.mediaUUID(fmt.Sprintf("banner-%d", i))
		}
	}
}

// genBio assembles a hobby-flavored one-liner bio.
func (g *generator) genBio() string {
	return g.clamp(fmt.Sprintf("%s. %s", g.f.Hobby(), g.f.HipsterSentence()))
}

// genMedia pre-generates placeholder image specs (profiles + a handful of
// posts). Actual PNG bytes are written at Apply time from the UUID so Apply is
// deterministic too.
func (g *generator) genMedia() {
	// Photo posts: attach a single image to the first MediaPosts posts
	// (all top-level, by the ordering guarantee in genPosts).
	for i := 0; i < MediaPosts; i++ {
		g.ds.Media = append(g.ds.Media, GenMedia{
			UUID:     g.mediaUUID(fmt.Sprintf("post-media-%d", i)),
			MimeType: "image/png",
			Filename: fmt.Sprintf("post_%d.png", i),
			Width:    800,
			Height:   600,
			PostIdx:  i, // index into Posts; genPosts assigns media before post index i
			Position: 1,
			AltText:  "",
		})
	}
}

// mediaUUID derives a deterministic MD5 UUID from a stable key so Apply is
// repeatable (no random ids leaking into the dataset shape).
func (g *generator) mediaUUID(key string) uuid.UUID {
	return uuid.NewMD5(uuid.NameSpaceOID, []byte(key))
}

// genPosts generates top-level posts then nested replies. Ordering guarantee:
// top-level posts occupy indices [0, TopLevelPosts); replies come after.
func (g *generator) genPosts() {
	// Top-level posts.
	for i := 0; i < TopLevelPosts; i++ {
		p := GenPost{
			AuthorIdx:  g.f.Number(0, TotalUsers-1),
			ParentIdx:  -1,
			QuotedIdx:  -1,
			Visibility: "public",
			CreatedAt:  g.now.Add(-time.Duration(g.f.Number(0, 28*24*60*60)) * time.Second),
			MediaIdx:   -1,
			PollIdx:    -1,
		}
		// Private accounts post followers-only content ~half the time.
		if g.ds.Users[p.AuthorIdx].IsPrivate && g.f.Number(1, 100) <= 50 {
			p.Visibility = "followers"
		}
		p.Content = g.genPostContent(i)

		// ~10% of top-level posts get a poll (uses this post's index).
		if g.f.Number(1, 100) <= 10 {
			p.PollIdx = g.genPoll(i, p.AuthorIdx)
		}
		// First MediaPosts get one image each.
		if i < MediaPosts {
			p.MediaIdx = i
		}
		g.ds.Posts = append(g.ds.Posts, p)
	}

	// Replies, mostly top-level-nested, occasionally nested under a reply.
	for i := 0; i < ReplyPosts; i++ {
		reply := GenPost{
			AuthorIdx:  g.f.Number(0, TotalUsers-1),
			ParentIdx:  g.f.Number(0, len(g.ds.Posts)-1),
			QuotedIdx:  -1,
			Content:    g.genReplyContent(),
			Visibility: "public",
			CreatedAt:  g.now.Add(-time.Duration(g.f.Number(1, 28*24*60*60)) * time.Second),
			MediaIdx:   -1,
			PollIdx:    -1,
		}
		// Deep-ish nesting ~1 in 7 replies.
		if i%7 == 3 && len(g.ds.Posts) > TopLevelPosts {
			reply.ParentIdx = len(g.ds.Posts) - 1
		}
		g.ds.Posts = append(g.ds.Posts, reply)
	}
}

// genPoll creates a poll for the given top-level post index and author.
// Returns the index of the new poll in ds.Polls.
func (g *generator) genPoll(postIdx, author int) int {
	opts := g.f.Number(2, 4)
	options := make([]string, opts)
	for i := range options {
		options[i] = g.f.HipsterWord() + " option " + string(rune('A'+i))
	}
	end := g.now.AddDate(0, 0, g.f.Number(1, 7))
	g.ds.Polls = append(g.ds.Polls, GenPoll{Options: options, EndsAt: &end})
	pollIdx := len(g.ds.Polls) - 1
	// Vote some users (exclude the author; unique voters).
	seen := make(map[int]bool)
	for v := 0; v < g.f.Number(PollVoteMin, PollVoteMax); v++ {
		voter := g.f.Number(0, TotalUsers-1)
		if voter == author || seen[voter] {
			continue
		}
		seen[voter] = true
		g.ds.PollVotes = append(g.ds.PollVotes, GenPollVote{
			PostIdx:   postIdx,
			VoterIdx:  voter,
			OptionIdx: g.f.Number(0, opts-1),
		})
	}
	return pollIdx
}

// genPostContent builds a top-level post body with hashtags and sometimes a
// mention, clamped to 280 runes (posts.content VARCHAR(280)). All hashtags and
// mentions live in the CONTENT string so Hashtags.SyncPost / Mentions.SyncPost
// parse them into the join tables at Apply time.
func (g *generator) genPostContent(i int) string {
	topic := postTopics[i%len(postTopics)]
	sent := g.f.Sentence(g.f.Number(6, 14))
	body := fmt.Sprintf("%s %s", topic, sent)

	// ~40% rich hashtags so /trends and hashtag feeds stay alive.
	if g.f.Number(1, 100) <= 40 {
		count := g.f.Number(1, 3)
		used := make(map[string]bool)
		var tags []string
		for len(tags) < count && len(tags) < len(hashtagPool) {
			tag := hashtagPool[g.f.Number(0, len(hashtagPool)-1)]
			if !used[tag] {
				used[tag] = true
				tags = append(tags, "#"+tag)
			}
		}
		body += " " + strings.Join(tags, " ")
	}

	// ~15% mention another user (resolved to a join row by Mentions.SyncPost).
	if g.f.Number(1, 100) <= 15 {
		body += " @" + g.ds.Users[g.f.Number(0, len(g.ds.Users)-1)].Username
	}

	return g.clamp(body)
}

// genReplyContent builds a short reply, pre-clamped.
func (g *generator) genReplyContent() string {
	o := replyOpeners[g.f.Number(0, len(replyOpeners)-1)]
	detail := g.f.HipsterSentence()
	return g.clamp(fmt.Sprintf("%s %s", o, detail))
}

// clamp truncates to 280 runes at a rune boundary.
func (g *generator) clamp(s string) string {
	runes := []rune(s)
	if len(runes) <= 280 {
		return s
	}
	return string(runes[:280])
}

// genRelationships — dense follow web + a couple of blocks/mutes.
func (g *generator) genRelationships() {
	for u := 0; u < TotalUsers; u++ {
		count := g.f.Number(FollowMin, FollowMax)
		for c := 0; c < count; c++ {
			target := g.f.Number(0, TotalUsers-1)
			if target == u {
				continue
			}
			g.ds.Relationships = append(g.ds.Relationships, GenRelationship{
				FollowerIdx:  u,
				FollowingIdx: target,
				Type:         "follow",
			})
		}
	}
	g.ds.Relationships = append(g.ds.Relationships,
		GenRelationship{FollowerIdx: 1, FollowingIdx: 7, Type: "block"},
		GenRelationship{FollowerIdx: 2, FollowingIdx: 5, Type: "block"},
		GenRelationship{FollowerIdx: 3, FollowingIdx: 9, Type: "mute"},
		GenRelationship{FollowerIdx: 4, FollowingIdx: 2, Type: "mute"},
	)
}

// genEngagement — likes/reposts/bookmarks per post (excluding the author).
// Each (post, user) pair is unique so the DB ON CONFLICT dedup never swallows
// rows and dataset counts match post-Apply row counts exactly.
func (g *generator) genEngagement() {
	seen := make(map[[3]int]bool)
	addEngagement := func(kind string, postIdx, userIdx int) {
		key := [3]int{kindIdx(kind), postIdx, userIdx}
		if seen[key] {
			return
		}
		seen[key] = true
		switch kind {
		case "like":
			g.ds.Likes = append(g.ds.Likes, GenEngagement{PostIdx: postIdx, UserIdx: userIdx})
		case "repost":
			g.ds.Reposts = append(g.ds.Reposts, GenEngagement{PostIdx: postIdx, UserIdx: userIdx})
		case "bookmark":
			g.ds.Bookmarks = append(g.ds.Bookmarks, GenEngagement{PostIdx: postIdx, UserIdx: userIdx})
		}
	}

	for i := range g.ds.Posts {
		p := &g.ds.Posts[i]
		for l := 0; l < g.f.Number(LikeMin, LikeMax); l++ {
			u := g.f.Number(0, TotalUsers-1)
			if u == p.AuthorIdx {
				continue
			}
			addEngagement("like", i, u)
		}
		// Reposts on top-level posts only (mirrors the app).
		if p.ParentIdx == -1 {
			for r := 0; r < g.f.Number(RepostMin, RepostMax); r++ {
				u := g.f.Number(0, TotalUsers-1)
				if u == p.AuthorIdx {
					continue
				}
				addEngagement("repost", i, u)
			}
		}
		for b := 0; b < g.f.Number(BookmarkMin, BookmarkMax); b++ {
			u := g.f.Number(0, TotalUsers-1)
			if u == p.AuthorIdx {
				continue
			}
			addEngagement("bookmark", i, u)
		}
	}
}

func kindIdx(kind string) int {
	switch kind {
	case "like":
		return 0
	case "repost":
		return 1
	case "bookmark":
		return 2
	}
	return 3
}

// genDMs — ~10 conversations with alternating messages.
func (g *generator) genDMs() {
	for c := 0; c < DMConversations; c++ {
		a := g.f.Number(0, TotalUsers-1)
		b := g.f.Number(0, TotalUsers-1)
		if b == a {
			b = (b + 1) % TotalUsers
		}
		conv := GenDMConversation{UserAIdx: a, UserBIdx: b}
		for m := 0; m < g.f.Number(3, 8); m++ {
			sender := a
			if m%2 == 1 {
				sender = b
			}
			conv.Messages = append(conv.Messages, GenDMMessage{
				SenderIdx: sender,
				Body:      g.clampDM(g.f.HipsterSentence()),
			})
		}
		g.ds.DMConversations = append(g.ds.DMConversations, conv)
	}
}

// clampDM limits message length to 2000 chars.
func (g *generator) clampDM(s string) string {
	runes := []rune(s)
	if len(runes) <= 2000 {
		return s
	}
	return string(runes[:2000])
}

// genLists — a handful of curated lists.
func (g *generator) genLists() {
	defs := []struct {
		owner int
		name  string
		desc  string
	}{
		{0, "Engineering reads", "Articles worth your time"},
		{1, "Design inspiration", ""},
		{3, "Travel bucket list", "Places on my radar"},
		{5, "Music to work to", "Focus playlist"},
		{8, "Startup crew", "Founders and builders"},
		{12, "Book club 2026", "This year's picks"},
	}
	for i := len(defs); i < Lists; i++ {
		defs = append(defs, struct {
			owner int
			name  string
			desc  string
		}{
			owner: g.f.Number(0, TotalUsers-1),
			name:  g.f.Sentence(2),
			desc:  g.f.Sentence(5),
		})
	}
	for i := 0; i < min(Lists, len(defs)); i++ {
		list := GenList{OwnerIdx: defs[i].owner, Name: defs[i].name, Description: defs[i].desc}
		for m := 0; m < g.f.Number(4, 10); m++ {
			member := g.f.Number(0, TotalUsers-1)
			if member == list.OwnerIdx {
				continue
			}
			list.MemberIdxs = append(list.MemberIdxs, member)
		}
		g.ds.Lists = append(g.ds.Lists, list)
	}
}

// genBadges — a few ASSIGNED badges, each granted to a user.
func (g *generator) genBadges() {
	defs := []struct {
		key, label, desc, icon string
		recipient              int
	}{
		{"community-star", "Community Star", "Recognized contributor", "star", 2},
		{"bug-hunter", "Bug Hunter", "Found and filed a production bug", "shield", 6},
		{"early-adopter", "Early Adopter", "One of our first members", "rocket", 13},
	}
	for _, d := range defs {
		g.ds.Badges = append(g.ds.Badges, GenBadgeGrant{
			Key: d.key, Label: d.label, Description: d.desc, Icon: d.icon,
			RecipientIdx: d.recipient,
		})
	}
}

// GenerateFixed is a convenience wrapper producing a seeded dataset using
// the standard seed value below: useful for tests and tools.
func GenerateFixed(now time.Time) *Dataset {
	return Generate(gofakeit.New(SeedValue), now)
}
