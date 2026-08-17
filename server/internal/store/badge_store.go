package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"log/slog"
	"time"

	"github.com/ba-reynolds/gophersocial/internal/apperrors"
	"github.com/ba-reynolds/gophersocial/internal/models"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/lib/pq"
)

type badgeStore struct {
	db     *sql.DB
	logger *slog.Logger
}

const badgeColumns = `badge_id, key, label, description, icon, kind, criteria, created_at`

func (s *badgeStore) scanBadge(row interface{ Scan(...any) error }) (models.Badge, error) {
	var b models.Badge
	var kind string
	var criteriaRaw []byte
	if err := row.Scan(&b.ID, &b.Key, &b.Label, &b.Description, &b.Icon, &kind, &criteriaRaw, &b.CreatedAt); err != nil {
		return models.Badge{}, err
	}
	b.Kind = models.BadgeKind(kind)
	if len(criteriaRaw) > 0 {
		var c models.BadgeCriteria
		if err := json.Unmarshal(criteriaRaw, &c); err != nil {
			return models.Badge{}, apperrors.InternalServerError(err)
		}
		b.Criteria = &c
	}
	return b, nil
}

func (s *badgeStore) ListCatalog(ctx context.Context) ([]models.Badge, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+badgeColumns+` FROM badges ORDER BY kind, badge_id`)
	if err != nil {
		return nil, apperrors.InternalServerError(err)
	}
	defer rows.Close()
	var out []models.Badge
	for rows.Next() {
		b, err := s.scanBadge(rows)
		if err != nil {
			return nil, apperrors.InternalServerError(err)
		}
		out = append(out, b)
	}
	if err := rows.Err(); err != nil {
		return nil, apperrors.InternalServerError(err)
	}
	return out, nil
}

// CreateBadge inserts a new admin-assigned badge definition.
func (s *badgeStore) CreateBadge(ctx context.Context, payload models.CreateBadgePayload) (*models.Badge, error) {
	criteria := []byte(nil)
	row := s.db.QueryRowContext(ctx, `INSERT INTO badges (key, label, description, icon, kind, criteria) VALUES ($1, $2, $3, $4, 'assigned', $5) RETURNING `+badgeColumns,
		payload.Key, payload.Label, payload.Description, payload.Icon, criteria)
	b, err := s.scanBadge(row)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil, apperrors.AlreadyExistsError("badge key already exists", err)
		}
		return nil, apperrors.InternalServerError(err)
	}
	return &b, nil
}

// UpdateBadge edits an admin-assigned badge definition (earned badges are immutable).
func (s *badgeStore) UpdateBadge(ctx context.Context, badgeID int, payload models.CreateBadgePayload) (*models.Badge, error) {
	row := s.db.QueryRowContext(ctx, `UPDATE badges SET key=$1, label=$2, description=$3, icon=$4 WHERE badge_id=$5 AND kind='assigned' RETURNING `+badgeColumns,
		payload.Key, payload.Label, payload.Description, payload.Icon, badgeID)
	b, err := s.scanBadge(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, apperrors.NotFoundError("assigned badge not found", err)
		}
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil, apperrors.AlreadyExistsError("badge key already exists", err)
		}
		return nil, apperrors.InternalServerError(err)
	}
	return &b, nil
}

// DeleteBadge removes an admin-assigned badge if no user still holds it.
func (s *badgeStore) DeleteBadge(ctx context.Context, badgeID int) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return apperrors.InternalServerError(err)
	}
	defer tx.Rollback()
	var kind string
	if err := tx.QueryRowContext(ctx, `SELECT kind FROM badges WHERE badge_id=$1`, badgeID).Scan(&kind); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return apperrors.NotFoundError("badge not found", err)
		}
		return apperrors.InternalServerError(err)
	}
	if kind == "earned" {
		return apperrors.BadRequestError("earned badges cannot be deleted", nil)
	}
	var grants int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM user_badges WHERE badge_id=$1`, badgeID).Scan(&grants); err != nil {
		return apperrors.InternalServerError(err)
	}
	if grants > 0 {
		return apperrors.AlreadyExistsError("badge is still assigned to users", nil)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM badges WHERE badge_id=$1`, badgeID); err != nil {
		return apperrors.InternalServerError(err)
	}
	if err := tx.Commit(); err != nil {
		return apperrors.InternalServerError(err)
	}
	return nil
}

// GrantBadge assigns an admin badge to a user. Returns AlreadyExists on
// duplicate grant, NotFound if the badge is not an assigned badge.
func (s *badgeStore) GrantBadge(ctx context.Context, userID, badgeID, grantedBy int) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return apperrors.InternalServerError(err)
	}
	defer tx.Rollback()
	var kind string
	if err := tx.QueryRowContext(ctx, `SELECT kind FROM badges WHERE badge_id=$1`, badgeID).Scan(&kind); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return apperrors.NotFoundError("badge not found", err)
		}
		return apperrors.InternalServerError(err)
	}
	if kind != "assigned" {
		return apperrors.BadRequestError("earned badges are granted automatically", nil)
	}
	var exists int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM users WHERE user_id=$1`, userID).Scan(&exists); err != nil {
		return apperrors.InternalServerError(err)
	}
	if exists == 0 {
		return apperrors.NotFoundError("user not found", nil)
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO user_badges (user_id, badge_id, granted_by) VALUES ($1, $2, $3)`, userID, badgeID, grantedBy); err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return apperrors.AlreadyExistsError("user already has this badge", err)
		}
		return apperrors.InternalServerError(err)
	}
	if err := tx.Commit(); err != nil {
		return apperrors.InternalServerError(err)
	}
	return nil
}

// RevokeBadge removes an admin-assigned badge from a user.
func (s *badgeStore) RevokeBadge(ctx context.Context, userID, badgeID int) error {
	result, err := s.db.ExecContext(ctx, `DELETE FROM user_badges WHERE user_id=$1 AND badge_id=$2`, userID, badgeID)
	if err != nil {
		return apperrors.InternalServerError(err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return apperrors.InternalServerError(err)
	}
	if affected == 0 {
		return apperrors.NotFoundError("user does not hold this badge", nil)
	}
	return nil
}

// GetBadgesForUsers returns the full badge set (admin-assigned from user_badges
// plus auto-earned computed from activity) per user ID.
func (s *badgeStore) GetBadgesForUsers(ctx context.Context, ids []int) (map[int][]models.UserBadge, error) {
	result := make(map[int][]models.UserBadge, len(ids))
	if len(ids) == 0 {
		return result, nil
	}
	catalog, err := s.ListCatalog(ctx)
	if err != nil {
		return nil, err
	}
	assigned, err := s.getAssignedByUsers(ctx, ids)
	if err != nil {
		return nil, err
	}
	metrics, err := s.getMetrics(ctx, ids)
	if err != nil {
		return nil, err
	}
	for _, id := range ids {
		var badges []models.UserBadge
		for b, grantedAt := range assigned[id] {
			ub := models.UserBadge{Badge: b, GrantedAt: &grantedAt}
			badges = append(badges, ub)
		}
		for _, b := range catalog {
			if b.Kind != models.BadgeKindEarned {
				continue
			}
			if metrics[id].Meets(b.Criteria) {
				badges = append(badges, models.UserBadge{Badge: b})
			}
		}
		result[id] = badges
	}
	return result, nil
}

// getAssignedByUsers loads admin-assigned badges for the given users.
func (s *badgeStore) getAssignedByUsers(ctx context.Context, ids []int) (map[int]map[models.Badge]time.Time, error) {
	result := make(map[int]map[models.Badge]time.Time)
	rows, err := s.db.QueryContext(ctx, `
		SELECT ub.user_id, b.`+badgeColumns+`, ub.granted_at
		FROM user_badges ub
		JOIN badges b ON b.badge_id = ub.badge_id
		WHERE ub.user_id = ANY($1)`, pq.Array(ids))
	if err != nil {
		return nil, apperrors.InternalServerError(err)
	}
	defer rows.Close()
	for rows.Next() {
		var userID int
		var grantedAt time.Time
		var b models.Badge
		var kind string
		var criteriaRaw []byte
		if err := rows.Scan(&userID, &b.ID, &b.Key, &b.Label, &b.Description, &b.Icon, &kind, &criteriaRaw, &b.CreatedAt, &grantedAt); err != nil {
			return nil, apperrors.InternalServerError(err)
		}
		b.Kind = models.BadgeKind(kind)
		if result[userID] == nil {
			result[userID] = make(map[models.Badge]time.Time)
		}
		result[userID][b] = grantedAt
	}
	if err := rows.Err(); err != nil {
		return nil, apperrors.InternalServerError(err)
	}
	return result, nil
}

// getMetrics computes the activity metrics needed to evaluate earned badges.
func (s *badgeStore) getMetrics(ctx context.Context, ids []int) (map[int]models.BadgeCriteriaMetrics, error) {
	result := make(map[int]models.BadgeCriteriaMetrics, len(ids))
	for _, id := range ids {
		result[id] = models.BadgeCriteriaMetrics{}
	}

	// Account age + follower count from the main user tables.
	rows, err := s.db.QueryContext(ctx, `
		SELECT u.user_id, EXTRACT(EPOCH FROM (CURRENT_TIMESTAMP - u.created_at)) / 86400,
		       COALESCE(up.followers_count, 0)
		FROM users u
		LEFT JOIN user_profiles up ON up.user_id = u.user_id
		WHERE u.user_id = ANY($1)`, pq.Array(ids))
	if err != nil {
		return nil, apperrors.InternalServerError(err)
	}
	for rows.Next() {
		var id int
		var ageDays float64
		var followers int
		if err := rows.Scan(&id, &ageDays, &followers); err != nil {
			rows.Close()
			return nil, apperrors.InternalServerError(err)
		}
		m := result[id]
		m.AccountAgeDays = int(ageDays)
		m.FollowersCount = followers
		result[id] = m
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, apperrors.InternalServerError(err)
	}

	// Top-level post count per author.
	rows, err = s.db.QueryContext(ctx, `
		SELECT author_id, COUNT(*) FROM posts
		WHERE author_id = ANY($1) AND soft_deleted = FALSE AND parent_id IS NULL
		GROUP BY author_id`, pq.Array(ids))
	if err != nil {
		return nil, apperrors.InternalServerError(err)
	}
	for rows.Next() {
		var id, posts int
		if err := rows.Scan(&id, &posts); err != nil {
			rows.Close()
			return nil, apperrors.InternalServerError(err)
		}
		m := result[id]
		m.PostsCount = posts
		result[id] = m
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, apperrors.InternalServerError(err)
	}

	// Likes received across an author's visible posts.
	rows, err = s.db.QueryContext(ctx, `
		SELECT author_id, COALESCE(SUM(likes_count), 0) FROM posts
		WHERE author_id = ANY($1) AND soft_deleted = FALSE
		GROUP BY author_id`, pq.Array(ids))
	if err != nil {
		return nil, apperrors.InternalServerError(err)
	}
	for rows.Next() {
		var id, likes int
		if err := rows.Scan(&id, &likes); err != nil {
			rows.Close()
			return nil, apperrors.InternalServerError(err)
		}
		m := result[id]
		m.LikesReceived = likes
		result[id] = m
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, apperrors.InternalServerError(err)
	}
	return result, nil
}
