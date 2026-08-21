package seedgen

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/ba-reynolds/gaggle/internal/apperrors"
	"github.com/ba-reynolds/gaggle/internal/models"
	"github.com/ba-reynolds/gaggle/internal/store"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

// ApplyOptions carries runtime knobs for Apply (media dir for placeholder
// image files, etc.).
type ApplyOptions struct {
	MediaDir string // directory to write <uuid> placeholder files into
}

// Apply writes the generated Dataset into the database via the store layer.
// It is safe to run repeatedly: individual writes are idempotent or guarded
// here (block/mute pairs are unique; duplicates are swallowed as AlreadyExists).
// It does NOT touch Redis — the server's writes invalidate caches from
// handlers only, and fresh-seed happens before any client traffic.
//
// Ordering matters:
//  1. media rows + files (profiles reference media via FK before users insert)
//  2. users + profiles (+ admin + private flags)
//  3. posts (top-level then replies) with hashtag/mention sync per post
//  4. engagement (likes/reposts/bookmarks) + polls + relationships
//  5. DMs, lists, badges, post-media links
func Apply(ctx context.Context, st *store.Store, log *slog.Logger, ds *Dataset, opts ApplyOptions) error {
	if err := applyMediaRows(ctx, st, log, ds, opts.MediaDir); err != nil {
		return fmt.Errorf("seed: media rows: %w", err)
	}
	if err := applyUsers(ctx, st, log, ds); err != nil {
		return fmt.Errorf("seed: users: %w", err)
	}
	userCatIDs, err := applyBookmarkCategories(ctx, st, log, ds)
	if err != nil {
		return fmt.Errorf("seed: bookmark categories: %w", err)
	}
	if err := applyPosts(ctx, st, log, ds); err != nil {
		return fmt.Errorf("seed: posts: %w", err)
	}
	if err := applyPolls(ctx, st, log, ds); err != nil {
		return fmt.Errorf("seed: polls: %w", err)
	}
	if err := applyEngagement(ctx, st, log, ds, userCatIDs); err != nil {
		return fmt.Errorf("seed: engagement: %w", err)
	}
	if err := applyRelationships(ctx, st, log, ds); err != nil {
		return fmt.Errorf("seed: relationships: %w", err)
	}
	if err := applyDMs(ctx, st, log, ds); err != nil {
		return fmt.Errorf("seed: dms: %w", err)
	}
	if err := applyLists(ctx, st, log, ds); err != nil {
		return fmt.Errorf("seed: lists: %w", err)
	}
	if err := applyBadges(ctx, st, log, ds); err != nil {
		return fmt.Errorf("seed: badges: %w", err)
	}
	if err := applyPostMediaLinks(ctx, st, log, ds); err != nil {
		return fmt.Errorf("seed: post media links: %w", err)
	}
	return nil
}

// hashPassword produces the bcrypt hash the store expects (bcrypt.DefaultCost).
func hashPassword(plain string) (string, error) {
	h, err := bcrypt.GenerateFromPassword([]byte(plain), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(h), nil
}

// applyUsers creates every user + profile. It records the mapping from dataset
// user index to DB user ID in ds.UserIDs (other Apply steps consume it).
func applyUsers(ctx context.Context, st *store.Store, log *slog.Logger, ds *Dataset) error {
	ids := make([]int, len(ds.Users))

	for i, gu := range ds.Users {
		tx, err := st.DB.BeginTx(ctx, nil)
		if err != nil {
			return fmt.Errorf("begin user tx: %w", err)
		}

		hash, err := hashPassword(gu.Password)
		if err != nil {
			tx.Rollback()
			return fmt.Errorf("hash password for %s: %w", gu.Username, err)
		}

		user := &models.User{
			Username: gu.Username,
			Email:    gu.Email,
			Password: hash,
		}
		if err := st.Users.Create(ctx, tx, user); err != nil {
			tx.Rollback()
			return fmt.Errorf("create user %s: %w", gu.Username, err)
		}
		ids[i] = user.ID

		// Rich profile from the generated data.
		profile := &models.UserWithProfile{
			User: *user,
			Profile: models.UserProfile{
				DisplayName:        gu.DisplayName,
				Bio:                gu.Bio,
				ProfilePictureUUID: gu.ProfilePictureUUID,
				BannerUUID:         gu.BannerUUID,
				BirthDate:          models.Date{Time: gu.BirthDate},
				Location:           gu.Location,
				Website:            gu.Website,
			},
		}
		if err := st.Users.UpdateUserProfile(ctx, tx, profile); err != nil {
			tx.Rollback()
			return fmt.Errorf("update profile for %s: %w", gu.Username, err)
		}

		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit user tx for %s: %w", gu.Username, err)
		}

		// is_admin isn't writable through the store (migration 000013's UPDATE
		// runs pre-seed, before alice exists), so promote admin users directly.
		if gu.IsAdmin {
			if _, err := st.DB.ExecContext(ctx,
				`UPDATE users SET is_admin = TRUE WHERE user_id = $1`, user.ID); err != nil {
				return fmt.Errorf("promote %s to admin: %w", gu.Username, err)
			}
		}
		// is_private likewise needs an explicit call (Users.Create only inserts
		// username/email/password).
		if gu.IsPrivate {
			if err := st.Users.SetPrivate(ctx, user.ID, true); err != nil {
				return fmt.Errorf("set %s private: %w", gu.Username, err)
			}
		}

		log.Info("seeded user", "username", gu.Username, "id", user.ID)
	}

	ds.UserIDs = ids
	return nil
}

// applyPosts creates top-level posts then replies in dataset order. Each post
// gets hashtag + mention sync in the same tx so /trends, hashtag feeds, the
// mentions feed and mention notifications all work for seeded content.
func applyPosts(ctx context.Context, st *store.Store, log *slog.Logger, ds *Dataset) error {
	ids := make([]int, len(ds.Posts))

	for i, gp := range ds.Posts {
		tx, err := st.DB.BeginTx(ctx, nil)
		if err != nil {
			return fmt.Errorf("begin post tx: %w", err)
		}

		post := &models.Post{
			Content:    gp.Content,
			AuthorID:   ds.UserIDs[gp.AuthorIdx],
			Visibility: gp.Visibility,
			CreatedAt:  gp.CreatedAt,
		}
		if gp.ParentIdx >= 0 {
			pid := ids[gp.ParentIdx]
			post.ParentID = &pid
		}
		if gp.QuotedIdx >= 0 {
			qid := ids[gp.QuotedIdx]
			post.QuotedPostID = &qid
		}

		if err := st.Posts.Create(ctx, tx, post); err != nil {
			tx.Rollback()
			return fmt.Errorf("create post %d: %w", i, err)
		}
		ids[i] = post.ID

		// Invariants the service layer maintains; the seed must mirror them.
		if err := st.Hashtags.SyncPost(ctx, tx, post.ID, post.Content); err != nil {
			tx.Rollback()
			return fmt.Errorf("sync hashtags for post %d: %w", i, err)
		}
		if err := st.Mentions.SyncPost(ctx, tx, post.ID, post.Content); err != nil {
			tx.Rollback()
			return fmt.Errorf("sync mentions for post %d: %w", i, err)
		}

		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit post tx %d: %w", i, err)
		}
	}

	ds.PostIDs = ids
	return nil
}

// applyPolls creates polls for posts that carry one, then records votes.
// pollStore.Vote requires a live tx and resolves option IDs by Position, so we
// look up each option's ID after Polls.Create.
func applyPolls(ctx context.Context, st *store.Store, log *slog.Logger, ds *Dataset) error {
	for i, gp := range ds.Posts {
		if gp.PollIdx < 0 || gp.PollIdx >= len(ds.Polls) {
			continue
		}
		poll := ds.Polls[gp.PollIdx]

		tx, err := st.DB.BeginTx(ctx, nil)
		if err != nil {
			return fmt.Errorf("begin poll tx: %w", err)
		}

		// Polls.Create stores the payload question; polls.question is
		// VARCHAR(140), so clamp (content can be up to 280 runes).
		question := gp.Content
		if runes := []rune(question); len(runes) > 140 {
			question = string(runes[:140])
		}
		payload := &models.CreatePollPayload{
			Question: question,
			Options:  poll.Options,
			EndsAt:   poll.EndsAt,
		}
		if err := st.Polls.Create(ctx, tx, ds.PostIDs[i], payload); err != nil {
			tx.Rollback()
			return fmt.Errorf("create poll for post %d: %w", i, err)
		}

		// Polls.Create does not return the option IDs; look them up by position
		// so we can translate generator option indices into DB rows.
		optionIDs := make([]int, len(poll.Options))
		for pos := 1; pos <= len(poll.Options); pos++ {
			var id int
			if err := tx.QueryRowContext(ctx,
				`SELECT option_id FROM poll_options WHERE poll_id = (SELECT poll_id FROM polls WHERE post_id = $1) AND position = $2`,
				ds.PostIDs[i], pos).Scan(&id); err != nil {
				tx.Rollback()
				return fmt.Errorf("lookup poll option %d for post %d: %w", pos, i, err)
			}
			optionIDs[pos-1] = id
		}

		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit poll tx %d: %w", i, err)
		}

		// Votes after the poll row exists; Polls.Vote needs its own tx.
		for _, v := range ds.PollVotes {
			if v.PostIdx != i {
				continue
			}
			vtx, err := st.DB.BeginTx(ctx, nil)
			if err != nil {
				return err
			}
			err = st.Polls.Vote(ctx, vtx, ds.PostIDs[i], optionIDs[v.OptionIdx], ds.UserIDs[v.VoterIdx])
			if err != nil {
				vtx.Rollback()
				if apperrors.Is(err, apperrors.AlreadyExists) {
					continue
				}
				// A poll whose ends_at already passed (e.g. re-seeding old
				// data or a fixed test clock) can't accept votes; skip them
				// instead of failing the whole seed.
				if appErr, ok := err.(*apperrors.AppError); ok && appErr.Code == apperrors.BadRequest {
					continue
				}
				return fmt.Errorf("vote on post %d: %w", i, err)
			}
			if err := vtx.Commit(); err != nil {
				return fmt.Errorf("commit vote tx %d: %w", i, err)
			}
		}
	}
	return nil
}

// applyBookmarkCategories creates bookmark_categories only for categories that
// are actually used (ds.BookmarkCategoryNames is pruned to used only by
// genEngagement/Task 2). Returns userIdx -> categoryName -> categoryID, handling
// AlreadyExists on DBs that still had the trigger's "General" row.
func applyBookmarkCategories(ctx context.Context, st *store.Store, log *slog.Logger, ds *Dataset) (map[int]map[string]int, error) {
	colorByName := map[string]string{}
	for _, c := range bookmarkCategoryPool {
		colorByName[c.Name] = c.Color
	}
	out := make(map[int]map[string]int, len(ds.Users))
	for uIdx, names := range ds.BookmarkCategoryNames {
		if len(names) == 0 {
			continue
		}
		uid := ds.UserIDs[uIdx]
		m := make(map[string]int, len(names))
		for _, name := range names {
			color := colorByName[name]
			if color == "" {
				color = "#1DA1F2"
			}
			cat, err := st.PostEngagements.CreateBookmarkCategory(ctx, nil, uid, name, color)
			if err != nil {
				if apperrors.Is(err, apperrors.AlreadyExists) {
					var id int
					if qerr := st.DB.QueryRowContext(ctx,
						`SELECT category_id FROM bookmark_categories WHERE user_id = $1 AND category_name = $2`,
						uid, name).Scan(&id); qerr != nil {
						return nil, fmt.Errorf("lookup bookmark category %q for user %d: %w", name, uid, qerr)
					}
					m[name] = id
					continue
				}
				return nil, fmt.Errorf("create bookmark category %q for user %d: %w", name, uid, err)
			}
			m[name] = cat.CategoryID
		}
		out[uIdx] = m
	}
	return out, nil
}

// applyEngagement inserts like/repost/bookmark rows. Underlying stores are
// NO-OP-on-duplicate, so plain inserts are safe; triggers maintain counts.
// userCatIDs maps dataset userIdx -> categoryName -> DB category_id (from
// applyBookmarkCategories); a bookmark's CategoryName is resolved to *int there.
func applyEngagement(ctx context.Context, st *store.Store, log *slog.Logger, ds *Dataset, userCatIDs map[int]map[string]int) error {
	for _, e := range ds.Likes {
		if _, err := st.PostEngagements.Like(ctx, nil, ds.PostIDs[e.PostIdx], ds.UserIDs[e.UserIdx]); err != nil {
			return fmt.Errorf("like post %d by user %d: %w", e.PostIdx, e.UserIdx, err)
		}
	}
	for _, e := range ds.Reposts {
		if _, err := st.PostEngagements.Repost(ctx, nil, ds.PostIDs[e.PostIdx], ds.UserIDs[e.UserIdx]); err != nil {
			return fmt.Errorf("repost post %d by user %d: %w", e.PostIdx, e.UserIdx, err)
		}
	}
	for _, e := range ds.Bookmarks {
		var catID *int
		if e.CategoryName != nil {
			if m, ok := userCatIDs[e.UserIdx]; ok {
				if id, ok := m[*e.CategoryName]; ok {
					v := id
					catID = &v
				}
			}
		}
		if err := st.PostEngagements.Bookmark(ctx, nil, ds.PostIDs[e.PostIdx], ds.UserIDs[e.UserIdx], catID); err != nil {
			return fmt.Errorf("bookmark post %d by user %d: %w", e.PostIdx, e.UserIdx, err)
		}
	}
	_ = log
	return nil
}

// applyRelationships inserts follows/blocks/mutes. Follows appear many times
// across the dataset (dense web); the UNIQUE(follower, following, type) key
// would reject duplicates, so we track what's already written and skip.
func applyRelationships(ctx context.Context, st *store.Store, log *slog.Logger, ds *Dataset) error {
	seen := make(map[string]bool)
	for _, r := range ds.Relationships {
		following := ds.UserIDs[r.FollowingIdx]
		follower := ds.UserIDs[r.FollowerIdx]
		key := fmt.Sprintf("%d:%d:%s", follower, following, r.Type)
		if seen[key] {
			continue
		}
		seen[key] = true

		// Mirror the app's soft-block semantics (service CreateRelationship):
		// blocking removes existing relationships in both directions, so a
		// block row can never coexist with a follow row (Twitter behavior)
		// and a blocked user is instantly unfollowed. genRelationships appends
		// block rows after all follows, so this runs after any colliding follows.
		if r.Type == "block" {
			if err := st.UserRelationships.Delete(ctx, nil, follower, following); err != nil && !apperrors.Is(err, apperrors.NotFound) {
				return fmt.Errorf("soft-block delete %d->%d: %w", follower, following, err)
			}
			if err := st.UserRelationships.Delete(ctx, nil, following, follower); err != nil && !apperrors.Is(err, apperrors.NotFound) {
				return fmt.Errorf("soft-block delete %d->%d: %w", following, follower, err)
			}
		}

		rel := &models.UserRelationship{
			FollowerID:       follower,
			FollowingID:      following,
			RelationshipType: r.Type,
		}
		if err := st.UserRelationships.Create(ctx, nil, rel); err != nil {
			if apperrors.Is(err, apperrors.AlreadyExists) {
				continue
			}
			return fmt.Errorf("relationship %d-%d %s: %w", r.FollowerIdx, r.FollowingIdx, r.Type, err)
		}
	}
	_ = log
	return nil
}

// applyDMs builds conversations + messages. AddMessage requires the
// conversation to exist first (GetOrCreateConversation is idempotent).
func applyDMs(ctx context.Context, st *store.Store, log *slog.Logger, ds *Dataset) error {
	for i, c := range ds.DMConversations {
		conv, err := st.DMs.GetOrCreateConversation(ctx, ds.UserIDs[c.UserAIdx], ds.UserIDs[c.UserBIdx])
		if err != nil {
			return fmt.Errorf("conversation %d: %w", i, err)
		}
		for _, m := range c.Messages {
			if _, err := st.DMs.AddMessage(ctx, conv.ID, ds.UserIDs[m.SenderIdx], m.Body); err != nil {
				return fmt.Errorf("message in conv %d: %w", i, err)
			}
		}
	}
	_ = log
	return nil
}

// applyLists creates lists + memberships (owner excluded by generator).
func applyLists(ctx context.Context, st *store.Store, log *slog.Logger, ds *Dataset) error {
	for i, l := range ds.Lists {
		list := &models.List{
			OwnerID:     ds.UserIDs[l.OwnerIdx],
			Name:        l.Name,
			Description: l.Description,
		}
		if err := st.Lists.Create(ctx, list); err != nil {
			return fmt.Errorf("create list %d: %w", i, err)
		}
		for _, m := range l.MemberIdxs {
			if err := st.Lists.AddMember(ctx, list.ID, ds.UserIDs[m]); err != nil {
				if apperrors.Is(err, apperrors.AlreadyExists) {
					continue
				}
				return fmt.Errorf("add member %d to list %d: %w", m, i, err)
			}
		}
	}
	_ = log
	return nil
}

// applyBadges creates ASSIGNED badges and grants them to the designated users.
// The 4 earned badges are seeded at migration time; the app computes them.
func applyBadges(ctx context.Context, st *store.Store, log *slog.Logger, ds *Dataset) error {
	for i, b := range ds.Badges {
		badge, err := st.Badges.CreateBadge(ctx, models.CreateBadgePayload{
			Key:         b.Key,
			Label:       b.Label,
			Description: b.Description,
			Icon:        b.Icon,
		})
		if err != nil {
			if apperrors.Is(err, apperrors.AlreadyExists) {
				continue
			}
			return fmt.Errorf("create badge %d: %w", i, err)
		}
		if err := st.Badges.GrantBadge(ctx, ds.UserIDs[b.RecipientIdx], badge.ID, ds.UserIDs[0]); err != nil {
			if apperrors.Is(err, apperrors.AlreadyExists) {
				continue
			}
			return fmt.Errorf("grant badge %s: %w", b.Key, err)
		}
	}
	_ = log
	return nil
}

// applyMediaRows writes placeholder PNG bytes for every profile avatar/banner
// and post attachment, and registers each media row. It must run FIRST because
// user_profiles.profile_picture_uuid/banner_uuid are FKs into media. Bytes are
// written directly (the store's SaveFile needs a multipart.File).
func applyMediaRows(ctx context.Context, st *store.Store, log *slog.Logger, ds *Dataset, mediaDir string) error {
	// Profile images (avatars + banners) are referenced by users' UUID fields
	// but have no GenMedia row; materialize them from the user records.
	type img struct {
		media models.Media
	}
	var images []img
	for _, u := range ds.Users {
		if u.ProfilePictureUUID != uuid.Nil {
			images = append(images, img{media: models.Media{UUID: u.ProfilePictureUUID, MimeType: "image/png", Filename: "avatar.png"}})
		}
		if u.BannerUUID != uuid.Nil {
			images = append(images, img{media: models.Media{UUID: u.BannerUUID, MimeType: "image/png", Filename: "banner.png"}})
		}
	}
	for _, m := range ds.Media {
		images = append(images, img{media: models.Media{UUID: m.UUID, MimeType: m.MimeType, Filename: m.Filename}})
	}

	if err := os.MkdirAll(mediaDir, 0o755); err != nil {
		return fmt.Errorf("mkdir media dir: %w", err)
	}

	for _, im := range images {
		if err := writePlaceholderPNG(filepath.Join(mediaDir, im.media.UUID.String()), 64, 64); err != nil {
			return fmt.Errorf("write media file %s: %w", im.media.UUID, err)
		}
		if err := st.Media.Create(ctx, nil, &im.media); err != nil {
			return fmt.Errorf("create media row %s: %w", im.media.UUID, err)
		}
	}
	_ = log
	return nil
}

// applyPostMediaLinks attaches post media to their posts now that posts exist.
func applyPostMediaLinks(ctx context.Context, st *store.Store, log *slog.Logger, ds *Dataset) error {
	for _, m := range ds.Media {
		if m.PostIdx < 0 {
			continue
		}
		pm := models.PostMedia{
			PostID:    ds.PostIDs[m.PostIdx],
			MediaUUID: m.UUID,
			Position:  m.Position,
			AltText:   m.AltText,
		}
		if err := st.Media.LinkMediaToPost(ctx, nil, pm); err != nil {
			return fmt.Errorf("link media %s to post %d: %w", m.UUID, m.PostIdx, err)
		}
	}
	_ = log
	return nil
}

// writePlaceholderPNG writes a tiny valid PNG of the given size so media
// endpoints return image/png bytes for every seeded avatar/banner/post image.
func writePlaceholderPNG(path string, w, h int) error {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for x := 0; x < w; x++ {
		for y := 0; y < h; y++ {
			img.Set(x, y, color.RGBA{R: uint8(x * 255 / w), G: uint8(y * 255 / h), B: 120, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return err
	}
	return os.WriteFile(path, buf.Bytes(), 0o644)
}
