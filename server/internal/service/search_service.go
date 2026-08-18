package service

import (
	"context"
	"log/slog"
	"strings"

	"github.com/ba-reynolds/gophersocial/internal/apperrors"
	"github.com/ba-reynolds/gophersocial/internal/models"
	"github.com/ba-reynolds/gophersocial/internal/store"
	"github.com/ba-reynolds/gophersocial/pkg/config"
)

type SearchService struct {
	store  *store.Store
	logger *slog.Logger
	config config.AppConfig
}

func NewSearchService(st *store.Store, logger *slog.Logger, cfg config.AppConfig) *SearchService {
	return &SearchService{store: st, logger: logger, config: cfg}
}

func (s *SearchService) Posts(ctx context.Context, viewerID int, query string, limit int, cursor string) (*models.PostFeed, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, apperrors.BadRequestError("search query is required", nil)
	}
	feed, err := s.store.Posts.Search(ctx, query, s.normalizeLimit(limit), cursor)
	if err != nil {
		s.logger.Error("post search failed", "error", err)
		return nil, err
	}
	return s.hydrateFeed(ctx, feed, viewerID)
}

func (s *SearchService) HashtagPosts(ctx context.Context, viewerID int, name string, limit int, cursor string) (*models.PostFeed, error) {
	name = strings.TrimPrefix(strings.ToLower(strings.TrimSpace(name)), "#")
	if name == "" {
		return nil, apperrors.BadRequestError("hashtag is required", nil)
	}
	feed, err := s.store.Posts.ListByHashtag(ctx, name, s.normalizeLimit(limit), cursor)
	if err != nil {
		return nil, err
	}
	return s.hydrateFeed(ctx, feed, viewerID)
}

func (s *SearchService) Users(ctx context.Context, query string, limit int) (*models.UserList, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, apperrors.BadRequestError("search query is required", nil)
	}
	return s.store.Users.Search(ctx, query, s.normalizeLimit(limit))
}

func (s *SearchService) Trends(ctx context.Context, limit int) ([]models.Trend, error) {
	return s.store.Hashtags.Trends(ctx, s.normalizeLimit(limit))
}

func (s *SearchService) normalizeLimit(limit int) int {
	return validateLimit(limit, s.config.DefaultPaginationLimit, s.config.MaxPaginationLimit)
}

func (s *SearchService) hydrateFeed(ctx context.Context, feed *models.PostFeed, viewerID int) (*models.PostFeed, error) {
	for index, item := range feed.Items {
		full, err := s.store.Posts.GetFullPostByID(ctx, item.ID)
		if err != nil {
			s.logger.Error("search result hydration failed", "post_id", item.ID, "error", err)
			return nil, err
		}
		feed.Items[index] = full
	}
	ids := make([]int, 0, len(feed.Items))
	for _, item := range feed.Items {
		ids = append(ids, item.ID)
	}
	polls, err := s.store.Polls.GetForPosts(ctx, ids, viewerID)
	if err != nil {
		s.logger.Error("search poll hydration failed", "error", err)
		return nil, err
	}
	for _, item := range feed.Items {
		item.Poll = polls[item.ID]
	}
	if err := s.store.Media.FetchPostMedia(ctx, feed.Items); err != nil {
		s.logger.Error("search media hydration failed", "error", err)
		return nil, err
	}
	engagements, err := s.store.PostEngagements.GetEngagementForPosts(ctx, ids, viewerID)
	if err != nil {
		s.logger.Error("search engagement hydration failed", "error", err)
		return nil, err
	}
	for _, item := range feed.Items {
		eng, ok := engagements[item.ID]
		if !ok {
			eng = &models.PostEngagement{}
		}
		eng.LikeCount = item.LikesCount
		eng.RepostCount = item.RepostsCount
		eng.ReplyCount = item.RepliesCount
		eng.ViewCount = item.ViewsCount
		eng.BookmarkCount = item.BookmarksCount
		eng.QuoteCount = item.QuotesCount
		item.Engagement = eng
	}
	return feed, nil
}

func validateLimit(limit, defaultLimit, maxLimit int) int {
	if limit <= 0 {
		return defaultLimit
	}
	if limit > maxLimit {
		return maxLimit
	}
	return limit
}
