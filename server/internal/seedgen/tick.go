package seedgen

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/ba-reynolds/gaggle/internal/models"
	"github.com/ba-reynolds/gaggle/internal/store"
	"github.com/brianvoe/gofakeit/v7"
)

// Post visibility values, matching the service layer and the posts.visibility
// CHECK constraint (see service/post_service.go).
const (
	VisibilityPublic    = "public"
	VisibilityFollowers = "followers"
	VisibilityMentions  = "mentions"
)

// Tick performs one small wave of live user activity on top of an existing
// seeded database: a handful of new posts/replies/likes/DM messages from the
// seeded users, all stamped near now() so the community keeps growing over
// time. It is the seam the future scheduled simulator (cmd/simulate / cron on
// the box) calls, and it reuses the same store primitives as Apply.
//
// Unlike Apply, Tick is intentionally NOT idempotent — it is a live actor and
// each run should produce fresh rows (its job is growth). It does not touch
// the anchor guard.
func Tick(ctx context.Context, st *store.Store, log *slog.Logger, f *gofakeit.Faker, now time.Time, postsPerTick int) error {
	// Resolve the seeded user pool (any who exist; the DB was seeded first).
	// NB: the store truncates results to Items[:limit], so limit 0 returns an
	// empty list even though the query itself matched a row. Use a real cap.
	users, err := st.Users.Search(ctx, "", 100)
	if err != nil {
		return fmt.Errorf("load users: %w", err)
	}
	if len(users.Items) == 0 {
		return fmt.Errorf("no users to simulate with; seed the database first")
	}

	// Random timestamps clustered around "now" (spread over the last 6h).
	var recentPostIDs []int
	for i := 0; i < postsPerTick; i++ {
		author := users.Items[f.Number(0, len(users.Items)-1)]
		authorID := author.UserID
		createdAt := now.Add(-time.Duration(f.Number(0, 6*60*60)) * time.Second)

		// Fresh top-level posts carry hashtags/mentions like the bulk seed.
		post := &models.Post{
			Content:    f.Sentence(f.Number(6, 14)),
			AuthorID:   authorID,
			Visibility: VisibilityPublic,
			CreatedAt:  createdAt,
		}

		tx, err := st.DB.BeginTx(ctx, nil)
		if err != nil {
			return fmt.Errorf("begin tick post tx: %w", err)
		}

		// ~40% rich hashtags like the bulk seed.
		if f.Number(1, 100) <= 40 {
			tag := hashtagPool[f.Number(0, len(hashtagPool)-1)]
			post.Content += " #" + tag
		}
		if err := st.Posts.Create(ctx, tx, post); err != nil {
			tx.Rollback()
			return fmt.Errorf("create tick post: %w", err)
		}
		if err := st.Hashtags.SyncPost(ctx, tx, post.ID, post.Content); err != nil {
			tx.Rollback()
			return fmt.Errorf("sync tick hashtags: %w", err)
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit tick post tx: %w", err)
		}
		recentPostIDs = append(recentPostIDs, post.ID)

		// ~1/3 of new posts get a couple of likes from other seeded users.
		if f.Number(1, 100) <= 33 {
			for l := 0; l < f.Number(1, 3); l++ {
				liker := users.Items[f.Number(0, len(users.Items)-1)]
				if liker.UserID == authorID {
					continue
				}
				if _, err := st.PostEngagements.Like(ctx, nil, post.ID, liker.UserID); err != nil {
					return fmt.Errorf("like tick post: %w", err)
				}
			}
		}

		log.Info("simulated post", "post_id", post.ID, "author", author.Username)
	}

	// A few replies against the freshest posts.
	for i := 0; i < f.Number(2, 5) && len(recentPostIDs) > 0; i++ {
		parent := recentPostIDs[f.Number(0, len(recentPostIDs)-1)]
		replier := users.Items[f.Number(0, len(users.Items)-1)]
		reply := &models.Post{
			Content:    replyOpeners[f.Number(0, len(replyOpeners)-1)],
			AuthorID:   replier.UserID,
			ParentID:   &parent,
			Visibility: VisibilityPublic,
			CreatedAt:  now.Add(-time.Duration(f.Number(0, 60*60)) * time.Second),
		}
		tx, err := st.DB.BeginTx(ctx, nil)
		if err != nil {
			return fmt.Errorf("begin tick reply tx: %w", err)
		}
		if err := st.Posts.Create(ctx, tx, reply); err != nil {
			tx.Rollback()
			return fmt.Errorf("create tick reply: %w", err)
		}
		if err := st.Hashtags.SyncPost(ctx, tx, reply.ID, reply.Content); err != nil {
			tx.Rollback()
			return fmt.Errorf("sync tick reply hashtags: %w", err)
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit tick reply tx: %w", err)
		}
		log.Info("simulated reply", "post_id", reply.ID, "parent", parent, "author", replier.Username)
	}

	// A handful of DM messages in random existing conversations. Scan users
	// until we find one with an inbox; the store's Conversation items expose
	// the viewer themselves and OtherParticipant (ParticipantA/B are never
	// populated by ListConversations), so always send as a known participant.
	for k := 0; k < len(users.Items); k++ {
		viewerID := users.Items[k].UserID
		inbox, err := st.DMs.ListConversations(ctx, viewerID)
		if err != nil {
			return fmt.Errorf("list conversations: %w", err)
		}
		if len(inbox.Items) == 0 {
			continue
		}
		for m := 0; m < f.Number(1, 3); m++ {
			conv := inbox.Items[f.Number(0, len(inbox.Items)-1)]
			sender := viewerID
			if conv.OtherParticipant != nil && f.Number(0, 1) == 1 {
				sender = conv.OtherParticipant.UserID
			}
			if _, err := st.DMs.AddMessage(ctx, conv.ID, sender, f.HipsterSentence()); err != nil {
				return fmt.Errorf("simulate DM: %w", err)
			}
		}
		break
	}

	return nil
}
