package store

import (
	"context"
	"database/sql"
	"log/slog"
	"net"
	"time"

	"github.com/ba-reynolds/gaggle/internal/apperrors"
	"github.com/ba-reynolds/gaggle/internal/models"
)

type metricsStore struct {
	db     *sql.DB
	logger *slog.Logger
}

// Record persists one page view / API request. Callers treat failure as
// best-effort — the request must not fail because analytics could not be
// written.
func (s *metricsStore) Record(ctx context.Context, userID *int, ip, method, path string, status int) error {
	var userIDArg any
	if userID != nil {
		userIDArg = *userID
	}
	var ipArg any
	if parsed := net.ParseIP(ip); parsed != nil {
		ipArg = ip
	}
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO page_views (user_id, ip, method, path, status) VALUES ($1, $2, $3, $4, $5)`,
		userIDArg, ipArg, method, path, status)
	if err != nil {
		return apperrors.InternalServerError(err)
	}
	return nil
}

// AppStats aggregates platform-wide counters.
func (s *metricsStore) AppStats(ctx context.Context) (*models.AppStats, error) {
	stats := models.AppStats{}
	var err error
	queries := []struct {
		dest *int
		sql  string
	}{
		{&stats.Users, `SELECT COUNT(*) FROM users WHERE soft_deleted = FALSE`},
		{&stats.Posts, `SELECT COUNT(*) FROM posts WHERE soft_deleted = FALSE AND parent_id IS NULL`},
		{&stats.Likes, `SELECT COUNT(*) FROM post_likes`},
		{&stats.Messages, `SELECT COUNT(*) FROM messages`},
		{&stats.ViewsTotal, `SELECT COUNT(*) FROM page_views`},
		{&stats.Signups24h, `SELECT COUNT(*) FROM users WHERE created_at >= now() - interval '24 hours'`},
	}
	for _, q := range queries {
		if qerr := s.db.QueryRowContext(ctx, q.sql).Scan(q.dest); qerr != nil {
			err = qerr
			break
		}
	}
	if err != nil {
		return nil, apperrors.InternalServerError(err)
	}
	return &stats, nil
}

// ViewsByDay returns view counts per day over the last `days` days, oldest
// first, using UTC day boundaries.
func (s *metricsStore) ViewsByDay(ctx context.Context, days int) ([]models.DayViewCount, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT to_char(date_trunc('day', created_at) AT TIME ZONE 'UTC', 'YYYY-MM-DD'), COUNT(*)
		FROM page_views
		WHERE created_at >= now() - ($1 * interval '1 day')
		GROUP BY 1
		ORDER BY 1`, days)
	if err != nil {
		return nil, apperrors.InternalServerError(err)
	}
	defer rows.Close()
	result := []models.DayViewCount{}
	for rows.Next() {
		var dc models.DayViewCount
		if err := rows.Scan(&dc.Day, &dc.Views); err != nil {
			return nil, apperrors.InternalServerError(err)
		}
		result = append(result, dc)
	}
	if err := rows.Err(); err != nil {
		return nil, apperrors.InternalServerError(err)
	}
	return result, nil
}

// RequestsLastMinute returns the number of recorded views in the last 60s.
func (s *metricsStore) RequestsLastMinute(ctx context.Context) (int, error) {
	var count int
	err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM page_views WHERE created_at >= now() - interval '60 seconds'`).Scan(&count)
	if err != nil {
		return 0, apperrors.InternalServerError(err)
	}
	return count, nil
}

// DistinctUsersActiveSince counts unique logged-in users with a page view at
// or after `since`.
func (s *metricsStore) DistinctUsersActiveSince(ctx context.Context, since time.Time) (int, error) {
	var count int
	err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(DISTINCT user_id) FROM page_views WHERE user_id IS NOT NULL AND created_at >= $1`,
		since).Scan(&count)
	if err != nil {
		return 0, apperrors.InternalServerError(err)
	}
	return count, nil
}
