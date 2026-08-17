package service

import (
	"context"
	"log/slog"
	"strings"

	"github.com/ba-reynolds/gophersocial/internal/apperrors"
	"github.com/ba-reynolds/gophersocial/internal/models"
	"github.com/ba-reynolds/gophersocial/internal/store"
	"github.com/ba-reynolds/gophersocial/pkg/config"
	"github.com/google/uuid"
)

type PostService struct {
	store  *store.Store
	logger *slog.Logger
	config config.AppConfig
}

// validateAndNormalizeLimit validates and normalizes pagination limits
func (s *PostService) validateAndNormalizeLimit(limit int) int {
	if limit <= 0 {
		return s.config.DefaultPaginationLimit
	}
	if limit > s.config.MaxPaginationLimit {
		return s.config.MaxPaginationLimit
	}
	return limit
}

// GetByID retrieves a post by its ID
func (s *PostService) GetByID(ctx context.Context, id int) (*models.Post, error) {
	post, err := s.store.Posts.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	return post, nil
}

// GetFullPostByID retrieves a full post with author information and the
// requesting user's engagement state.
func (s *PostService) GetFullPostByID(ctx context.Context, id int, viewerID int) (*models.FullPost, error) {
	post, err := s.store.Posts.GetFullPostByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if err := s.store.Media.FetchPostMedia(ctx, []*models.FullPost{post}); err != nil {
		s.logger.Error("failed to get media associated with post",
			"operation", "get_full_post_by_id",
			"postID", id,
			"error", err,
		)
		return nil, err
	}

	if err := s.hydrateEngagement(ctx, []*models.FullPost{post}, viewerID); err != nil {
		return nil, err
	}

	return post, nil
}

// hydrateEngagement populates each post's Engagement object with the counts
// from the post row plus the viewer-specific flags (liked/reposted/bookmarked).
func (s *PostService) hydrateEngagement(ctx context.Context, posts []*models.FullPost, viewerID int) error {
	if len(posts) == 0 {
		return nil
	}

	ids := make([]int, 0, len(posts))
	for _, p := range posts {
		if p != nil {
			ids = append(ids, p.ID)
		}
	}

	engagements, err := s.store.PostEngagements.GetEngagementForPosts(ctx, ids, viewerID)
	if err != nil {
		s.logger.Error("failed to load engagement state",
			"operation", "hydrate_engagement",
			"viewerID", viewerID,
			"postIDs", ids,
			"error", err,
		)
		return err
	}

	for _, p := range posts {
		if p == nil {
			continue
		}
		eng, ok := engagements[p.ID]
		if !ok {
			eng = &models.PostEngagement{}
		}
		eng.LikeCount = p.LikesCount
		eng.RepostCount = p.RepostsCount
		eng.ReplyCount = p.RepliesCount
		eng.ViewCount = p.ViewsCount
		eng.BookmarkCount = p.BookmarksCount
		eng.QuoteCount = p.QuotesCount
		p.Engagement = eng
	}

	return nil
}

// GetFullPostByIDWithAncestors retrieves a full post with optional ancestor chain
func (s *PostService) GetFullPostByIDWithAncestors(ctx context.Context, id int, viewerID int, includeAncestors bool, ancestorLimit int) (*models.FullPost, *models.PostChain, error) {
	post, err := s.GetFullPostByID(ctx, id, viewerID)
	if err != nil {
		return nil, nil, err
	}

	var ancestors *models.PostChain
	if includeAncestors {
		normalizedLimit := s.validateAndNormalizeLimit(ancestorLimit)
		ancestors, err = s.GetParentChain(ctx, id, viewerID, normalizedLimit, "")
		if err != nil {
			return nil, nil, err
		}
	}

	return post, ancestors, nil
}

// GetDescendants retrieves direct replies to a post up to a specified limit
func (s *PostService) GetDescendants(ctx context.Context, postID int, viewerID int, limit int, cursor string) (*models.PostDescendants, error) {
	// Validate the post exists first
	_, err := s.store.Posts.GetByID(ctx, postID)
	if err != nil {
		return nil, err
	}

	// Normalize the limit
	normalizedLimit := s.validateAndNormalizeLimit(limit)

	// Get the descendants
	descendants, err := s.store.Posts.GetDescendants(ctx, postID, normalizedLimit, cursor)
	if err != nil {
		return nil, err
	}

	if err := s.store.Media.FetchPostMedia(ctx, descendants.Items); err != nil {
		return nil, err
	}

	if err := s.hydrateEngagement(ctx, descendants.Items, viewerID); err != nil {
		return nil, err
	}

	return descendants, nil
}

// GetFullPostByIDWithAncestorsAndDescendants retrieves a full post with optional ancestor and descendant chains
func (s *PostService) GetFullPostByIDWithAncestorsAndDescendants(ctx context.Context, id int, viewerID int, includeAncestors bool, ancestorLimit int, includeDescendants bool, descendantLimit int) (*models.FullPost, *models.PostChain, *models.PostDescendants, error) {
	post, err := s.GetFullPostByID(ctx, id, viewerID)
	if err != nil {
		return nil, nil, nil, err
	}

	var ancestors *models.PostChain
	if includeAncestors {
		normalizedAncestorLimit := s.validateAndNormalizeLimit(ancestorLimit)
		ancestors, err = s.GetParentChain(ctx, id, viewerID, normalizedAncestorLimit, "")
		if err != nil {
			return nil, nil, nil, err
		}
	}

	var descendants *models.PostDescendants
	if includeDescendants {
		normalizedDescendantLimit := s.validateAndNormalizeLimit(descendantLimit)
		descendants, err = s.GetDescendants(ctx, id, viewerID, normalizedDescendantLimit, "")
		if err != nil {
			return nil, nil, nil, err
		}
	}

	return post, ancestors, descendants, nil
}

// Create creates a new post
func (s *PostService) Create(ctx context.Context, post *models.Post, mediaItems []models.PostMediaRequest) error {
	// Start a transaction for post creation
	tx, err := s.store.DB.BeginTx(ctx, nil)
	if err != nil {
		// Log transaction errors - these are service layer concerns
		s.logger.Error("failed to begin transaction for post creation",
			"operation", "create_post",
			"authorID", post.AuthorID,
			"parentID", post.ParentID,
			"error", err,
		)
		return apperrors.InternalServerError(err)
	}
	defer tx.Rollback()

	// Check that parent post exists
	if post.ParentID != nil {
		parentPost, err := s.store.Posts.GetByID(ctx, *post.ParentID)
		if err != nil {
			// Don't log here - storage layer already logged database errors
			// Just handle business logic
			if appErr, ok := err.(*apperrors.AppError); ok && appErr.Code == apperrors.NotFound {
				s.logger.Info("post creation failed due to missing parent",
					"parentID", *post.ParentID,
					"authorID", post.AuthorID,
				)
				return apperrors.NotFoundError("parent post not found", err)
			}
			return err
		}

		if parentPost.SoftDeleted {
			// Log business logic decisions for debugging
			s.logger.Debug("attempted to create post with deleted parent",
				"parentID", *post.ParentID,
				"authorID", post.AuthorID,
				"operation", "create_post",
			)
			return apperrors.NotFoundError("parent post not found", nil)
		}
	}

	// Check that content has been provided
	if len(mediaItems) == 0 && strings.TrimSpace(post.Content) == "" {
		// Log business logic decisions for debugging
		s.logger.Debug("post creation failed due to missing content",
			"authorID", post.AuthorID,
			"parentID", post.ParentID,
			"operation", "create_post",
		)
		return apperrors.BadRequestError("post content is required", nil)
	}

	// Create the post
	if err := s.store.Posts.Create(ctx, tx, post); err != nil {
		// Don't log here - storage layer already logged database errors
		// Just handle business logic
		return err
	}

	// associate each media item, preserving order as Position
	for idx, m := range mediaItems {
		mediaUUID, err := uuid.Parse(m.UUID)
		if err != nil {
			// Log validation errors - these are service layer concerns
			s.logger.Error("invalid media UUID format",
				"operation", "create_post",
				"authorID", post.AuthorID,
				"postID", post.ID,
				"mediaUUID", m.UUID,
				"error", err,
			)
			return apperrors.BadRequestError("invalid media UUID format", err)
		}

		pm := models.PostMedia{
			PostID:    post.ID,
			MediaUUID: mediaUUID,
			Position:  idx + 1,
			AltText:   m.AltText,
		}

		if err := s.store.Media.LinkMediaToPost(ctx, tx, pm); err != nil {
			// Don't log here - storage layer already logged database errors
			// Just handle business logic
			return err
		}
	}
	if err := s.store.Hashtags.SyncPost(ctx, tx, post.ID, post.Content); err != nil {
		return err
	}

	// Commit the transaction
	if err := tx.Commit(); err != nil {
		// Log transaction commit errors - these are service layer concerns
		s.logger.Error("failed to commit transaction for post creation",
			"operation", "create_post",
			"authorID", post.AuthorID,
			"postID", post.ID,
			"error", err,
		)
		return apperrors.InternalServerError(err)
	}

	// Log successful business operations
	s.logger.Info("post created successfully",
		"postID", post.ID,
		"authorID", post.AuthorID,
		"parentID", post.ParentID,
		"mediaCount", len(mediaItems),
	)

	return nil
}

func (s *PostService) DeleteByID(ctx context.Context, post *models.Post, actorID int) error {
	if post.AuthorID != actorID {
		// Log business logic decisions for debugging
		s.logger.Debug("attempted to delete post by non-author",
			"postID", post.ID,
			"authorID", post.AuthorID,
			"actorID", actorID,
			"operation", "delete_post_by_id",
		)
		return apperrors.ForbiddenError("not authorized to delete this post", nil)
	}

	err := s.store.Posts.DeleteByID(ctx, post.ID)
	if err != nil {
		// Don't log here - storage layer already logged database errors
		// Just handle business logic
		return err
	}

	// Log successful business operations
	s.logger.Info("post deleted successfully",
		"postID", post.ID,
		"authorID", post.AuthorID,
	)

	return nil
}

// GetParentChain retrieves a chain of parent posts up to a specified limit
func (s *PostService) GetParentChain(ctx context.Context, postID int, viewerID int, limit int, cursor string) (*models.PostChain, error) {
	// Validate the post exists first
	_, err := s.store.Posts.GetByID(ctx, postID)
	if err != nil {
		// Don't log here - storage layer already logged database errors
		// Just handle business logic
		return nil, err
	}

	// Normalize the limit
	normalizedLimit := s.validateAndNormalizeLimit(limit)

	// Get the parent chain
	chain, err := s.store.Posts.GetParentChain(ctx, postID, normalizedLimit, cursor)
	if err != nil {
		// Don't log here - storage layer already logged database errors
		// Just handle business logic
		return nil, err
	}

	if err := s.store.Media.FetchPostMedia(ctx, chain.Items); err != nil {
		return nil, err
	}

	if err := s.hydrateEngagement(ctx, chain.Items, viewerID); err != nil {
		return nil, err
	}

	return chain, nil
}

// GetHomeFeed retrieves posts from users that the authenticated user follows
func (s *PostService) GetHomeFeed(ctx context.Context, userID int, limit int, cursor string) (*models.PostFeed, error) {
	// Validate the user exists first
	_, err := s.store.Users.GetByID(ctx, userID)
	if err != nil {
		// Don't log here - storage layer already logged database errors
		// Just handle business logic
		return nil, err
	}

	// Normalize the limit
	normalizedLimit := s.validateAndNormalizeLimit(limit)

	// Get the home feed
	feed, err := s.store.Posts.GetHomeFeed(ctx, userID, normalizedLimit, cursor)
	if err != nil {
		// Don't log here - storage layer already logged database errors
		// Just handle business logic
		return nil, err
	}

	if err := s.store.Media.FetchPostMedia(ctx, feed.Items); err != nil {
		return nil, err
	}

	if err := s.hydrateEngagement(ctx, feed.Items, userID); err != nil {
		return nil, err
	}

	return feed, nil
}

// GetUserFeed retrieves posts made by a specific user
func (s *PostService) GetUserFeed(ctx context.Context, userID int, viewerID int, includeReplies bool, limit int, cursor string) (*models.PostFeed, error) {
	// Validate the user exists first
	_, err := s.store.Users.GetByID(ctx, userID)
	if err != nil {
		// Don't log here - storage layer already logged database errors
		// Just handle business logic
		return nil, err
	}

	// Normalize the limit
	normalizedLimit := s.validateAndNormalizeLimit(limit)

	// Get the user feed
	feed, err := s.store.Posts.GetUserFeed(ctx, userID, includeReplies, normalizedLimit, cursor)
	if err != nil {
		// Don't log here - storage layer already logged database errors
		// Just handle business logic
		return nil, err
	}

	if err := s.store.Media.FetchPostMedia(ctx, feed.Items); err != nil {
		return nil, err
	}

	if err := s.hydrateEngagement(ctx, feed.Items, viewerID); err != nil {
		return nil, err
	}

	return feed, nil
}

func (s *PostService) GetBookmarkedPostsFeed(ctx context.Context, userID int, viewerID int, categoryIDs []int, limit int, cursor string) (*models.PostFeed, error) {
	feed, err := s.store.Posts.GetBookmarkedPostsFeed(ctx, userID, categoryIDs, limit, cursor)
	if err != nil {
		s.logger.Error("failed to get bookmarked posts feed", "operation", "get_bookmarked_posts_feed", "userID", userID, "categoryIDs", categoryIDs, "error", err)
		return nil, err
	}
	if err := s.store.Media.FetchPostMedia(ctx, feed.Items); err != nil {
		return nil, err
	}
	if err := s.hydrateEngagement(ctx, feed.Items, viewerID); err != nil {
		return nil, err
	}
	s.logger.Info("bookmarked posts feed fetched", "userID", userID, "categoryIDs", categoryIDs, "count", len(feed.Items))
	return feed, nil
}

func (s *PostService) GetLikedPostsFeed(ctx context.Context, userID int, viewerID int, limit int, cursor string) (*models.PostFeed, error) {
	feed, err := s.store.Posts.GetLikedPostsFeed(ctx, userID, limit, cursor)
	if err != nil {
		s.logger.Error("failed to get liked posts feed", "operation", "get_liked_posts_feed", "userID", userID, "error", err)
		return nil, err
	}
	if err := s.store.Media.FetchPostMedia(ctx, feed.Items); err != nil {
		return nil, err
	}
	if err := s.hydrateEngagement(ctx, feed.Items, viewerID); err != nil {
		return nil, err
	}
	s.logger.Info("liked posts feed fetched", "userID", userID, "count", len(feed.Items))
	return feed, nil
}

// GetQuotesFeed retrieves a paginated feed of posts quoting a given post.
func (s *PostService) GetQuotesFeed(ctx context.Context, postID int, viewerID int, limit int, cursor string) (*models.PostFeed, error) {
	_, err := s.store.Posts.GetByID(ctx, postID)
	if err != nil {
		return nil, err
	}
	feed, err := s.store.Posts.GetQuotesFeed(ctx, postID, limit, cursor)
	if err != nil {
		s.logger.Error("failed to get quotes feed", "operation", "get_quotes_feed", "postID", postID, "error", err)
		return nil, err
	}
	if err := s.store.Media.FetchPostMedia(ctx, feed.Items); err != nil {
		return nil, err
	}
	if err := s.hydrateEngagement(ctx, feed.Items, viewerID); err != nil {
		return nil, err
	}
	return feed, nil
}

// GetPostLikers retrieves a paginated list of users who liked a post.
func (s *PostService) GetPostLikers(ctx context.Context, postID int, limit int, cursor string) (*models.UserList, error) {
	return s.store.PostEngagements.GetPostLikers(ctx, postID, limit, cursor)
}

// GetPostReposters retrieves a paginated list of users who reposted a post.
func (s *PostService) GetPostReposters(ctx context.Context, postID int, limit int, cursor string) (*models.UserList, error) {
	return s.store.PostEngagements.GetPostReposters(ctx, postID, limit, cursor)
}

// QuotePost creates a new post quoting another post (with quoted_post_id set)
func (s *PostService) QuotePost(ctx context.Context, post *models.Post, mediaItems []models.PostMediaRequest) error {
	tx, err := s.store.DB.BeginTx(ctx, nil)
	if err != nil {
		s.logger.Error("failed to begin transaction for quote post creation",
			"operation", "quote_post",
			"authorID", post.AuthorID,
			"quotedPostID", post.QuotedPostID,
			"error", err,
		)
		return apperrors.InternalServerError(err)
	}
	defer tx.Rollback()

	// Validate quoted_post_id exists and is not soft deleted
	if post.QuotedPostID == nil {
		s.logger.Warn("quoted_post_id is required for quoting",
			"operation", "quote_post",
			"authorID", post.AuthorID,
		)
		return apperrors.BadRequestError("quoted post ID is required", nil)
	}
	quoted, err := s.store.Posts.GetByID(ctx, *post.QuotedPostID)
	if err != nil {
		return err
	}
	if quoted.SoftDeleted {
		s.logger.Warn("attempted to quote a deleted post",
			"operation", "quote_post",
			"authorID", post.AuthorID,
			"quotedPostID", *post.QuotedPostID,
		)
		return apperrors.NotFoundError("quoted post not found", nil)
	}

	// Check that content or media is provided
	if len(mediaItems) == 0 && post.Content == "" {
		s.logger.Debug("quote post creation failed due to missing content",
			"authorID", post.AuthorID,
			"quotedPostID", *post.QuotedPostID,
			"operation", "quote_post",
		)
		return apperrors.BadRequestError("quote post content is required", nil)
	}

	// Create the quoted post
	if err := s.store.Posts.CreateQuotedPost(ctx, tx, post); err != nil {
		return err
	}

	// Associate media
	for idx, m := range mediaItems {
		mediaUUID, err := uuid.Parse(m.UUID)
		if err != nil {
			s.logger.Error("invalid media UUID format",
				"operation", "quote_post",
				"authorID", post.AuthorID,
				"postID", post.ID,
				"mediaUUID", m.UUID,
				"error", err,
			)
			return apperrors.BadRequestError("invalid media UUID format", err)
		}
		pm := models.PostMedia{
			PostID:    post.ID,
			MediaUUID: mediaUUID,
			Position:  idx + 1,
			AltText:   m.AltText,
		}
		if err := s.store.Media.LinkMediaToPost(ctx, tx, pm); err != nil {
			return err
		}
	}
	if err := s.store.Hashtags.SyncPost(ctx, tx, post.ID, post.Content); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		s.logger.Error("failed to commit transaction for quote post creation",
			"operation", "quote_post",
			"authorID", post.AuthorID,
			"postID", post.ID,
			"error", err,
		)
		return apperrors.InternalServerError(err)
	}

	s.logger.Info("quote post created successfully",
		"postID", post.ID,
		"authorID", post.AuthorID,
		"quotedPostID", *post.QuotedPostID,
		"mediaCount", len(mediaItems),
	)
	return nil
}
