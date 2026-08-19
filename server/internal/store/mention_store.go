package store

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"

	"github.com/ba-reynolds/gaggle/internal/apperrors"
	"github.com/ba-reynolds/gaggle/internal/postutil"
)

type mentionStore struct {
	db     *sql.DB
	logger *slog.Logger
}

// SyncPost replaces the stored mentions of a post with the @usernames found in
// content. Only usernames that resolve to a real, non-deleted user are kept
// (matched case-insensitively, since usernames are unique on LOWER(username)).
func (s *mentionStore) SyncPost(ctx context.Context, tx *sql.Tx, postID int, content string) error {
	exec := s.db.ExecContext
	if tx != nil {
		exec = tx.ExecContext
	}
	if _, err := exec(ctx, `DELETE FROM post_mentions WHERE post_id = $1`, postID); err != nil {
		return apperrors.InternalServerError(err)
	}
	for _, name := range postutil.ExtractMentions(content) {
		row := s.db.QueryRowContext
		if tx != nil {
			row = tx.QueryRowContext
		}
		var userID int
		if err := row(ctx, `SELECT user_id FROM users WHERE LOWER(username) = LOWER($1) AND soft_deleted = FALSE`, name).Scan(&userID); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				continue
			}
			s.logger.Error("mention lookup failed",
				"operation", "sync_post_mentions",
				"postID", postID,
				"username", name,
				"error", err,
			)
			return apperrors.InternalServerError(err)
		}
		if _, err := exec(ctx, `INSERT INTO post_mentions (post_id, user_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`, postID, userID); err != nil {
			return apperrors.InternalServerError(err)
		}
	}
	return nil
}