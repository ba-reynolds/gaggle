package store

import (
	"context"
	"database/sql"
	"log/slog"

	"github.com/ba-reynolds/gophersocial/internal/apperrors"
	"github.com/ba-reynolds/gophersocial/internal/models"
	"github.com/ba-reynolds/gophersocial/internal/postutil"
)

type hashtagStore struct {
	db     *sql.DB
	logger *slog.Logger
}

func (s *hashtagStore) SyncPost(ctx context.Context, tx *sql.Tx, postID int, content string) error {
	exec := s.db.ExecContext
	if tx != nil {
		exec = tx.ExecContext
	}
	if _, err := exec(ctx, `DELETE FROM post_hashtags WHERE post_id = $1`, postID); err != nil {
		return apperrors.InternalServerError(err)
	}
	for _, name := range postutil.ExtractHashtags(content) {
		var hashtagID int
		row := s.db.QueryRowContext
		if tx != nil {
			row = tx.QueryRowContext
		}
		if err := row(ctx, `INSERT INTO hashtags (name) VALUES ($1) ON CONFLICT (name) DO UPDATE SET name = EXCLUDED.name RETURNING hashtag_id`, name).Scan(&hashtagID); err != nil {
			return apperrors.InternalServerError(err)
		}
		if _, err := exec(ctx, `INSERT INTO post_hashtags (post_id, hashtag_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`, postID, hashtagID); err != nil {
			return apperrors.InternalServerError(err)
		}
	}
	return nil
}

func (s *hashtagStore) Trends(ctx context.Context, limit int) ([]models.Trend, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT h.name, COUNT(*)::int AS post_count
		FROM hashtags h
		JOIN post_hashtags ph ON ph.hashtag_id = h.hashtag_id
		JOIN posts p ON p.post_id = ph.post_id
		WHERE p.soft_deleted = FALSE AND p.parent_id IS NULL
		  AND p.created_at >= CURRENT_TIMESTAMP - INTERVAL '24 hours'
		GROUP BY h.name
		ORDER BY post_count DESC, h.name ASC
		LIMIT $1`, limit)
	if err != nil {
		return nil, apperrors.InternalServerError(err)
	}
	defer rows.Close()
	trends := make([]models.Trend, 0, limit)
	for rows.Next() {
		var trend models.Trend
		if err := rows.Scan(&trend.Name, &trend.Count); err != nil {
			return nil, apperrors.InternalServerError(err)
		}
		trends = append(trends, trend)
	}
	if err := rows.Err(); err != nil {
		return nil, apperrors.InternalServerError(err)
	}
	return trends, nil
}
