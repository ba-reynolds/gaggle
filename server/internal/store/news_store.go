package store

import (
	"context"
	"database/sql"
	"log/slog"

	"github.com/ba-reynolds/gaggle/internal/apperrors"
	"github.com/ba-reynolds/gaggle/internal/models"
	"github.com/lib/pq"
)

type newsStore struct {
	db     *sql.DB
	logger *slog.Logger
}

func (s *newsStore) Create(ctx context.Context, tx *sql.Tx, news models.NewsLink) error {
	exec := s.db.ExecContext
	if tx != nil {
		exec = tx.ExecContext
	}
	if _, err := exec(ctx, `INSERT INTO post_news (post_id, url, title, image_url, site_name) VALUES ($1, $2, $3, $4, $5)`,
		news.PostID, news.URL, news.Title, news.ImageURL, news.SiteName); err != nil {
		return apperrors.InternalServerError(err)
	}
	return nil
}

// GetForPosts returns the news attachment for each of the given posts (missing
// post IDs are simply absent from the map). Batch version used by feed hydration.
func (s *newsStore) GetForPosts(ctx context.Context, postIDs []int) (map[int]*models.NewsLink, error) {
	result := make(map[int]*models.NewsLink)
	if len(postIDs) == 0 {
		return result, nil
	}
	rows, err := s.db.QueryContext(ctx, `SELECT post_id, url, title, image_url, site_name FROM post_news WHERE post_id = ANY($1)`, pq.Array(postIDs))
	if err != nil {
		return nil, apperrors.InternalServerError(err)
	}
	defer rows.Close()
	for rows.Next() {
		var news models.NewsLink
		if err := rows.Scan(&news.PostID, &news.URL, &news.Title, &news.ImageURL, &news.SiteName); err != nil {
			return nil, apperrors.InternalServerError(err)
		}
		result[news.PostID] = &news
	}
	if err := rows.Err(); err != nil {
		return nil, apperrors.InternalServerError(err)
	}
	return result, nil
}
