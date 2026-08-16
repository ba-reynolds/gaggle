package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"os"
	"path/filepath"
	"strings"

	"github.com/ba-reynolds/vitrilium/internal/apperrors"
	"github.com/ba-reynolds/vitrilium/internal/models"
	"github.com/google/uuid"
)

type mediaStore struct {
	db       *sql.DB
	mediaDir string
	logger   *slog.Logger
}

// SaveFile writes the uploaded file bytes to disk under mediaDir named by uuid
func (store *mediaStore) SaveFile(mediaUUID uuid.UUID, file multipart.File) error {
	dstPath := filepath.Join(store.mediaDir, mediaUUID.String())
	f, err := os.Create(dstPath)
	if err != nil {
		// Log file system errors with full context
		store.logger.Error("failed to create media file on disk",
			"operation", "save_file",
			"mediaUUID", mediaUUID,
			"path", dstPath,
			"error", err,
		)
		return apperrors.InternalServerError(err)
	}
	defer f.Close()

	if _, err = io.Copy(f, file); err != nil {
		// Log file system errors with full context
		store.logger.Error("failed to write media file to disk",
			"operation", "save_file",
			"mediaUUID", mediaUUID,
			"path", dstPath,
			"error", err,
		)
		return apperrors.InternalServerError(err)
	}
	return nil
}

// DeleteFile deletes a media file from disk
func (store *mediaStore) DeleteFile(mediaUUID uuid.UUID) error {
	diskPath := filepath.Join(store.mediaDir, mediaUUID.String())
	if err := os.Remove(diskPath); err != nil && !os.IsNotExist(err) {
		// Log file system errors with full context
		store.logger.Error("failed to delete media file from disk",
			"operation", "delete_file",
			"mediaUUID", mediaUUID,
			"path", diskPath,
			"error", err,
		)
		return apperrors.InternalServerError(err)
	}
	return nil
}

// InsertMedia inserts media metadata into DB; should be called after SaveFile succeeds
func (store *mediaStore) Create(ctx context.Context, tx *sql.Tx, media *models.Media) error {
	query := `
		INSERT INTO media (media_uuid, mime_type, filename)
		VALUES ($1, $2, $3)
	`

	exec := store.db.ExecContext
	if tx != nil {
		exec = tx.ExecContext
	}

	_, err := exec(ctx, query, media.UUID, media.MimeType, media.Filename)
	if err != nil {
		// Log database insert errors with full context
		store.logger.Error("database insert failed",
			"operation", "create_media",
			"mediaUUID", media.UUID,
			"mimeType", media.MimeType,
			"filename", media.Filename,
			"query", query,
			"error", err,
		)
		return apperrors.InternalServerError(err)
	}

	return nil
}

// DeleteMedia deletes media metadata and removes file from disk
func (store *mediaStore) Delete(ctx context.Context, tx *sql.Tx, mediaUUID uuid.UUID) error {
	query := `DELETE FROM media WHERE media_uuid = $1`
	exec := store.db.ExecContext
	if tx != nil {
		exec = tx.ExecContext
	}
	result, err := exec(ctx, query, mediaUUID)
	if err != nil {
		// Log database delete errors with full context
		store.logger.Error("database delete failed",
			"operation", "delete_media",
			"mediaUUID", mediaUUID,
			"query", query,
			"error", err,
		)
		return apperrors.InternalServerError(err)
	}

	// Check if any rows were actually deleted
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		// Log errors checking rows affected
		store.logger.Error("failed to check rows affected",
			"operation", "delete_media",
			"mediaUUID", mediaUUID,
			"error", err,
		)
		return apperrors.InternalServerError(err)
	}

	if rowsAffected == 0 {
		// Log when no rows were deleted (could indicate media not found)
		store.logger.Warn("no rows deleted",
			"operation", "delete_media",
			"mediaUUID", mediaUUID,
		)
		return apperrors.NotFoundError("media not found", nil)
	}

	if err := store.DeleteFile(mediaUUID); err != nil {
		// Don't log here - DeleteFile already logs file system errors
		// Just handle business logic
		return err
	}

	return nil
}

// GetMedia retrieves media metadata by uuid
func (store *mediaStore) GetByID(ctx context.Context, mediaUUID uuid.UUID) (*models.Media, error) {
	query := `
		SELECT media_uuid, mime_type, filename, created_at
		FROM media
		WHERE media_uuid = $1
	`

	var media models.Media
	err := store.db.QueryRowContext(ctx, query, mediaUUID).Scan(
		&media.UUID,
		&media.MimeType,
		&media.Filename,
		&media.CreatedAt,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			// Don't log - this is expected behavior, not an error
			return nil, apperrors.NotFoundError(fmt.Sprintf("media with uuid %s not found", mediaUUID), err)
		}
		// Log actual database errors with full context
		store.logger.Error("database query failed",
			"operation", "get_media_by_id",
			"mediaUUID", mediaUUID,
			"query", query,
			"error", err,
		)
		return nil, apperrors.InternalServerError(err)
	}
	return &media, nil
}

// LinkMediaToPost creates an association between post and media
func (store *mediaStore) LinkMediaToPost(ctx context.Context, tx *sql.Tx, pm models.PostMedia) error {
	// Verify media exists before linking
	if _, err := store.GetByID(ctx, pm.MediaUUID); err != nil {
		// Don't log here - GetByID already logs database errors
		// Just handle business logic
		return err
	}

	query := `
		INSERT INTO post_media (post_id, media_uuid, position, alt_text)
		VALUES ($1, $2, $3, $4)
	`
	exec := store.db.ExecContext
	if tx != nil {
		exec = tx.ExecContext
	}
	_, err := exec(ctx, query, pm.PostID, pm.MediaUUID, pm.Position, pm.AltText)

	if err != nil {
		// Log database insert errors with full context
		store.logger.Error("database insert failed",
			"operation", "link_media_to_post",
			"mediaUUID", pm.MediaUUID,
			"postID", pm.PostID,
			"position", pm.Position,
			"query", query,
			"error", err,
		)
		return apperrors.InternalServerError(err)
	}

	return nil
}

// FetchPostMedia fetches media for a list of posts
// We fetch various posts at once because we will use this later one
// To fetch posts from the feed
func (store *mediaStore) FetchPostMedia(ctx context.Context, posts []*models.FullPost) error {
	if len(posts) == 0 {
		return nil
	}

	// Build a list of post IDs for the IN query
	postIDs := make([]string, len(posts))
	postMap := make(map[string]*models.FullPost)

	for i, post := range posts {
		postIDs[i] = fmt.Sprintf("%d", post.ID)
		postMap[fmt.Sprintf("%d", post.ID)] = post
		// Initialize empty media slice
		post.Media = make([]models.PostMedia, 0)
	}

	// Create placeholders for the IN query
	placeholders := make([]string, len(postIDs))
	args := make([]interface{}, len(postIDs))
	for i, id := range postIDs {
		placeholders[i] = fmt.Sprintf("$%d", i+1)
		args[i] = id
	}

	mediaQuery := fmt.Sprintf(`
		SELECT pm.post_id, pm.media_uuid, pm.position, pm.alt_text
		FROM post_media pm
		WHERE pm.post_id IN (%s)
		ORDER BY pm.post_id, pm.position
	`, strings.Join(placeholders, ","))

	mediaRows, err := store.db.QueryContext(ctx, mediaQuery, args...)
	if err != nil {
		// Log database query errors with full context
		store.logger.Error("database query failed",
			"operation", "fetch_post_media",
			"postCount", len(posts),
			"query", mediaQuery,
			"error", err,
		)
		return apperrors.InternalServerError(err)
	}
	defer mediaRows.Close()

	for mediaRows.Next() {
		var pm models.PostMedia
		var postID int

		err := mediaRows.Scan(
			&postID,
			&pm.MediaUUID,
			&pm.Position,
			&pm.AltText,
		)
		if err != nil {
			// Log row scanning errors
			store.logger.Error("failed to scan post media row",
				"operation", "fetch_post_media",
				"postID", postID,
				"error", err,
			)
			return apperrors.InternalServerError(err)
		}

		pm.PostID = postID
		if post, exists := postMap[fmt.Sprintf("%d", postID)]; exists {
			post.Media = append(post.Media, pm)
		}
	}

	if err = mediaRows.Err(); err != nil {
		// Log row iteration errors
		store.logger.Error("error after iterating post media rows",
			"operation", "fetch_post_media",
			"error", err,
		)
		return apperrors.InternalServerError(err)
	}

	return nil
}
