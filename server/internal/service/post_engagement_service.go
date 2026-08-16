package service

import (
	"context"
	"log/slog"

	"github.com/ba-reynolds/gophersocial/internal/apperrors"
	"github.com/ba-reynolds/gophersocial/internal/models"
	"github.com/ba-reynolds/gophersocial/internal/store"
	"github.com/ba-reynolds/gophersocial/internal/util"
)

type PostEngagementService struct {
	store  *store.Store
	logger *slog.Logger
}

func NewPostEngagementService(store *store.Store, logger *slog.Logger) *PostEngagementService {
	return &PostEngagementService{store: store, logger: logger}
}

func (s *PostEngagementService) Like(ctx context.Context, postID, userID int) error {
	tx, err := s.store.DB.BeginTx(ctx, nil)
	if err != nil {
		s.logger.Error("failed to begin transaction for like", "operation", "like_post", "postID", postID, "userID", userID, "error", err)
		return apperrors.InternalServerError(err)
	}
	defer tx.Rollback()

	if err := s.store.PostEngagements.Like(ctx, tx, postID, userID); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		s.logger.Error("failed to commit transaction for like", "operation", "like_post", "postID", postID, "userID", userID, "error", err)
		return apperrors.InternalServerError(err)
	}

	s.logger.Info("post liked successfully", "postID", postID, "userID", userID)
	return nil
}

func (s *PostEngagementService) Unlike(ctx context.Context, postID, userID int) error {
	tx, err := s.store.DB.BeginTx(ctx, nil)
	if err != nil {
		s.logger.Error("failed to begin transaction for unlike", "operation", "unlike_post", "postID", postID, "userID", userID, "error", err)
		return apperrors.InternalServerError(err)
	}
	defer tx.Rollback()

	if err := s.store.PostEngagements.Unlike(ctx, tx, postID, userID); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		s.logger.Error("failed to commit transaction for unlike", "operation", "unlike_post", "postID", postID, "userID", userID, "error", err)
		return apperrors.InternalServerError(err)
	}

	s.logger.Info("post unliked successfully", "postID", postID, "userID", userID)
	return nil
}

func (s *PostEngagementService) Repost(ctx context.Context, postID, userID int) error {
	tx, err := s.store.DB.BeginTx(ctx, nil)
	if err != nil {
		s.logger.Error("failed to begin transaction for repost", "operation", "repost_post", "postID", postID, "userID", userID, "error", err)
		return apperrors.InternalServerError(err)
	}
	defer tx.Rollback()

	if err := s.store.PostEngagements.Repost(ctx, tx, postID, userID); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		s.logger.Error("failed to commit transaction for repost", "operation", "repost_post", "postID", postID, "userID", userID, "error", err)
		return apperrors.InternalServerError(err)
	}

	s.logger.Info("post reposted successfully", "postID", postID, "userID", userID)
	return nil
}

func (s *PostEngagementService) Unrepost(ctx context.Context, postID, userID int) error {
	tx, err := s.store.DB.BeginTx(ctx, nil)
	if err != nil {
		s.logger.Error("failed to begin transaction for unrepost", "operation", "unrepost_post", "postID", postID, "userID", userID, "error", err)
		return apperrors.InternalServerError(err)
	}
	defer tx.Rollback()

	if err := s.store.PostEngagements.Unrepost(ctx, tx, postID, userID); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		s.logger.Error("failed to commit transaction for unrepost", "operation", "unrepost_post", "postID", postID, "userID", userID, "error", err)
		return apperrors.InternalServerError(err)
	}

	s.logger.Info("post unreposted successfully", "postID", postID, "userID", userID)
	return nil
}

func (s *PostEngagementService) Bookmark(ctx context.Context, postID, userID int, categoryID *int) error {
	tx, err := s.store.DB.BeginTx(ctx, nil)
	if err != nil {
		s.logger.Error("failed to begin transaction for bookmark", "operation", "bookmark_post", "postID", postID, "userID", userID, "categoryID", categoryID, "error", err)
		return apperrors.InternalServerError(err)
	}
	defer tx.Rollback()

	if err := s.store.PostEngagements.Bookmark(ctx, tx, postID, userID, categoryID); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		s.logger.Error("failed to commit transaction for bookmark", "operation", "bookmark_post", "postID", postID, "userID", userID, "categoryID", categoryID, "error", err)
		return apperrors.InternalServerError(err)
	}

	s.logger.Info("post bookmarked successfully", "postID", postID, "userID", userID, "categoryID", categoryID)
	return nil
}

func (s *PostEngagementService) Unbookmark(ctx context.Context, postID, userID int) error {
	tx, err := s.store.DB.BeginTx(ctx, nil)
	if err != nil {
		s.logger.Error("failed to begin transaction for unbookmark", "operation", "unbookmark_post", "postID", postID, "userID", userID, "error", err)
		return apperrors.InternalServerError(err)
	}
	defer tx.Rollback()

	if err := s.store.PostEngagements.Unbookmark(ctx, tx, postID, userID); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		s.logger.Error("failed to commit transaction for unbookmark", "operation", "unbookmark_post", "postID", postID, "userID", userID, "error", err)
		return apperrors.InternalServerError(err)
	}

	s.logger.Info("post unbookmarked successfully", "postID", postID, "userID", userID)
	return nil
}

func (s *PostEngagementService) AddView(ctx context.Context, postID int, userID *int, ipAddress, userAgent string) error {
	if err := s.store.PostEngagements.AddView(ctx, postID, userID, ipAddress, userAgent); err != nil {
		s.logger.Error("failed to add view", "operation", "add_view", "postID", postID, "userID", userID, "ipAddress", ipAddress, "userAgent", userAgent, "error", err)
		return err
	}
	s.logger.Info("view added successfully", "postID", postID, "userID", userID, "ipAddress", ipAddress)
	return nil
}

func (s *PostEngagementService) CreateBookmarkCategory(ctx context.Context, userID int, categoryName, color string) (*models.BookmarkCategory, error) {
	if !util.IsHexColor(color) {
		s.logger.Warn("invalid hex color for bookmark category", "userID", userID, "categoryName", categoryName, "color", color)
		return nil, apperrors.BadRequestError("invalid hex color format (expected #RRGGBB)", nil)
	}
	tx, err := s.store.DB.BeginTx(ctx, nil)
	if err != nil {
		s.logger.Error("failed to begin transaction for create bookmark category", "operation", "create_bookmark_category", "userID", userID, "categoryName", categoryName, "color", color, "error", err)
		return nil, apperrors.InternalServerError(err)
	}
	defer tx.Rollback()

	cat, err := s.store.PostEngagements.CreateBookmarkCategory(ctx, tx, userID, categoryName, color)
	if err != nil {
		if appErr, ok := err.(*apperrors.AppError); ok && appErr.Code == apperrors.AlreadyExists {
			s.logger.Info("duplicate bookmark category name", "userID", userID, "categoryName", categoryName)
			return nil, err
		}
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		s.logger.Error("failed to commit transaction for create bookmark category", "operation", "create_bookmark_category", "userID", userID, "categoryName", categoryName, "color", color, "error", err)
		return nil, apperrors.InternalServerError(err)
	}

	s.logger.Info("bookmark category created successfully", "userID", userID, "categoryName", categoryName, "color", color, "categoryID", cat.CategoryID)
	return cat, nil
}

func (s *PostEngagementService) ListBookmarkCategories(ctx context.Context, userID int) ([]models.BookmarkCategory, error) {
	categories, err := s.store.PostEngagements.ListBookmarkCategories(ctx, userID)
	if err != nil {
		s.logger.Error("failed to list bookmark categories", "operation", "list_bookmark_categories", "userID", userID, "error", err)
		return nil, err
	}
	s.logger.Info("bookmark categories listed successfully", "userID", userID, "count", len(categories))
	return categories, nil
}

func (s *PostEngagementService) DeleteBookmarkCategory(ctx context.Context, userID, categoryID int) error {
	tx, err := s.store.DB.BeginTx(ctx, nil)
	if err != nil {
		s.logger.Error("failed to begin transaction for delete bookmark category", "operation", "delete_bookmark_category", "userID", userID, "categoryID", categoryID, "error", err)
		return apperrors.InternalServerError(err)
	}
	defer tx.Rollback()

	err = s.store.PostEngagements.DeleteBookmarkCategory(ctx, tx, userID, categoryID)
	if err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		s.logger.Error("failed to commit transaction for delete bookmark category", "operation", "delete_bookmark_category", "userID", userID, "categoryID", categoryID, "error", err)
		return apperrors.InternalServerError(err)
	}

	s.logger.Info("bookmark category deleted successfully", "userID", userID, "categoryID", categoryID)
	return nil
}
