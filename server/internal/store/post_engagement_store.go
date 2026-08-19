package store

import (
	"context"
	"database/sql"
	"log/slog"
	"strings"
	"time"

	"github.com/ba-reynolds/gaggle/internal/apperrors"
	"github.com/ba-reynolds/gaggle/internal/models"
	"github.com/ba-reynolds/gaggle/internal/util"
)

type postEngagementStore struct {
	db     *sql.DB
	logger *slog.Logger
}

// Like a post
func (store *postEngagementStore) Like(ctx context.Context, tx *sql.Tx, postID, userID int) (bool, error) {
	query := `INSERT INTO post_likes (post_id, user_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`
	exec := store.db.ExecContext
	if tx != nil {
		exec = tx.ExecContext
	}
	result, err := exec(ctx, query, postID, userID)
	if err != nil {
		store.logger.Error("database insert failed", "operation", "like_post", "postID", postID, "userID", userID, "query", query, "error", err)
		return false, apperrors.InternalServerError(err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return false, apperrors.InternalServerError(err)
	}
	return affected == 1, nil
}

// Unlike a post
func (store *postEngagementStore) Unlike(ctx context.Context, tx *sql.Tx, postID, userID int) error {
	query := `DELETE FROM post_likes WHERE post_id = $1 AND user_id = $2`
	exec := store.db.ExecContext
	if tx != nil {
		exec = tx.ExecContext
	}
	_, err := exec(ctx, query, postID, userID)
	if err != nil {
		store.logger.Error("database delete failed", "operation", "unlike_post", "postID", postID, "userID", userID, "query", query, "error", err)
		return apperrors.InternalServerError(err)
	}
	return nil
}

// Repost a post (only tracks reposts, not quotes)
func (store *postEngagementStore) Repost(ctx context.Context, tx *sql.Tx, postID, userID int) (bool, error) {
	query := `INSERT INTO post_reposts (original_post_id, user_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`
	exec := store.db.ExecContext
	if tx != nil {
		exec = tx.ExecContext
	}
	result, err := exec(ctx, query, postID, userID)
	if err != nil {
		store.logger.Error("database insert failed", "operation", "repost_post", "postID", postID, "userID", userID, "query", query, "error", err)
		return false, apperrors.InternalServerError(err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return false, apperrors.InternalServerError(err)
	}
	return affected == 1, nil
}

// Unrepost a post
func (store *postEngagementStore) Unrepost(ctx context.Context, tx *sql.Tx, postID, userID int) error {
	query := `DELETE FROM post_reposts WHERE original_post_id = $1 AND user_id = $2`
	exec := store.db.ExecContext
	if tx != nil {
		exec = tx.ExecContext
	}
	_, err := exec(ctx, query, postID, userID)
	if err != nil {
		store.logger.Error("database delete failed", "operation", "unrepost_post", "postID", postID, "userID", userID, "query", query, "error", err)
		return apperrors.InternalServerError(err)
	}
	return nil
}

// Bookmark a post
func (store *postEngagementStore) Bookmark(ctx context.Context, tx *sql.Tx, postID, userID int, categoryID *int) error {
	// If categoryID is not nil, check if it exists and belongs to the user
	if categoryID != nil {
		var count int
		queryCheck := `SELECT COUNT(1) FROM bookmark_categories WHERE category_id = $1 AND user_id = $2`
		exec := store.db.QueryRowContext
		if tx != nil {
			exec = tx.QueryRowContext
		}
		err := exec(ctx, queryCheck, *categoryID, userID).Scan(&count)
		if err != nil {
			store.logger.Error("database query failed", "operation", "check_bookmark_category", "userID", userID, "categoryID", *categoryID, "query", queryCheck, "error", err)
			return apperrors.InternalServerError(err)
		}
		if count == 0 {
			return apperrors.BadRequestError("invalid bookmark category: does not exist or does not belong to user", nil)
		}
	}

	query := `INSERT INTO post_bookmarks (post_id, user_id, category_id) VALUES ($1, $2, $3)
		ON CONFLICT (post_id, user_id) DO UPDATE SET category_id = EXCLUDED.category_id, created_at = CURRENT_TIMESTAMP`
	exec := store.db.ExecContext
	if tx != nil {
		exec = tx.ExecContext
	}
	_, err := exec(ctx, query, postID, userID, categoryID)
	if err != nil {
		store.logger.Error("database upsert failed", "operation", "bookmark_post", "postID", postID, "userID", userID, "categoryID", categoryID, "query", query, "error", err)
		return apperrors.InternalServerError(err)
	}
	return nil
}

// Unbookmark a post
func (store *postEngagementStore) Unbookmark(ctx context.Context, tx *sql.Tx, postID, userID int) error {
	query := `DELETE FROM post_bookmarks WHERE post_id = $1 AND user_id = $2`
	exec := store.db.ExecContext
	if tx != nil {
		exec = tx.ExecContext
	}
	_, err := exec(ctx, query, postID, userID)
	if err != nil {
		store.logger.Error("database delete failed", "operation", "unbookmark_post", "postID", postID, "userID", userID, "query", query, "error", err)
		return apperrors.InternalServerError(err)
	}
	return nil
}

// AddView records a view for a post. A logged-in user only counts once per
// post (enforced by the partial unique index on post_views(post_id, user_id)),
// so refetches of the post detail page — including the ones engagement
// mutations trigger — do not inflate the view count.
func (store *postEngagementStore) AddView(ctx context.Context, postID int, userID *int, ipAddress, userAgent string) error {
	query := `INSERT INTO post_views (post_id, user_id, ip_address, user_agent) VALUES ($1, $2, $3, $4) ON CONFLICT DO NOTHING`
	_, err := store.db.ExecContext(ctx, query, postID, userID, ipAddress, userAgent)
	if err != nil {
		store.logger.Error("database insert failed", "operation", "add_view", "postID", postID, "userID", userID, "ipAddress", ipAddress, "userAgent", userAgent, "query", query, "error", err)
		return apperrors.InternalServerError(err)
	}
	return nil
}

// GetEngagementForPosts returns, for each post ID, the viewer-specific
// engagement state (is_liked, is_reposted, is_bookmarked and the bookmark
// category if any). Counts are populated by the caller from the Post struct.
func (store *postEngagementStore) GetEngagementForPosts(ctx context.Context, postIDs []int, viewerID int) (map[int]*models.PostEngagement, error) {
	engagements := make(map[int]*models.PostEngagement, len(postIDs))
	if len(postIDs) == 0 {
		return engagements, nil
	}

	ids := make([]any, 0, len(postIDs))
	for _, id := range postIDs {
		ids = append(ids, id)
	}

	// Liked posts
	rows, err := store.db.QueryContext(ctx,
		`SELECT post_id FROM post_likes WHERE user_id = $1 AND post_id = ANY($2)`,
		viewerID, ids)
	if err != nil {
		store.logger.Error("database query failed", "operation", "get_engagement_likes", "viewerID", viewerID, "postIDs", postIDs, "error", err)
		return nil, apperrors.InternalServerError(err)
	}
	for rows.Next() {
		var postID int
		if err := rows.Scan(&postID); err != nil {
			rows.Close()
			return nil, apperrors.InternalServerError(err)
		}
		eng, ok := engagements[postID]
		if !ok {
			eng = &models.PostEngagement{}
			engagements[postID] = eng
		}
		eng.IsLiked = true
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, apperrors.InternalServerError(err)
	}

	// Reposted posts
	rows, err = store.db.QueryContext(ctx,
		`SELECT original_post_id FROM post_reposts WHERE user_id = $1 AND original_post_id = ANY($2)`,
		viewerID, ids)
	if err != nil {
		store.logger.Error("database query failed", "operation", "get_engagement_reposts", "viewerID", viewerID, "postIDs", postIDs, "error", err)
		return nil, apperrors.InternalServerError(err)
	}
	for rows.Next() {
		var postID int
		if err := rows.Scan(&postID); err != nil {
			rows.Close()
			return nil, apperrors.InternalServerError(err)
		}
		eng, ok := engagements[postID]
		if !ok {
			eng = &models.PostEngagement{}
			engagements[postID] = eng
		}
		eng.IsReposted = true
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, apperrors.InternalServerError(err)
	}

	// Bookmarked posts (with category)
	rows, err = store.db.QueryContext(ctx,
		`SELECT pb.post_id, bc.category_id, bc.category_name
		 FROM post_bookmarks pb
		 LEFT JOIN bookmark_categories bc ON pb.category_id = bc.category_id
		 WHERE pb.user_id = $1 AND pb.post_id = ANY($2)`,
		viewerID, ids)
	if err != nil {
		store.logger.Error("database query failed", "operation", "get_engagement_bookmarks", "viewerID", viewerID, "postIDs", postIDs, "error", err)
		return nil, apperrors.InternalServerError(err)
	}
	for rows.Next() {
		var postID int
		var categoryID sql.NullInt64
		var categoryName sql.NullString
		if err := rows.Scan(&postID, &categoryID, &categoryName); err != nil {
			rows.Close()
			return nil, apperrors.InternalServerError(err)
		}
		eng, ok := engagements[postID]
		if !ok {
			eng = &models.PostEngagement{}
			engagements[postID] = eng
		}
		eng.IsBookmarked = true
		if categoryID.Valid {
			eng.BookmarkCategory = &models.BookmarkCategorySummary{
				ID:   int(categoryID.Int64),
				Name: categoryName.String,
			}
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, apperrors.InternalServerError(err)
	}

	return engagements, nil
}

// GetPostLikers returns a paginated list of users who liked a post.
func (store *postEngagementStore) GetPostLikers(ctx context.Context, postID int, limit int, cursor string) (*models.UserList, error) {
	query := `
		SELECT u.user_id, u.username, u.created_at,
			   up.display_name, up.bio, up.profile_picture_uuid, up.banner_uuid, up.birth_date, up.location, up.website,
			   up.followers_count, up.following_count
		FROM post_likes pl
		JOIN users u ON pl.user_id = u.user_id
		LEFT JOIN user_profiles up ON u.user_id = up.user_id
		WHERE pl.post_id = $1 AND u.soft_deleted = FALSE
		ORDER BY pl.created_at DESC
		LIMIT $2
	`
	args := []interface{}{postID, limit + 1}

	if cursor != "" {
		cursorData, err := util.DecodeCursor(cursor)
		if err != nil {
			store.logger.Error("failed to decode cursor", "operation", "get_post_likers", "postID", postID, "cursor", cursor, "error", err)
			return nil, apperrors.BadRequestError("invalid cursor", err)
		}
		if cursorData == nil || cursorData.Timestamp == "" {
			return nil, apperrors.BadRequestError("invalid cursor data", nil)
		}
		query = `
			SELECT u.user_id, u.username, u.created_at,
				   up.display_name, up.bio, up.profile_picture_uuid, up.banner_uuid, up.birth_date, up.location, up.website,
				   up.followers_count, up.following_count
			FROM post_likes pl
			JOIN users u ON pl.user_id = u.user_id
			LEFT JOIN user_profiles up ON u.user_id = up.user_id
			WHERE pl.post_id = $1 AND u.soft_deleted = FALSE AND pl.created_at < $2
			ORDER BY pl.created_at DESC
			LIMIT $3
		`
		args = []interface{}{postID, cursorData.Timestamp, limit + 1}
	}

	rows, err := store.db.QueryContext(ctx, query, args...)
	if err != nil {
		store.logger.Error("database query failed", "operation", "get_post_likers", "postID", postID, "query", query, "error", err)
		return nil, apperrors.InternalServerError(err)
	}
	defer rows.Close()

	var items []models.UserProfileResponse
	count := 0
	for rows.Next() {
		var u models.UserWithProfile
		if err := rows.Scan(
			&u.ID, &u.Username, &u.CreatedAt,
			&u.Profile.DisplayName, &u.Profile.Bio, &u.Profile.ProfilePictureUUID, &u.Profile.BannerUUID,
			&u.Profile.BirthDate, &u.Profile.Location, &u.Profile.Website,
			&u.Profile.FollowersCount, &u.Profile.FollowingCount,
		); err != nil {
			return nil, apperrors.InternalServerError(err)
		}
		count++
		if count > limit {
			break
		}
		items = append(items, u.ToProfileResponse())
	}
	if err := rows.Err(); err != nil {
		return nil, apperrors.InternalServerError(err)
	}

	hasMore := len(items) == limit
	var nextCursor string
	if hasMore && len(items) > 0 {
		lastID, lastTimestamp, err := store.lastLikeTimestamp(ctx, postID, limit)
		if err == nil {
			if cursorData, err := util.CreateTimestampCursor(lastID, lastTimestamp); err == nil {
				if encoded, err := util.EncodeCursor(*cursorData); err == nil {
					nextCursor = encoded
				}
			}
		}
	}

	return &models.UserList{Items: items, HasMore: hasMore, NextCursor: nextCursor}, nil
}

// GetPostReposters returns a paginated list of users who reposted a post.
func (store *postEngagementStore) GetPostReposters(ctx context.Context, postID int, limit int, cursor string) (*models.UserList, error) {
	query := `
		SELECT u.user_id, u.username, u.created_at,
			   up.display_name, up.bio, up.profile_picture_uuid, up.banner_uuid, up.birth_date, up.location, up.website,
			   up.followers_count, up.following_count
		FROM post_reposts pr
		JOIN users u ON pr.user_id = u.user_id
		LEFT JOIN user_profiles up ON u.user_id = up.user_id
		WHERE pr.original_post_id = $1 AND u.soft_deleted = FALSE
		ORDER BY pr.created_at DESC
		LIMIT $2
	`
	args := []interface{}{postID, limit + 1}

	if cursor != "" {
		cursorData, err := util.DecodeCursor(cursor)
		if err != nil {
			store.logger.Error("failed to decode cursor", "operation", "get_post_reposters", "postID", postID, "cursor", cursor, "error", err)
			return nil, apperrors.BadRequestError("invalid cursor", err)
		}
		if cursorData == nil || cursorData.Timestamp == "" {
			return nil, apperrors.BadRequestError("invalid cursor data", nil)
		}
		query = `
			SELECT u.user_id, u.username, u.created_at,
				   up.display_name, up.bio, up.profile_picture_uuid, up.banner_uuid, up.birth_date, up.location, up.website,
				   up.followers_count, up.following_count
			FROM post_reposts pr
			JOIN users u ON pr.user_id = u.user_id
			LEFT JOIN user_profiles up ON u.user_id = up.user_id
			WHERE pr.original_post_id = $1 AND u.soft_deleted = FALSE AND pr.created_at < $2
			ORDER BY pr.created_at DESC
			LIMIT $3
		`
		args = []interface{}{postID, cursorData.Timestamp, limit + 1}
	}

	rows, err := store.db.QueryContext(ctx, query, args...)
	if err != nil {
		store.logger.Error("database query failed", "operation", "get_post_reposters", "postID", postID, "query", query, "error", err)
		return nil, apperrors.InternalServerError(err)
	}
	defer rows.Close()

	var items []models.UserProfileResponse
	count := 0
	for rows.Next() {
		var u models.UserWithProfile
		if err := rows.Scan(
			&u.ID, &u.Username, &u.CreatedAt,
			&u.Profile.DisplayName, &u.Profile.Bio, &u.Profile.ProfilePictureUUID, &u.Profile.BannerUUID,
			&u.Profile.BirthDate, &u.Profile.Location, &u.Profile.Website,
			&u.Profile.FollowersCount, &u.Profile.FollowingCount,
		); err != nil {
			return nil, apperrors.InternalServerError(err)
		}
		count++
		if count > limit {
			break
		}
		items = append(items, u.ToProfileResponse())
	}
	if err := rows.Err(); err != nil {
		return nil, apperrors.InternalServerError(err)
	}

	hasMore := len(items) == limit
	var nextCursor string
	if hasMore && len(items) > 0 {
		lastID, lastTimestamp, err := store.lastRepostTimestamp(ctx, postID, limit)
		if err == nil {
			if cursorData, err := util.CreateTimestampCursor(lastID, lastTimestamp); err == nil {
				if encoded, err := util.EncodeCursor(*cursorData); err == nil {
					nextCursor = encoded
				}
			}
		}
	}

	return &models.UserList{Items: items, HasMore: hasMore, NextCursor: nextCursor}, nil
}

// lastLikeTimestamp returns the like ID and created_at of the nth most recent
// like on a post, used to build a correct next cursor.
func (store *postEngagementStore) lastLikeTimestamp(ctx context.Context, postID, offset int) (int, string, error) {
	var id int
	var ts time.Time
	err := store.db.QueryRowContext(ctx,
		`SELECT like_id, created_at FROM post_likes WHERE post_id = $1 ORDER BY created_at DESC, like_id DESC OFFSET $2 LIMIT 1`,
		postID, offset-1,
	).Scan(&id, &ts)
	if err != nil {
		return 0, "", err
	}
	return id, ts.Format(time.RFC3339), nil
}

// lastRepostTimestamp returns the repost ID and created_at of the nth most
// recent repost on a post, used to build a correct next cursor.
func (store *postEngagementStore) lastRepostTimestamp(ctx context.Context, postID, offset int) (int, string, error) {
	var id int
	var ts time.Time
	err := store.db.QueryRowContext(ctx,
		`SELECT repost_id, created_at FROM post_reposts WHERE original_post_id = $1 ORDER BY created_at DESC, repost_id DESC OFFSET $2 LIMIT 1`,
		postID, offset-1,
	).Scan(&id, &ts)
	if err != nil {
		return 0, "", err
	}
	return id, ts.Format(time.RFC3339), nil
}

// CreateBookmarkCategory creates a new bookmark category for a user
func (store *postEngagementStore) CreateBookmarkCategory(ctx context.Context, tx *sql.Tx, userID int, categoryName, color string) (*models.BookmarkCategory, error) {
	query := `INSERT INTO bookmark_categories (user_id, category_name, color) VALUES ($1, $2, $3) RETURNING category_id, user_id, category_name, color, created_at, updated_at`
	exec := store.db.QueryRowContext
	if tx != nil {
		exec = tx.QueryRowContext
	}
	var cat models.BookmarkCategory
	err := exec(ctx, query, userID, categoryName, color).Scan(
		&cat.CategoryID,
		&cat.UserID,
		&cat.CategoryName,
		&cat.Color,
		&cat.CreatedAt,
		&cat.UpdatedAt,
	)
	if err != nil {
		if strings.Contains(err.Error(), "SQLSTATE 23505") {
			return nil, apperrors.AlreadyExistsError("bookmark category name already exists for this user", err)
		}
		store.logger.Error("database insert failed", "operation", "create_bookmark_category", "userID", userID, "categoryName", categoryName, "color", color, "query", query, "error", err)
		return nil, apperrors.InternalServerError(err)
	}
	return &cat, nil
}

// ListBookmarkCategories returns all bookmark categories for a user
func (store *postEngagementStore) ListBookmarkCategories(ctx context.Context, userID int) ([]models.BookmarkCategory, error) {
	query := `
		SELECT c.category_id, c.user_id, c.category_name, c.color, c.created_at, c.updated_at,
			COALESCE(b.bookmarks_count, 0) AS bookmarks_count
		FROM bookmark_categories c
		LEFT JOIN LATERAL (
			SELECT COUNT(*) AS bookmarks_count
			FROM post_bookmarks b
			WHERE b.user_id = c.user_id AND b.category_id = c.category_id
		) b ON TRUE
		WHERE c.user_id = $1
		ORDER BY c.created_at ASC`
	rows, err := store.db.QueryContext(ctx, query, userID)
	if err != nil {
		store.logger.Error("database query failed", "operation", "list_bookmark_categories", "userID", userID, "query", query, "error", err)
		return nil, apperrors.InternalServerError(err)
	}
	defer rows.Close()

	var categories []models.BookmarkCategory
	for rows.Next() {
		var cat models.BookmarkCategory
		if err := rows.Scan(&cat.CategoryID, &cat.UserID, &cat.CategoryName, &cat.Color, &cat.CreatedAt, &cat.UpdatedAt, &cat.BookmarksCount); err != nil {
			store.logger.Error("row scan failed", "operation", "list_bookmark_categories", "userID", userID, "error", err)
			return nil, apperrors.InternalServerError(err)
		}
		categories = append(categories, cat)
	}
	if err := rows.Err(); err != nil {
		store.logger.Error("row iteration error", "operation", "list_bookmark_categories", "userID", userID, "error", err)
		return nil, apperrors.InternalServerError(err)
	}
	return categories, nil
}

// DeleteBookmarkCategory deletes a bookmark category by category_id and user_id
func (store *postEngagementStore) DeleteBookmarkCategory(ctx context.Context, tx *sql.Tx, userID, categoryID int) error {
	query := `DELETE FROM bookmark_categories WHERE category_id = $1 AND user_id = $2`
	exec := store.db.ExecContext
	if tx != nil {
		exec = tx.ExecContext
	}
	result, err := exec(ctx, query, categoryID, userID)
	if err != nil {
		store.logger.Error("database delete failed", "operation", "delete_bookmark_category", "userID", userID, "categoryID", categoryID, "query", query, "error", err)
		return apperrors.InternalServerError(err)
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		store.logger.Error("failed to get rows affected", "operation", "delete_bookmark_category", "userID", userID, "categoryID", categoryID, "error", err)
		return apperrors.InternalServerError(err)
	}
	if rowsAffected == 0 {
		return apperrors.NotFoundError("bookmark category not found", nil)
	}
	return nil
}
