package service

import (
	"context"
	"mime/multipart"

	"github.com/ba-reynolds/vitrilium/internal/apperrors"
	"github.com/ba-reynolds/vitrilium/internal/models"
	"github.com/ba-reynolds/vitrilium/internal/store"
	"github.com/google/uuid"
	"log/slog"
)

type MediaService struct {
	store  *store.Store
	logger *slog.Logger
}

// Create saves a new media file and its metadata
func (s *MediaService) Create(ctx context.Context, media *models.Media, file multipart.File) error {
	// Start a transaction for media creation
	tx, err := s.store.DB.BeginTx(ctx, nil)
	if err != nil {
		// Log transaction errors - these are service layer concerns
		s.logger.Error("failed to begin transaction for media creation",
			"operation", "create_media",
			"mediaUUID", media.UUID,
			"mimeType", media.MimeType,
			"filename", media.Filename,
			"error", err,
		)
		return apperrors.InternalServerError(err)
	}
	defer tx.Rollback()

	// First insert the media record
	if err := s.store.Media.Create(ctx, tx, media); err != nil {
		// Don't log here - storage layer already logged database errors
		// Just handle business logic
		return err
	}

	// Then save the actual file
	if err := s.store.Media.SaveFile(media.UUID, file); err != nil {
		// Don't log here - storage layer already logged file system errors
		// Just handle business logic
		return err
	}

	// Commit the transaction
	if err := tx.Commit(); err != nil {
		// Log transaction commit errors - these are service layer concerns
		s.logger.Error("failed to commit transaction for media creation",
			"operation", "create_media",
			"mediaUUID", media.UUID,
			"error", err,
		)
		return apperrors.InternalServerError(err)
	}

	// Log successful business operations
	s.logger.Info("media created successfully",
		"mediaUUID", media.UUID,
		"mimeType", media.MimeType,
		"filename", media.Filename,
	)

	return nil
}

// GetByID retrieves media by its ID
func (s *MediaService) GetByID(ctx context.Context, id uuid.UUID) (*models.Media, error) {
	media, err := s.store.Media.GetByID(ctx, id)
	if err != nil {
		// Don't log here - storage layer already logged database errors
		// Just handle business logic
		return nil, err
	}

	return media, nil
}

// DeleteByID deletes media by its ID
func (s *MediaService) DeleteByID(ctx context.Context, id uuid.UUID) error {
	// Start a transaction for media deletion
	tx, err := s.store.DB.BeginTx(ctx, nil)
	if err != nil {
		// Log transaction errors - these are service layer concerns
		s.logger.Error("failed to begin transaction for media deletion",
			"operation", "delete_media_by_id",
			"mediaUUID", id,
			"error", err,
		)
		return apperrors.InternalServerError(err)
	}
	defer tx.Rollback()

	// First delete the database record
	if err := s.store.Media.Delete(ctx, tx, id); err != nil {
		// Don't log here - storage layer already logged database errors
		// Just handle business logic
		return err
	}

	// Then delete the physical file
	if err := s.store.Media.DeleteFile(id); err != nil {
		// Log file system warnings - these are service layer concerns
		// but not critical since database record is already deleted
		s.logger.Warn("failed to delete media file from disk",
			"operation", "delete_media_by_id",
			"mediaUUID", id,
			"error", err,
		)
		// Continue even if file deletion fails, as the database record is already gone
	}

	// Commit the transaction
	if err := tx.Commit(); err != nil {
		// Log transaction commit errors - these are service layer concerns
		s.logger.Error("failed to commit transaction for media deletion",
			"operation", "delete_media_by_id",
			"mediaUUID", id,
			"error", err,
		)
		return apperrors.InternalServerError(err)
	}

	// Log successful business operations
	s.logger.Info("media deleted successfully",
		"mediaUUID", id,
	)

	return nil
}
