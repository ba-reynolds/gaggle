package store

import (
	"context"
	"database/sql"
	"log/slog"
	"net"
	"time"

	"github.com/ba-reynolds/gaggle/internal/apperrors"
	"github.com/ba-reynolds/gaggle/internal/metrics"
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

// InsertHostSample persists one background host-stats sample. Written by the
// metrics Sampler, not by request handlers.
func (s *metricsStore) InsertHostSample(ctx context.Context, h *metrics.HostStats) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO host_metrics_samples
		(cpu_percent, mem_total, mem_used, mem_percent, load1, load5, load15, uptime_seconds, disk_total, disk_used, disk_percent)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`,
		h.CPUPercent, h.MemTotalBytes, h.MemUsedBytes, h.MemPercent,
		h.Load1, h.Load5, h.Load15, h.UptimeSeconds,
		h.DiskTotalBytes, h.DiskUsedBytes, h.DiskPercent)
	if err != nil {
		return apperrors.InternalServerError(err)
	}
	return nil
}

// historyBucket returns the downsampling bucket and look-back window for a
// range. Short ranges keep minute resolution; longer ones aggregate so the
// response stays small.
func historyBucket(r models.HistoryRange) (bucket, window string, ok bool) {
	switch r {
	case models.History24h:
		return "1 minute", "24 hours", true
	case models.History7d:
		return "5 minutes", "7 days", true
	case models.History30d:
		return "15 minutes", "30 days", true
	}
	return "", "", false
}

// HostSeries returns host-metric points over the given range, aggregated into
// fixed time buckets (AVG per bucket). Buckets that had no samples are simply
// absent from the result.
func (s *metricsStore) HostSeries(ctx context.Context, r models.HistoryRange) ([]models.HostSamplePoint, error) {
	bucket, window, ok := historyBucket(r)
	if !ok {
		return nil, apperrors.BadRequestError("invalid history range", nil)
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT date_bin($1::interval, created_at, now()),
		       AVG(cpu_percent), AVG(mem_percent), AVG(disk_percent), AVG(load1)
		FROM host_metrics_samples
		WHERE created_at >= now() - $2::interval
		GROUP BY 1
		ORDER BY 1`, bucket, window)
	if err != nil {
		return nil, apperrors.InternalServerError(err)
	}
	defer rows.Close()
	result := []models.HostSamplePoint{}
	for rows.Next() {
		var p models.HostSamplePoint
		if err := rows.Scan(&p.Timestamp, &p.CPUPercent, &p.MemPercent, &p.DiskPercent, &p.Load1); err != nil {
			return nil, apperrors.InternalServerError(err)
		}
		result = append(result, p)
	}
	if err := rows.Err(); err != nil {
		return nil, apperrors.InternalServerError(err)
	}
	return result, nil
}

// PrunePageViews deletes view rows older than `cutoff`. Called by the sampler
// on an hourly loop so the analytics table can't grow unbounded.
func (s *metricsStore) PrunePageViews(ctx context.Context, cutoff time.Time) error {
	if _, err := s.db.ExecContext(ctx, `DELETE FROM page_views WHERE created_at < $1`, cutoff); err != nil {
		return apperrors.InternalServerError(err)
	}
	return nil
}

// PruneHostSamples deletes host sample rows older than `cutoff`.
func (s *metricsStore) PruneHostSamples(ctx context.Context, cutoff time.Time) error {
	if _, err := s.db.ExecContext(ctx, `DELETE FROM host_metrics_samples WHERE created_at < $1`, cutoff); err != nil {
		return apperrors.InternalServerError(err)
	}
	return nil
}
