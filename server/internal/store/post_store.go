package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/ba-reynolds/gaggle/internal/apperrors"
	"github.com/ba-reynolds/gaggle/internal/models"
	"github.com/ba-reynolds/gaggle/internal/util"
	"github.com/google/uuid"
	"github.com/lib/pq"
)

type postStore struct {
	db     *sql.DB
	logger *slog.Logger
}

// nonNilIntSlice returns ids, or an empty non-nil slice, so pq.Array never
// sends NULL into the NOT NULL mentioned_user_ids column.
func nonNilIntSlice(ids []int) []int {
	if ids == nil {
		return []int{}
	}
	return ids
}

// mentionedScanner scans a postgres int[] into a []int through pq (which only
// supports scanning into []int64).
type mentionedScanner struct{ dst *[]int }

func (m *mentionedScanner) Scan(src any) error {
	var raw []int64
	if err := pq.Array(&raw).Scan(src); err != nil {
		return err
	}
	out := make([]int, len(raw))
	for i, v := range raw {
		out[i] = int(v)
	}
	*m.dst = out
	return nil
}

func scanMentionedIDs(dst *[]int) sql.Scanner {
	return &mentionedScanner{dst: dst}
}

// GetByID fetches a post by ID
func (store *postStore) GetByID(ctx context.Context, id int) (*models.Post, error) {
	query := `
		SELECT post_id, author_id, content, parent_id, soft_deleted, soft_deleted_at, created_at, updated_at,
		edited_at, is_pinned, likes_count, reposts_count, quotes_count, bookmarks_count, views_count, replies_count
		FROM posts
		WHERE post_id = $1
	`

	var post models.Post
	err := store.db.QueryRowContext(ctx, query, id).Scan(
		&post.ID,
		&post.AuthorID,
		&post.Content,
		&post.ParentID,
		&post.SoftDeleted,
		&post.SoftDeletedAt,
		&post.CreatedAt,
		&post.UpdatedAt,
		&post.EditedAt,
		&post.IsPinned,
		&post.LikesCount,
		&post.RepostsCount,
		&post.QuotesCount,
		&post.BookmarksCount,
		&post.ViewsCount,
		&post.RepliesCount,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, apperrors.NotFoundError(fmt.Sprintf("post with id %d not found", id), err)
		}
		// Log actual database errors with full context
		store.logger.Error("database query failed",
			"operation", "get_post_by_id",
			"postID", id,
			"query", query,
			"error", err,
		)
		return nil, apperrors.InternalServerError(err)
	}

	return &post, nil
}

// GetFullPostByID fetches a full post with author information
func (store *postStore) GetFullPostByID(ctx context.Context, id int) (*models.FullPost, error) {
	query := `
		SELECT
			p.post_id, p.content, p.author_id, p.parent_id, p.soft_deleted, p.soft_deleted_at, p.created_at, p.updated_at,
			p.edited_at, p.is_pinned, p.likes_count, p.reposts_count, p.quotes_count, p.bookmarks_count, p.views_count, p.replies_count,
			p.visibility, p.mentioned_user_ids,
			author.username, author_profile.display_name, author_profile.profile_picture_uuid
		FROM posts p
		JOIN users author ON p.author_id = author.user_id
		JOIN user_profiles author_profile ON author.user_id = author_profile.user_id
		WHERE p.post_id = $1 AND p.soft_deleted = FALSE
	`

	var post models.FullPost
	var profilePictureUUID uuid.UUID
	err := store.db.QueryRowContext(ctx, query, id).Scan(
		&post.ID,
		&post.Content,
		&post.AuthorID,
		&post.ParentID,
		&post.SoftDeleted,
		&post.SoftDeletedAt,
		&post.CreatedAt,
		&post.UpdatedAt,
		&post.EditedAt,
		&post.IsPinned,
		&post.LikesCount,
		&post.RepostsCount,
		&post.QuotesCount,
		&post.BookmarksCount,
		&post.ViewsCount,
		&post.RepliesCount,
		&post.Visibility,
		scanMentionedIDs(&post.MentionedUserIDs),
		&post.Author.Username,
		&post.Author.DisplayName,
		&profilePictureUUID,
	)
	post.Author = models.ToPostAuthor(post.Author.Username, post.Author.DisplayName, profilePictureUUID)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			// Don't log - this is expected behavior, not an error
			return nil, apperrors.NotFoundError(fmt.Sprintf("post with id %d not found", id), err)
		}
		// Log actual database errors with full context
		store.logger.Error("database query failed",
			"operation", "get_full_post_by_id",
			"postID", id,
			"query", query,
			"error", err,
		)
		return nil, apperrors.InternalServerError(err)
	}

	return &post, nil
}

// GetPostVisibilities returns, for each post ID, the visibility rule plus the
// author and the resolved mentioned-user set. Used by the service layer to
// enforce post-level privacy across feeds without loading full posts.
func (store *postStore) GetPostVisibilities(ctx context.Context, postIDs []int) (map[int]*models.Post, error) {
	result := make(map[int]*models.Post, len(postIDs))
	if len(postIDs) == 0 {
		return result, nil
	}
	rows, err := store.db.QueryContext(ctx, `
		SELECT post_id, author_id, visibility, mentioned_user_ids
		FROM posts
		WHERE post_id = ANY($1)
	`, pq.Array(postIDs))
	if err != nil {
		store.logger.Error("database query failed",
			"operation", "get_post_visibilities",
			"postIDs", postIDs,
			"error", err,
		)
		return nil, apperrors.InternalServerError(err)
	}
	defer rows.Close()
	for rows.Next() {
		var post models.Post
		if err := rows.Scan(&post.ID, &post.AuthorID, &post.Visibility, scanMentionedIDs(&post.MentionedUserIDs)); err != nil {
			store.logger.Error("failed to scan post visibility", "operation", "get_post_visibilities", "error", err)
			return nil, apperrors.InternalServerError(err)
		}
		result[post.ID] = &post
	}
	if err := rows.Err(); err != nil {
		store.logger.Error("row iteration failed", "operation", "get_post_visibilities", "error", err)
		return nil, apperrors.InternalServerError(err)
	}
	return result, nil
}

// GetParentInfo returns, for each reply post ID, a summary of the post it is
// replying to (the parent's ID and author). Parents that were soft-deleted or
// are otherwise missing are reported with Deleted=true and no author. Posts
// that are not replies are absent from the map.
func (store *postStore) GetParentInfo(ctx context.Context, postIDs []int) (map[int]*models.PostParentInfo, error) {
	result := make(map[int]*models.PostParentInfo, len(postIDs))
	if len(postIDs) == 0 {
		return result, nil
	}

	rows, err := store.db.QueryContext(ctx, `
		SELECT p.post_id,
		       parent.post_id,
		       parent.soft_deleted,
		       au.username,
		       ap.display_name,
		       ap.profile_picture_uuid::text
		FROM posts p
		LEFT JOIN posts parent ON parent.post_id = p.parent_id
		LEFT JOIN users au ON au.user_id = parent.author_id
		LEFT JOIN user_profiles ap ON ap.user_id = parent.author_id
		WHERE p.post_id = ANY($1) AND p.parent_id IS NOT NULL
	`, postIDs)
	if err != nil {
		store.logger.Error("database query failed", "operation", "get_parent_info", "postIDs", postIDs, "error", err)
		return nil, apperrors.InternalServerError(err)
	}
	defer rows.Close()

	for rows.Next() {
		var postID int
		var parentID sql.NullInt64
		var parentSoftDeleted sql.NullBool
		var username, displayName, profilePicture sql.NullString
		if err := rows.Scan(&postID, &parentID, &parentSoftDeleted, &username, &displayName, &profilePicture); err != nil {
			store.logger.Error("failed to scan parent info", "operation", "get_parent_info", "error", err)
			return nil, apperrors.InternalServerError(err)
		}
		if !parentID.Valid {
			continue
		}
		info := &models.PostParentInfo{ID: int(parentID.Int64)}
		if parentSoftDeleted.Valid && !parentSoftDeleted.Bool {
			parsed, err := uuid.Parse(profilePicture.String)
			if err != nil {
				parsed = uuid.Nil
			}
			author := models.ToPostAuthor(username.String, displayName.String, parsed)
			info.Author = &author
		} else {
			info.Deleted = true
		}
		result[postID] = info
	}
	if err := rows.Err(); err != nil {
		store.logger.Error("row iteration failed", "operation", "get_parent_info", "error", err)
		return nil, apperrors.InternalServerError(err)
	}
	return result, nil
}

// Create inserts a new post into the database. When post.CreatedAt is set
// (non-zero) it is honored via COALESCE so seeds can backdate posts; the app
// always leaves it zero, which maps to CURRENT_TIMESTAMP (unchanged behavior).
func (store *postStore) Create(ctx context.Context, tx *sql.Tx, post *models.Post) error {
	query := `
		INSERT INTO posts (content, author_id, parent_id, visibility, mentioned_user_ids, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, COALESCE($6, CURRENT_TIMESTAMP), CURRENT_TIMESTAMP)
		RETURNING post_id, created_at, updated_at
	`

	exec := store.db.QueryRowContext
	if tx != nil {
		exec = tx.QueryRowContext
	}

	var createdAt any
	if !post.CreatedAt.IsZero() {
		createdAt = post.CreatedAt
	}

	err := exec(ctx, query, post.Content, post.AuthorID, post.ParentID, post.Visibility, pq.Array(nonNilIntSlice(post.MentionedUserIDs)), createdAt).Scan(&post.ID, &post.CreatedAt, &post.UpdatedAt)
	if err != nil {
		// Log database insert errors with full context
		store.logger.Error("database insert failed",
			"operation", "create_post",
			"authorID", post.AuthorID,
			"parentID", post.ParentID,
			"query", query,
			"error", err,
		)
		return apperrors.InternalServerError(err)
	}

	return nil
}

// CreateQuotedPost inserts a new post with quoted_post_id set (for quoting another post)
func (store *postStore) CreateQuotedPost(ctx context.Context, tx *sql.Tx, post *models.Post) error {
	query := `
		INSERT INTO posts (content, author_id, parent_id, quoted_post_id, visibility, mentioned_user_ids, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, COALESCE($7, CURRENT_TIMESTAMP), CURRENT_TIMESTAMP)
		RETURNING post_id, created_at, updated_at
	`

	exec := store.db.QueryRowContext
	if tx != nil {
		exec = tx.QueryRowContext
	}

	var createdAt any
	if !post.CreatedAt.IsZero() {
		createdAt = post.CreatedAt
	}

	err := exec(ctx, query, post.Content, post.AuthorID, post.ParentID, post.QuotedPostID, post.Visibility, pq.Array(nonNilIntSlice(post.MentionedUserIDs)), createdAt).Scan(&post.ID, &post.CreatedAt, &post.UpdatedAt)
	if err != nil {
		store.logger.Error("database insert failed",
			"operation", "create_quoted_post",
			"authorID", post.AuthorID,
			"parentID", post.ParentID,
			"quotedPostID", post.QuotedPostID,
			"query", query,
			"error", err,
		)
		return apperrors.InternalServerError(err)
	}

	return nil
}

// DeleteByID soft-deletes a post
func (store *postStore) DeleteByID(ctx context.Context, id int) error {
	query := `
		UPDATE posts
		SET soft_deleted = TRUE, soft_deleted_at = CURRENT_TIMESTAMP
		WHERE post_id = $1 AND soft_deleted = FALSE
	`

	result, err := store.db.ExecContext(ctx, query, id)
	if err != nil {
		// Log database update errors with full context
		store.logger.Error("database update failed",
			"operation", "delete_post_by_id",
			"postID", id,
			"query", query,
			"error", err,
		)
		return apperrors.InternalServerError(err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		// Log errors checking rows affected
		store.logger.Error("failed to check rows affected",
			"operation", "delete_post_by_id",
			"postID", id,
			"error", err,
		)
		return apperrors.InternalServerError(err)
	}

	if rowsAffected == 0 {
		// Log when no rows were updated (could indicate post not found or already deleted)
		store.logger.Warn("no rows updated",
			"operation", "delete_post_by_id",
			"postID", id,
		)
		return apperrors.NotFoundError("post not found", nil)
	}

	return nil
}

func (store *postStore) Update(ctx context.Context, tx *sql.Tx, postID, authorID int, content string) (*models.Post, error) {
	query := `
		UPDATE posts
		SET content = $1, updated_at = CURRENT_TIMESTAMP, edited_at = CURRENT_TIMESTAMP
		WHERE post_id = $2 AND author_id = $3 AND soft_deleted = FALSE
		RETURNING post_id, author_id, content, parent_id, soft_deleted, soft_deleted_at, created_at, updated_at,
		          edited_at, is_pinned, likes_count, reposts_count, quotes_count, bookmarks_count, views_count, replies_count
	`
	row := store.db.QueryRowContext
	if tx != nil {
		row = tx.QueryRowContext
	}
	var post models.Post
	err := row(ctx, query, content, postID, authorID).Scan(
		&post.ID, &post.AuthorID, &post.Content, &post.ParentID, &post.SoftDeleted, &post.SoftDeletedAt,
		&post.CreatedAt, &post.UpdatedAt, &post.EditedAt, &post.IsPinned, &post.LikesCount,
		&post.RepostsCount, &post.QuotesCount, &post.BookmarksCount, &post.ViewsCount, &post.RepliesCount,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, apperrors.NotFoundError("post not found or not owned by user", err)
	}
	if err != nil {
		return nil, apperrors.InternalServerError(err)
	}
	return &post, nil
}

func (store *postStore) DeleteCascade(ctx context.Context, tx *sql.Tx, id int) error {
	query := `
		WITH RECURSIVE descendants AS (
			SELECT post_id FROM posts WHERE post_id = $1
			UNION ALL
			SELECT p.post_id FROM posts p JOIN descendants d ON p.parent_id = d.post_id
		)
		UPDATE posts SET soft_deleted = TRUE, soft_deleted_at = CURRENT_TIMESTAMP
		WHERE post_id IN (SELECT post_id FROM descendants) AND soft_deleted = FALSE
	`
	exec := store.db.ExecContext
	if tx != nil {
		exec = tx.ExecContext
	}
	result, err := exec(ctx, query, id)
	if err != nil {
		return apperrors.InternalServerError(err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return apperrors.InternalServerError(err)
	}
	if affected == 0 {
		return apperrors.NotFoundError("post not found", nil)
	}
	return nil
}

func (store *postStore) Pin(ctx context.Context, tx *sql.Tx, postID, authorID int) error {
	exec := store.db.ExecContext
	if tx != nil {
		exec = tx.ExecContext
	}
	if _, err := exec(ctx, `UPDATE posts SET is_pinned = FALSE WHERE author_id = $1 AND is_pinned = TRUE`, authorID); err != nil {
		return apperrors.InternalServerError(err)
	}
	result, err := exec(ctx, `UPDATE posts SET is_pinned = TRUE WHERE post_id = $1 AND author_id = $2 AND soft_deleted = FALSE`, postID, authorID)
	if err != nil {
		return apperrors.InternalServerError(err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return apperrors.InternalServerError(err)
	}
	if affected == 0 {
		return apperrors.NotFoundError("post not found or not owned by user", nil)
	}
	return nil
}

func (store *postStore) Unpin(ctx context.Context, tx *sql.Tx, postID, authorID int) error {
	exec := store.db.ExecContext
	if tx != nil {
		exec = tx.ExecContext
	}
	result, err := exec(ctx, `UPDATE posts SET is_pinned = FALSE WHERE post_id = $1 AND author_id = $2 AND is_pinned = TRUE`, postID, authorID)
	if err != nil {
		return apperrors.InternalServerError(err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return apperrors.InternalServerError(err)
	}
	if affected == 0 {
		return apperrors.NotFoundError("pinned post not found", nil)
	}
	return nil
}

func (store *postStore) GetPinned(ctx context.Context, authorID int) (*models.Post, error) {
	var id int
	if err := store.db.QueryRowContext(ctx, `SELECT post_id FROM posts WHERE author_id = $1 AND is_pinned = TRUE AND soft_deleted = FALSE`, authorID).Scan(&id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, apperrors.NotFoundError("pinned post not found", err)
		}
		return nil, apperrors.InternalServerError(err)
	}
	return store.GetByID(ctx, id)
}

func (store *postStore) ListEdits(ctx context.Context, postID int) (*models.PostEditHistory, error) {
	rows, err := store.db.QueryContext(ctx, `SELECT edit_id, content_before, edited_at FROM post_edits WHERE post_id = $1 ORDER BY edited_at DESC, edit_id DESC`, postID)
	if err != nil {
		return nil, apperrors.InternalServerError(err)
	}
	defer rows.Close()
	history := &models.PostEditHistory{Items: make([]models.PostEdit, 0)}
	for rows.Next() {
		var edit models.PostEdit
		if err := rows.Scan(&edit.ID, &edit.ContentBefore, &edit.EditedAt); err != nil {
			return nil, apperrors.InternalServerError(err)
		}
		history.Items = append(history.Items, edit)
	}
	if err := rows.Err(); err != nil {
		return nil, apperrors.InternalServerError(err)
	}
	return history, nil
}

func (store *postStore) CreateEdit(ctx context.Context, tx *sql.Tx, postID int, contentBefore string) error {
	exec := store.db.ExecContext
	if tx != nil {
		exec = tx.ExecContext
	}
	if _, err := exec(ctx, `INSERT INTO post_edits (post_id, content_before) VALUES ($1, $2)`, postID, contentBefore); err != nil {
		return apperrors.InternalServerError(err)
	}
	return nil
}

// GetParentChain fetches parent posts up to a specified limit
func (store *postStore) GetParentChain(ctx context.Context, postID int, limit int, cursor string) (*models.PostChain, error) {
	var query string
	var args []interface{}

	// Build the query based on whether we have a cursor
	if cursor == "" {
		// Initial query - start from the given post
		query = `
			WITH RECURSIVE post_chain AS (
				-- Base case: start with the given post
				SELECT 
					post_id, content, author_id, parent_id, soft_deleted, soft_deleted_at, created_at, updated_at,
					likes_count, reposts_count, quotes_count, bookmarks_count, views_count, replies_count,
					1 as level
				FROM posts 
				WHERE post_id = $1
				
				UNION ALL
				
				-- Recursive case: get parent posts
				SELECT 
					p.post_id, p.content, p.author_id, p.parent_id, p.soft_deleted, p.soft_deleted_at, p.created_at, p.updated_at,
					p.likes_count, p.reposts_count, p.quotes_count, p.bookmarks_count, p.views_count, p.replies_count,
					pc.level + 1
				FROM posts p
				INNER JOIN post_chain pc ON p.post_id = pc.parent_id
				WHERE pc.level < $2
			)
			SELECT 
				post_id, content, author_id, parent_id, soft_deleted, soft_deleted_at, created_at, updated_at,
				likes_count, reposts_count, quotes_count, bookmarks_count, views_count, replies_count
			FROM post_chain
			WHERE level > 1  -- Exclude the original post, only get parents
			ORDER BY level ASC
			LIMIT $3
		`
		args = []interface{}{postID, limit + 1, limit}
	} else {
		// Decode cursor to get the starting post ID
		cursorData, err := util.DecodeCursor(cursor)
		if err != nil {
			store.logger.Error("failed to decode cursor",
				"operation", "get_parent_chain",
				"postID", postID,
				"cursor", cursor,
				"error", err,
			)
			return nil, apperrors.BadRequestError("invalid cursor", err)
		}

		if cursorData == nil || cursorData.ID == nil {
			store.logger.Error("invalid cursor data",
				"operation", "get_parent_chain",
				"postID", postID,
				"cursor", cursor,
			)
			return nil, apperrors.BadRequestError("invalid cursor data", nil)
		}

		// Cursor-based query - start from the cursor post
		query = `
			WITH RECURSIVE post_chain AS (
				-- Base case: start with the cursor post
				SELECT 
					post_id, content, author_id, parent_id, soft_deleted, soft_deleted_at, created_at, updated_at,
					likes_count, reposts_count, quotes_count, bookmarks_count, views_count, replies_count,
					1 as level
				FROM posts 
				WHERE post_id = $1
				
				UNION ALL
				
				-- Recursive case: get parent posts
				SELECT 
					p.post_id, p.content, p.author_id, p.parent_id, p.soft_deleted, p.soft_deleted_at, p.created_at, p.updated_at,
					p.likes_count, p.reposts_count, p.quotes_count, p.bookmarks_count, p.views_count, p.replies_count,
					pc.level + 1
				FROM posts p
				INNER JOIN post_chain pc ON p.post_id = pc.parent_id
				WHERE pc.level < $2
			)
			SELECT 
				post_id, content, author_id, parent_id, soft_deleted, soft_deleted_at, created_at, updated_at,
				likes_count, reposts_count, quotes_count, bookmarks_count, views_count, replies_count
			FROM post_chain
			WHERE level > 1  -- Exclude the cursor post, only get parents
			ORDER BY level ASC
			LIMIT $3
		`
		args = []interface{}{cursorData.ID, limit + 1, limit}
	}

	rows, err := store.db.QueryContext(ctx, query, args...)
	if err != nil {
		// Log database query errors with full context
		store.logger.Error("database query failed",
			"operation", "get_parent_chain",
			"postID", postID,
			"limit", limit,
			"cursor", cursor,
			"query", query,
			"error", err,
		)
		return nil, apperrors.InternalServerError(err)
	}
	defer rows.Close()

	var postIDs []int
	var hasMore bool

	for rows.Next() {
		var post models.Post
		err := rows.Scan(
			&post.ID,
			&post.Content,
			&post.AuthorID,
			&post.ParentID,
			&post.SoftDeleted,
			&post.SoftDeletedAt,
			&post.CreatedAt,
			&post.UpdatedAt,
			&post.LikesCount,
			&post.RepostsCount,
			&post.QuotesCount,
			&post.BookmarksCount,
			&post.ViewsCount,
			&post.RepliesCount,
		)
		if err != nil {
			// Log row scanning errors
			store.logger.Error("failed to scan parent post",
				"operation", "get_parent_chain",
				"postID", postID,
				"error", err,
			)
			return nil, apperrors.InternalServerError(err)
		}

		// Skip soft-deleted parents — they are gone from the timeline and
		// GetFullPostByID would fail on them, breaking the whole chain (e.g.
		// viewing a reply whose parent was deleted). The reply itself still
		// carries its `parent` summary so the UI can say "replying to a
		// deleted post".
		if post.SoftDeleted {
			continue
		}

		postIDs = append(postIDs, post.ID)
	}

	if err = rows.Err(); err != nil {
		// Log row iteration errors
		store.logger.Error("error iterating over parent chain rows",
			"operation", "get_parent_chain",
			"postID", postID,
			"error", err,
		)
		return nil, apperrors.InternalServerError(err)
	}

	// Check if there are more parents
	if len(postIDs) == limit {
		// Check if the last post in our chain has a parent
		if len(postIDs) > 0 {
			lastPost, err := store.GetByID(ctx, postIDs[len(postIDs)-1])
			if err == nil && lastPost.ParentID != nil {
				hasMore = true
			}
		}
	}

	// Fetch full posts for each post ID
	fullPosts := make([]*models.FullPost, 0, len(postIDs))
	for _, id := range postIDs {
		fullPost, err := store.GetFullPostByID(ctx, id)
		if err != nil {
			// Don't log here - GetFullPostByID already logs database errors
			// Just handle business logic
			return nil, err
		}
		fullPosts = append(fullPosts, fullPost)
	}

	// Generate next cursor if there are more posts
	var nextCursor string
	if hasMore && len(postIDs) > 0 {
		cursorData, err := util.CreateIDCursor(postIDs[len(postIDs)-1])
		if err == nil {
			if encodedCursor, err := util.EncodeCursor(*cursorData); err == nil {
				nextCursor = encodedCursor
			}
		}
	}

	return &models.PostChain{
		Items:      fullPosts,
		HasMore:    hasMore,
		NextCursor: nextCursor,
	}, nil
}

// GetDescendants fetches direct replies to a post up to a specified limit
func (store *postStore) GetDescendants(ctx context.Context, postID int, limit int, cursor string) (*models.PostDescendants, error) {
	var query string
	var args []interface{}

	// Build the query based on whether we have a cursor
	if cursor == "" {
		// Initial query - get first batch of descendants
		query = `
			SELECT 
				p.post_id, p.content, p.author_id, p.parent_id, p.soft_deleted, p.soft_deleted_at, p.created_at, p.updated_at,
				p.likes_count, p.reposts_count, p.quotes_count, p.bookmarks_count, p.views_count, p.replies_count
			FROM posts p
			WHERE p.parent_id = $1
			ORDER BY p.created_at DESC
			LIMIT $2
		`
		args = []interface{}{postID, limit + 1}
	} else {
		// Decode cursor to get the timestamp
		cursorData, err := util.DecodeCursor(cursor)
		if err != nil {
			store.logger.Error("failed to decode cursor",
				"operation", "get_descendants",
				"postID", postID,
				"cursor", cursor,
				"error", err,
			)
			return nil, apperrors.BadRequestError("invalid cursor", err)
		}

		if cursorData == nil || cursorData.Timestamp == "" {
			store.logger.Error("invalid cursor data",
				"operation", "get_descendants",
				"postID", postID,
				"cursor", cursor,
			)
			return nil, apperrors.BadRequestError("invalid cursor data", nil)
		}

		// Cursor-based query - get descendants older than the cursor timestamp
		query = `
			SELECT 
				p.post_id, p.content, p.author_id, p.parent_id, p.soft_deleted, p.soft_deleted_at, p.created_at, p.updated_at,
				p.likes_count, p.reposts_count, p.quotes_count, p.bookmarks_count, p.views_count, p.replies_count
			FROM posts p
			WHERE p.parent_id = $1
			AND p.created_at < $2
			ORDER BY p.created_at DESC
			LIMIT $3
		`
		args = []interface{}{postID, cursorData.Timestamp, limit + 1}
	}

	rows, err := store.db.QueryContext(ctx, query, args...)
	if err != nil {
		// Log database query errors with full context
		store.logger.Error("database query failed",
			"operation", "get_descendants",
			"postID", postID,
			"limit", limit,
			"cursor", cursor,
			"query", query,
			"error", err,
		)
		return nil, apperrors.InternalServerError(err)
	}
	defer rows.Close()

	var postIDs []int
	var hasMore bool
	count := 0

	for rows.Next() {
		var post models.Post

		err := rows.Scan(
			&post.ID,
			&post.Content,
			&post.AuthorID,
			&post.ParentID,
			&post.SoftDeleted,
			&post.SoftDeletedAt,
			&post.CreatedAt,
			&post.UpdatedAt,
			&post.LikesCount,
			&post.RepostsCount,
			&post.QuotesCount,
			&post.BookmarksCount,
			&post.ViewsCount,
			&post.RepliesCount,
		)
		if err != nil {
			// Log row scanning errors
			store.logger.Error("failed to scan descendant post",
				"operation", "get_descendants",
				"postID", postID,
				"error", err,
			)
			return nil, apperrors.InternalServerError(err)
		}

		count++
		if count > limit {
			break
		}

		postIDs = append(postIDs, post.ID)
	}

	if err = rows.Err(); err != nil {
		// Log row iteration errors
		store.logger.Error("error iterating over descendant rows",
			"operation", "get_descendants",
			"postID", postID,
			"error", err,
		)
		return nil, apperrors.InternalServerError(err)
	}

	// Check if there are more descendants
	if len(postIDs) == limit {
		hasMore = true
	}

	// Fetch full posts for each post ID
	fullPosts := make([]*models.FullPost, 0, len(postIDs))
	for _, id := range postIDs {
		fullPost, err := store.GetFullPostByID(ctx, id)
		if err != nil {
			// Don't log here - GetFullPostByID already logs database errors
			// Just handle business logic
			return nil, err
		}
		fullPosts = append(fullPosts, fullPost)
	}

	// Generate next cursor if there are more posts
	var nextCursor string
	if hasMore && len(postIDs) > 0 {
		// Get the last post to create cursor with timestamp
		lastPost, err := store.GetByID(ctx, postIDs[len(postIDs)-1])
		if err == nil {
			cursorData, err := util.CreateTimestampCursor(lastPost.ID, lastPost.CreatedAt.Format(time.RFC3339Nano))
			if err == nil {
				if encodedCursor, err := util.EncodeCursor(*cursorData); err == nil {
					nextCursor = encodedCursor
				}
			}
		}
	}

	return &models.PostDescendants{
		Items:      fullPosts,
		HasMore:    hasMore,
		NextCursor: nextCursor,
	}, nil
}

// GetHomeFeed fetches posts from users that the authenticated user follows
func (store *postStore) GetHomeFeed(ctx context.Context, userID int, limit int, cursor string) (*models.PostFeed, error) {
	var query string
	var args []interface{}

	// Build the query based on whether we have a cursor
	if cursor == "" {
		// Initial query - get first batch of posts from followed users
		query = `
			SELECT 
				p.post_id, p.content, p.author_id, p.parent_id, p.soft_deleted, p.soft_deleted_at, p.created_at, p.updated_at,
				p.likes_count, p.reposts_count, p.quotes_count, p.bookmarks_count, p.views_count, p.replies_count
			FROM posts p
			LEFT JOIN user_relationships ur
			  ON p.author_id = ur.following_id AND ur.follower_id = $1 AND ur.relationship_type = 'follow'
			WHERE p.soft_deleted = FALSE
			  AND p.parent_id IS NULL  -- Only top-level posts (not replies)
			  AND (ur.follower_id IS NOT NULL OR p.author_id = $1)  -- followed users + own posts
			ORDER BY p.created_at DESC, p.post_id DESC
			LIMIT $2
		`
		args = []interface{}{userID, limit + 1}
	} else {
		// Decode cursor to get the timestamp
		cursorData, err := util.DecodeCursor(cursor)
		if err != nil {
			store.logger.Error("failed to decode cursor",
				"operation", "get_home_feed",
				"userID", userID,
				"cursor", cursor,
				"error", err,
			)
			return nil, apperrors.BadRequestError("invalid cursor", err)
		}

		if cursorData == nil || cursorData.Timestamp == "" {
			store.logger.Error("invalid cursor data",
				"operation", "get_home_feed",
				"userID", userID,
				"cursor", cursor,
			)
			return nil, apperrors.BadRequestError("invalid cursor data", nil)
		}

		// Cursor-based query - get posts after the cursor timestamp
		query = `
			SELECT 
				p.post_id, p.content, p.author_id, p.parent_id, p.soft_deleted, p.soft_deleted_at, p.created_at, p.updated_at,
				p.likes_count, p.reposts_count, p.quotes_count, p.bookmarks_count, p.views_count, p.replies_count
			FROM posts p
			LEFT JOIN user_relationships ur
			  ON p.author_id = ur.following_id AND ur.follower_id = $1 AND ur.relationship_type = 'follow'
			WHERE p.soft_deleted = FALSE
			  AND p.parent_id IS NULL  -- Only top-level posts (not replies)
			  AND (ur.follower_id IS NOT NULL OR p.author_id = $1)  -- followed users + own posts
			  AND (p.created_at, p.post_id) < ($2, $3)
			ORDER BY p.created_at DESC, p.post_id DESC
			LIMIT $4
		`
		args = []interface{}{userID, cursorData.Timestamp, cursorData.ID, limit + 1}
	}

	rows, err := store.db.QueryContext(ctx, query, args...)
	if err != nil {
		// Log database query errors with full context
		store.logger.Error("database query failed",
			"operation", "get_home_feed",
			"userID", userID,
			"limit", limit,
			"cursor", cursor,
			"query", query,
			"error", err,
		)
		return nil, apperrors.InternalServerError(err)
	}
	defer rows.Close()

	var postIDs []int
	var hasMore bool
	count := 0

	for rows.Next() {
		var post models.Post

		err := rows.Scan(
			&post.ID,
			&post.Content,
			&post.AuthorID,
			&post.ParentID,
			&post.SoftDeleted,
			&post.SoftDeletedAt,
			&post.CreatedAt,
			&post.UpdatedAt,
			&post.LikesCount,
			&post.RepostsCount,
			&post.QuotesCount,
			&post.BookmarksCount,
			&post.ViewsCount,
			&post.RepliesCount,
		)
		if err != nil {
			// Log row scanning errors
			store.logger.Error("failed to scan home feed post",
				"operation", "get_home_feed",
				"userID", userID,
				"error", err,
			)
			return nil, apperrors.InternalServerError(err)
		}

		count++
		if count > limit {
			break
		}

		postIDs = append(postIDs, post.ID)
	}

	if err = rows.Err(); err != nil {
		// Log row iteration errors
		store.logger.Error("error iterating over home feed rows",
			"operation", "get_home_feed",
			"userID", userID,
			"error", err,
		)
		return nil, apperrors.InternalServerError(err)
	}

	// Check if there are more posts
	if len(postIDs) == limit {
		hasMore = true
	}

	// Fetch full posts for each post ID
	fullPosts := make([]*models.FullPost, 0, len(postIDs))
	for _, id := range postIDs {
		fullPost, err := store.GetFullPostByID(ctx, id)
		if err != nil {
			// Don't log here - GetFullPostByID already logs database errors
			// Just handle business logic
			return nil, err
		}
		fullPosts = append(fullPosts, fullPost)
	}

	// Generate next cursor if there are more posts
	var nextCursor string
	if hasMore && len(postIDs) > 0 {
		// Get the last post to create cursor with timestamp
		lastPost, err := store.GetByID(ctx, postIDs[len(postIDs)-1])
		if err == nil {
			cursorData, err := util.CreateTimestampCursor(lastPost.ID, lastPost.CreatedAt.Format(time.RFC3339Nano))
			if err == nil {
				if encodedCursor, err := util.EncodeCursor(*cursorData); err == nil {
					nextCursor = encodedCursor
				}
			}
		}
	}

	return &models.PostFeed{
		Items:      fullPosts,
		HasMore:    hasMore,
		NextCursor: nextCursor,
	}, nil
}

// GetListFeed fetches top-level posts authored by users in the given list,
// newest first, using the same keyset-cursor shape as the home feed.
func (store *postStore) GetListFeed(ctx context.Context, listID int, limit int, cursor string) (*models.PostFeed, error) {
	var query string
	var args []interface{}

	if cursor == "" {
		query = `
			SELECT 
				p.post_id, p.content, p.author_id, p.parent_id, p.soft_deleted, p.soft_deleted_at, p.created_at, p.updated_at,
				p.likes_count, p.reposts_count, p.quotes_count, p.bookmarks_count, p.views_count, p.replies_count
			FROM posts p
			INNER JOIN list_members lm ON p.author_id = lm.user_id
			WHERE lm.list_id = $1
			AND p.soft_deleted = FALSE
			AND p.parent_id IS NULL  -- Only top-level posts (not replies)
			ORDER BY p.created_at DESC, p.post_id DESC
			LIMIT $2
		`
		args = []interface{}{listID, limit + 1}
	} else {
		cursorData, err := util.DecodeCursor(cursor)
		if err != nil {
			store.logger.Error("failed to decode cursor",
				"operation", "get_list_feed",
				"listID", listID,
				"cursor", cursor,
				"error", err,
			)
			return nil, apperrors.BadRequestError("invalid cursor", err)
		}
		if cursorData == nil || cursorData.Timestamp == "" {
			store.logger.Error("invalid cursor data",
				"operation", "get_list_feed",
				"listID", listID,
				"cursor", cursor,
			)
			return nil, apperrors.BadRequestError("invalid cursor data", nil)
		}

		query = `
			SELECT 
				p.post_id, p.content, p.author_id, p.parent_id, p.soft_deleted, p.soft_deleted_at, p.created_at, p.updated_at,
				p.likes_count, p.reposts_count, p.quotes_count, p.bookmarks_count, p.views_count, p.replies_count
			FROM posts p
			INNER JOIN list_members lm ON p.author_id = lm.user_id
			WHERE lm.list_id = $1
			AND p.soft_deleted = FALSE
			AND p.parent_id IS NULL  -- Only top-level posts (not replies)
			AND (p.created_at, p.post_id) < ($2, $3)
			ORDER BY p.created_at DESC, p.post_id DESC
			LIMIT $4
		`
		args = []interface{}{listID, cursorData.Timestamp, cursorData.ID, limit + 1}
	}

	rows, err := store.db.QueryContext(ctx, query, args...)
	if err != nil {
		store.logger.Error("database query failed",
			"operation", "get_list_feed",
			"listID", listID,
			"limit", limit,
			"cursor", cursor,
			"query", query,
			"error", err,
		)
		return nil, apperrors.InternalServerError(err)
	}
	defer rows.Close()

	var postIDs []int
	count := 0
	for rows.Next() {
		var post models.Post
		if err := rows.Scan(
			&post.ID,
			&post.Content,
			&post.AuthorID,
			&post.ParentID,
			&post.SoftDeleted,
			&post.SoftDeletedAt,
			&post.CreatedAt,
			&post.UpdatedAt,
			&post.LikesCount,
			&post.RepostsCount,
			&post.QuotesCount,
			&post.BookmarksCount,
			&post.ViewsCount,
			&post.RepliesCount,
		); err != nil {
			store.logger.Error("failed to scan list feed post",
				"operation", "get_list_feed",
				"listID", listID,
				"error", err,
			)
			return nil, apperrors.InternalServerError(err)
		}
		count++
		if count > limit {
			break
		}
		postIDs = append(postIDs, post.ID)
	}
	if err = rows.Err(); err != nil {
		store.logger.Error("error iterating over list feed rows",
			"operation", "get_list_feed",
			"listID", listID,
			"error", err,
		)
		return nil, apperrors.InternalServerError(err)
	}

	hasMore := len(postIDs) == limit

	fullPosts := make([]*models.FullPost, 0, len(postIDs))
	for _, id := range postIDs {
		fullPost, err := store.GetFullPostByID(ctx, id)
		if err != nil {
			return nil, err
		}
		fullPosts = append(fullPosts, fullPost)
	}

	var nextCursor string
	if hasMore && len(postIDs) > 0 {
		lastPost, err := store.GetByID(ctx, postIDs[len(postIDs)-1])
		if err == nil {
			cursorData, err := util.CreateTimestampCursor(lastPost.ID, lastPost.CreatedAt.Format(time.RFC3339Nano))
			if err == nil {
				if encodedCursor, err := util.EncodeCursor(*cursorData); err == nil {
					nextCursor = encodedCursor
				}
			}
		}
	}

	return &models.PostFeed{
		Items:      fullPosts,
		HasMore:    hasMore,
		NextCursor: nextCursor,
	}, nil
}

// GetUserFeed fetches posts made by a specific user
func (store *postStore) GetUserFeed(ctx context.Context, userID int, includeReplies bool, limit int, cursor string) (*models.PostFeed, error) {
	var query string
	var args []interface{}

	// Build the query based on whether we have a cursor and includeReplies parameter
	if cursor == "" {
		// Initial query - get first batch of posts from the user
		if includeReplies {
			// Include all posts (top-level and replies)
			query = `
				SELECT 
					p.post_id, p.content, p.author_id, p.parent_id, p.soft_deleted, p.soft_deleted_at, p.created_at, p.updated_at,
					p.likes_count, p.reposts_count, p.quotes_count, p.bookmarks_count, p.views_count, p.replies_count
				FROM posts p
				WHERE p.author_id = $1 
				AND p.soft_deleted = FALSE
				ORDER BY p.created_at DESC, p.post_id DESC
				LIMIT $2
			`
			args = []interface{}{userID, limit + 1}
		} else {
			// Only top-level posts (no replies)
			query = `
				SELECT 
					p.post_id, p.content, p.author_id, p.parent_id, p.soft_deleted, p.soft_deleted_at, p.created_at, p.updated_at,
					p.likes_count, p.reposts_count, p.quotes_count, p.bookmarks_count, p.views_count, p.replies_count
				FROM posts p
				WHERE p.author_id = $1 
				AND p.soft_deleted = FALSE
				AND p.parent_id IS NULL
				ORDER BY p.created_at DESC, p.post_id DESC
				LIMIT $2
			`
			args = []interface{}{userID, limit + 1}
		}
	} else {
		// Decode cursor to get the timestamp
		cursorData, err := util.DecodeCursor(cursor)
		if err != nil {
			store.logger.Error("failed to decode cursor",
				"operation", "get_user_feed",
				"userID", userID,
				"cursor", cursor,
				"error", err,
			)
			return nil, apperrors.BadRequestError("invalid cursor", err)
		}

		if cursorData == nil || cursorData.Timestamp == "" {
			store.logger.Error("invalid cursor data",
				"operation", "get_user_feed",
				"userID", userID,
				"cursor", cursor,
			)
			return nil, apperrors.BadRequestError("invalid cursor data", nil)
		}

		// Cursor-based query - get posts after the cursor timestamp
		if includeReplies {
			// Include all posts (top-level and replies)
			query = `
				SELECT 
					p.post_id, p.content, p.author_id, p.parent_id, p.soft_deleted, p.soft_deleted_at, p.created_at, p.updated_at,
					p.likes_count, p.reposts_count, p.quotes_count, p.bookmarks_count, p.views_count, p.replies_count
				FROM posts p
				WHERE p.author_id = $1 
				AND p.soft_deleted = FALSE
				AND (p.created_at, p.post_id) < ($2, $3)
				ORDER BY p.created_at DESC, p.post_id DESC
				LIMIT $4
			`
			args = []interface{}{userID, cursorData.Timestamp, cursorData.ID, limit + 1}
		} else {
			// Only top-level posts (no replies)
			query = `
				SELECT 
					p.post_id, p.content, p.author_id, p.parent_id, p.soft_deleted, p.soft_deleted_at, p.created_at, p.updated_at,
					p.likes_count, p.reposts_count, p.quotes_count, p.bookmarks_count, p.views_count, p.replies_count
				FROM posts p
				WHERE p.author_id = $1 
				AND p.soft_deleted = FALSE
				AND p.parent_id IS NULL
				AND (p.created_at, p.post_id) < ($2, $3)
				ORDER BY p.created_at DESC, p.post_id DESC
				LIMIT $4
			`
			args = []interface{}{userID, cursorData.Timestamp, cursorData.ID, limit + 1}
		}
	}

	rows, err := store.db.QueryContext(ctx, query, args...)
	if err != nil {
		// Log database query errors with full context
		store.logger.Error("database query failed",
			"operation", "get_user_feed",
			"userID", userID,
			"includeReplies", includeReplies,
			"limit", limit,
			"cursor", cursor,
			"query", query,
			"error", err,
		)
		return nil, apperrors.InternalServerError(err)
	}
	defer rows.Close()

	var postIDs []int
	var hasMore bool
	count := 0

	for rows.Next() {
		var post models.Post

		err := rows.Scan(
			&post.ID,
			&post.Content,
			&post.AuthorID,
			&post.ParentID,
			&post.SoftDeleted,
			&post.SoftDeletedAt,
			&post.CreatedAt,
			&post.UpdatedAt,
			&post.LikesCount,
			&post.RepostsCount,
			&post.QuotesCount,
			&post.BookmarksCount,
			&post.ViewsCount,
			&post.RepliesCount,
		)
		if err != nil {
			// Log row scanning errors
			store.logger.Error("failed to scan user feed post",
				"operation", "get_user_feed",
				"userID", userID,
				"error", err,
			)
			return nil, apperrors.InternalServerError(err)
		}

		count++
		if count > limit {
			break
		}

		postIDs = append(postIDs, post.ID)
	}

	if err = rows.Err(); err != nil {
		// Log row iteration errors
		store.logger.Error("error iterating over user feed rows",
			"operation", "get_user_feed",
			"userID", userID,
			"error", err,
		)
		return nil, apperrors.InternalServerError(err)
	}

	// Check if there are more posts
	if len(postIDs) == limit {
		hasMore = true
	}

	// Fetch full posts for each post ID
	fullPosts := make([]*models.FullPost, 0, len(postIDs))
	for _, id := range postIDs {
		fullPost, err := store.GetFullPostByID(ctx, id)
		if err != nil {
			// Don't log here - GetFullPostByID already logs database errors
			// Just handle business logic
			return nil, err
		}
		fullPosts = append(fullPosts, fullPost)
	}

	// Generate next cursor if there are more posts
	var nextCursor string
	if hasMore && len(postIDs) > 0 {
		// Get the last post to create cursor with timestamp
		lastPost, err := store.GetByID(ctx, postIDs[len(postIDs)-1])
		if err == nil {
			cursorData, err := util.CreateTimestampCursor(lastPost.ID, lastPost.CreatedAt.Format(time.RFC3339Nano))
			if err == nil {
				if encodedCursor, err := util.EncodeCursor(*cursorData); err == nil {
					nextCursor = encodedCursor
				}
			}
		}
	}

	return &models.PostFeed{
		Items:      fullPosts,
		HasMore:    hasMore,
		NextCursor: nextCursor,
	}, nil
}

// buildFeedFromPostIDs hydrates full posts for the given IDs and produces a
// paginated PostFeed response with a next cursor when more rows exist.
func (store *postStore) buildFeedFromPostIDs(ctx context.Context, postIDs []int, limit int) (*models.PostFeed, error) {
	fullPosts := make([]*models.FullPost, 0, len(postIDs))
	for _, id := range postIDs {
		fullPost, err := store.GetFullPostByID(ctx, id)
		if err != nil {
			// Don't log here - GetFullPostByID already logs database errors
			return nil, err
		}
		fullPosts = append(fullPosts, fullPost)
	}

	var nextCursor string
	if len(postIDs) == limit && len(postIDs) > 0 {
		lastPost, err := store.GetByID(ctx, postIDs[len(postIDs)-1])
		if err == nil {
			cursorData, err := util.CreateTimestampCursor(lastPost.ID, lastPost.CreatedAt.Format(time.RFC3339Nano))
			if err == nil {
				if encodedCursor, err := util.EncodeCursor(*cursorData); err == nil {
					nextCursor = encodedCursor
				}
			}
		}
	}

	return &models.PostFeed{
		Items:      fullPosts,
		HasMore:    len(postIDs) == limit,
		NextCursor: nextCursor,
	}, nil
}

// buildUserFeedQuery compiles the row-selection query for a user's feed with an
// optional mode: "all" (top-level only), "replies" (replies only), or "media"
// (posts carrying media). When cursor != "" the query is paginated.
func (store *postStore) buildUserFeedQuery(userID int, mode string, limit int, cursor string) (string, []interface{}, error) {
	compiled := `
		SELECT
			p.post_id, p.content, p.author_id, p.parent_id, p.soft_deleted, p.soft_deleted_at, p.created_at, p.updated_at,
			p.likes_count, p.reposts_count, p.quotes_count, p.bookmarks_count, p.views_count, p.replies_count
		FROM posts p
		WHERE p.author_id = $1
		AND p.soft_deleted = FALSE
	`
	args := []interface{}{userID}
	argIdx := 2

	switch mode {
	case "replies":
		compiled += "\nAND p.parent_id IS NOT NULL"
	case "media":
		compiled += "\nAND EXISTS (SELECT 1 FROM post_media pm WHERE pm.post_id = p.post_id)"
	default:
		compiled += "\nAND p.parent_id IS NULL"
	}

	if cursor != "" {
		cursorData, err := util.DecodeCursor(cursor)
		if err != nil {
			store.logger.Error("failed to decode cursor",
				"operation", "build_user_feed_query",
				"userID", userID,
				"cursor", cursor,
				"error", err,
			)
			return "", nil, apperrors.BadRequestError("invalid cursor", err)
		}
		if cursorData == nil || cursorData.Timestamp == "" {
			store.logger.Error("invalid cursor data",
				"operation", "build_user_feed_query",
				"userID", userID,
				"cursor", cursor,
			)
			return "", nil, apperrors.BadRequestError("invalid cursor data", nil)
		}
		compiled += "\nAND (p.created_at, p.post_id) < ($2, $3)"
		args = append(args, cursorData.Timestamp, cursorData.ID)
		argIdx = 4
	}

	compiled += "\nORDER BY p.created_at DESC, p.post_id DESC\nLIMIT $" + fmt.Sprintf("%d", argIdx)
	args = append(args, limit+1)

	return compiled, args, nil
}

// runUserFeedQuery executes a mode-specific user feed query and hydrates the
// post IDs into a paginated PostFeed.
func (store *postStore) runUserFeedQuery(ctx context.Context, userID int, mode string, limit int, cursor string) (*models.PostFeed, error) {
	query, args, err := store.buildUserFeedQuery(userID, mode, limit, cursor)
	if err != nil {
		return nil, err
	}

	rows, err := store.db.QueryContext(ctx, query, args...)
	if err != nil {
		store.logger.Error("database query failed",
			"operation", "user_feed_query",
			"userID", userID,
			"mode", mode,
			"limit", limit,
			"cursor", cursor,
			"query", query,
			"error", err,
		)
		return nil, apperrors.InternalServerError(err)
	}
	defer rows.Close()

	var postIDs []int
	count := 0
	for rows.Next() {
		var post models.Post
		err := rows.Scan(
			&post.ID,
			&post.Content,
			&post.AuthorID,
			&post.ParentID,
			&post.SoftDeleted,
			&post.SoftDeletedAt,
			&post.CreatedAt,
			&post.UpdatedAt,
			&post.LikesCount,
			&post.RepostsCount,
			&post.QuotesCount,
			&post.BookmarksCount,
			&post.ViewsCount,
			&post.RepliesCount,
		)
		if err != nil {
			store.logger.Error("failed to scan user feed post",
				"operation", "user_feed_query",
				"userID", userID,
				"mode", mode,
				"error", err,
			)
			return nil, apperrors.InternalServerError(err)
		}

		count++
		if count > limit {
			break
		}
		postIDs = append(postIDs, post.ID)
	}
	if err := rows.Err(); err != nil {
		store.logger.Error("error iterating over user feed rows",
			"operation", "user_feed_query",
			"userID", userID,
			"mode", mode,
			"error", err,
		)
		return nil, apperrors.InternalServerError(err)
	}

	return store.buildFeedFromPostIDs(ctx, postIDs, limit)
}

// GetUserReplies fetches the replies (posts with a parent) made by a specific user
func (store *postStore) GetUserReplies(ctx context.Context, userID int, limit int, cursor string) (*models.PostFeed, error) {
	return store.runUserFeedQuery(ctx, userID, "replies", limit, cursor)
}

// GetUserMediaFeed fetches posts with media made by a specific user
func (store *postStore) GetUserMediaFeed(ctx context.Context, userID int, limit int, cursor string) (*models.PostFeed, error) {
	return store.runUserFeedQuery(ctx, userID, "media", limit, cursor)
}
func (store *postStore) GetBookmarkedPostsFeed(ctx context.Context, userID int, categoryIDs []int, limit int, cursor string) (*models.PostFeed, error) {
	var query string
	var args []interface{}
	argIdx := 1

	baseQuery := `
		SELECT p.post_id, pb.created_at
		FROM post_bookmarks pb
		JOIN posts p ON pb.post_id = p.post_id
		WHERE pb.user_id = $1 AND p.soft_deleted = FALSE`
	args = append(args, userID)
	argIdx++

	if len(categoryIDs) > 0 {
		placeholders := make([]string, len(categoryIDs))
		for i, id := range categoryIDs {
			placeholders[i] = fmt.Sprintf("$%d", argIdx)
			args = append(args, id)
			argIdx++
		}
		baseQuery += fmt.Sprintf(" AND pb.category_id IN (%s)", strings.Join(placeholders, ","))
	}

	if cursor != "" {
		cursorData, err := util.DecodeCursor(cursor)
		if err != nil {
			store.logger.Error("failed to decode cursor", "operation", "get_bookmarked_posts_feed", "userID", userID, "cursor", cursor, "error", err)
			return nil, apperrors.BadRequestError("invalid cursor", err)
		}
		if cursorData == nil || cursorData.Timestamp == "" {
			store.logger.Error("invalid cursor data", "operation", "get_bookmarked_posts_feed", "userID", userID, "cursor", cursor)
			return nil, apperrors.BadRequestError("invalid cursor data", nil)
		}
		baseQuery += fmt.Sprintf(" AND pb.created_at < $%d", argIdx)
		args = append(args, cursorData.Timestamp)
		argIdx++
	}

	baseQuery += " ORDER BY pb.created_at DESC LIMIT $" + fmt.Sprint(argIdx)
	args = append(args, limit+1)

	query = baseQuery

	rows, err := store.db.QueryContext(ctx, query, args...)
	if err != nil {
		store.logger.Error("database query failed", "operation", "get_bookmarked_posts_feed", "userID", userID, "query", query, "error", err)
		return nil, apperrors.InternalServerError(err)
	}
	defer rows.Close()

	var postIDs []int
	var lastCreatedAt time.Time
	count := 0
	for rows.Next() {
		var postID int
		var bookmarkCreatedAt time.Time
		if err := rows.Scan(&postID, &bookmarkCreatedAt); err != nil {
			store.logger.Error("failed to scan bookmarked post id", "operation", "get_bookmarked_posts_feed", "userID", userID, "error", err)
			return nil, apperrors.InternalServerError(err)
		}
		count++
		if count > limit {
			break
		}
		postIDs = append(postIDs, postID)
		lastCreatedAt = bookmarkCreatedAt
	}
	if err := rows.Err(); err != nil {
		store.logger.Error("row iteration error", "operation", "get_bookmarked_posts_feed", "userID", userID, "error", err)
		return nil, apperrors.InternalServerError(err)
	}

	hasMore := len(postIDs) == limit

	fullPosts := make([]*models.FullPost, 0, len(postIDs))
	for _, id := range postIDs {
		fullPost, err := store.GetFullPostByID(ctx, id)
		if err != nil {
			return nil, err
		}
		fullPosts = append(fullPosts, fullPost)
	}

	var nextCursor string
	if hasMore && len(postIDs) > 0 {
		cursorData, err := util.CreateTimestampCursor(postIDs[len(postIDs)-1], lastCreatedAt.Format(time.RFC3339))
		if err == nil {
			if encodedCursor, err := util.EncodeCursor(*cursorData); err == nil {
				nextCursor = encodedCursor
			}
		}
	}

	return &models.PostFeed{
		Items:      fullPosts,
		HasMore:    hasMore,
		NextCursor: nextCursor,
	}, nil
}

// GetLikedPostsFeed fetches posts liked by a user, paginated by created_at DESC
func (store *postStore) GetLikedPostsFeed(ctx context.Context, userID int, limit int, cursor string) (*models.PostFeed, error) {
	var query string
	var args []interface{}
	argIdx := 1

	baseQuery := `
		SELECT p.post_id, pl.created_at
		FROM post_likes pl
		JOIN posts p ON pl.post_id = p.post_id
		WHERE pl.user_id = $1 AND p.soft_deleted = FALSE`
	args = append(args, userID)
	argIdx++

	if cursor != "" {
		cursorData, err := util.DecodeCursor(cursor)
		if err != nil {
			store.logger.Error("failed to decode cursor", "operation", "get_liked_posts_feed", "userID", userID, "cursor", cursor, "error", err)
			return nil, apperrors.BadRequestError("invalid cursor", err)
		}
		if cursorData == nil || cursorData.Timestamp == "" {
			store.logger.Error("invalid cursor data", "operation", "get_liked_posts_feed", "userID", userID, "cursor", cursor)
			return nil, apperrors.BadRequestError("invalid cursor data", nil)
		}
		baseQuery += fmt.Sprintf(" AND pl.created_at < $%d", argIdx)
		args = append(args, cursorData.Timestamp)
		argIdx++
	}

	baseQuery += " ORDER BY pl.created_at DESC LIMIT $" + fmt.Sprint(argIdx)
	args = append(args, limit+1)

	query = baseQuery

	rows, err := store.db.QueryContext(ctx, query, args...)
	if err != nil {
		store.logger.Error("database query failed", "operation", "get_liked_posts_feed", "userID", userID, "query", query, "error", err)
		return nil, apperrors.InternalServerError(err)
	}
	defer rows.Close()

	var postIDs []int
	var lastCreatedAt time.Time
	count := 0
	for rows.Next() {
		var postID int
		var likeCreatedAt time.Time
		if err := rows.Scan(&postID, &likeCreatedAt); err != nil {
			store.logger.Error("failed to scan liked post id", "operation", "get_liked_posts_feed", "userID", userID, "error", err)
			return nil, apperrors.InternalServerError(err)
		}
		count++
		if count > limit {
			break
		}
		postIDs = append(postIDs, postID)
		lastCreatedAt = likeCreatedAt
	}
	if err := rows.Err(); err != nil {
		store.logger.Error("row iteration error", "operation", "get_liked_posts_feed", "userID", userID, "error", err)
		return nil, apperrors.InternalServerError(err)
	}

	hasMore := len(postIDs) == limit

	fullPosts := make([]*models.FullPost, 0, len(postIDs))
	for _, id := range postIDs {
		fullPost, err := store.GetFullPostByID(ctx, id)
		if err != nil {
			return nil, err
		}
		fullPosts = append(fullPosts, fullPost)
	}

	var nextCursor string
	if hasMore && len(postIDs) > 0 {
		cursorData, err := util.CreateTimestampCursor(postIDs[len(postIDs)-1], lastCreatedAt.Format(time.RFC3339))
		if err == nil {
			if encodedCursor, err := util.EncodeCursor(*cursorData); err == nil {
				nextCursor = encodedCursor
			}
		}
	}

	return &models.PostFeed{
		Items:      fullPosts,
		HasMore:    hasMore,
		NextCursor: nextCursor,
	}, nil
}

// GetQuotesFeed fetches a paginated feed of posts that quote a given post.
func (store *postStore) GetQuotesFeed(ctx context.Context, postID int, limit int, cursor string) (*models.PostFeed, error) {
	var query string
	var args []interface{}

	if cursor == "" {
		query = `
			SELECT p.post_id
			FROM posts p
			WHERE p.quoted_post_id = $1 AND p.soft_deleted = FALSE
			ORDER BY p.created_at DESC
			LIMIT $2
		`
		args = []interface{}{postID, limit + 1}
	} else {
		cursorData, err := util.DecodeCursor(cursor)
		if err != nil {
			store.logger.Error("failed to decode cursor", "operation", "get_quotes_feed", "postID", postID, "cursor", cursor, "error", err)
			return nil, apperrors.BadRequestError("invalid cursor", err)
		}
		if cursorData == nil || cursorData.Timestamp == "" {
			return nil, apperrors.BadRequestError("invalid cursor data", nil)
		}
		query = `
			SELECT p.post_id
			FROM posts p
			WHERE p.quoted_post_id = $1 AND p.soft_deleted = FALSE AND p.created_at < $2
			ORDER BY p.created_at DESC
			LIMIT $3
		`
		args = []interface{}{postID, cursorData.Timestamp, limit + 1}
	}

	rows, err := store.db.QueryContext(ctx, query, args...)
	if err != nil {
		store.logger.Error("database query failed", "operation", "get_quotes_feed", "postID", postID, "query", query, "error", err)
		return nil, apperrors.InternalServerError(err)
	}
	defer rows.Close()

	var postIDs []int
	count := 0
	for rows.Next() {
		var postID int
		if err := rows.Scan(&postID); err != nil {
			store.logger.Error("failed to scan quote post id", "operation", "get_quotes_feed", "postID", postID, "error", err)
			return nil, apperrors.InternalServerError(err)
		}
		count++
		if count > limit {
			break
		}
		postIDs = append(postIDs, postID)
	}
	if err := rows.Err(); err != nil {
		return nil, apperrors.InternalServerError(err)
	}

	hasMore := len(postIDs) == limit

	fullPosts := make([]*models.FullPost, 0, len(postIDs))
	for _, id := range postIDs {
		fullPost, err := store.GetFullPostByID(ctx, id)
		if err != nil {
			return nil, err
		}
		fullPosts = append(fullPosts, fullPost)
	}

	var nextCursor string
	if hasMore && len(postIDs) > 0 {
		lastPost, err := store.GetByID(ctx, postIDs[len(postIDs)-1])
		if err == nil {
			cursorData, err := util.CreateTimestampCursor(lastPost.ID, lastPost.CreatedAt.Format(time.RFC3339))
			if err == nil {
				if encodedCursor, err := util.EncodeCursor(*cursorData); err == nil {
					nextCursor = encodedCursor
				}
			}
		}
	}

	return &models.PostFeed{
		Items:      fullPosts,
		HasMore:    hasMore,
		NextCursor: nextCursor,
	}, nil
}

func (store *postStore) Search(ctx context.Context, query string, filters models.PostSearchFilters, limit int, cursor string) (*models.PostFeed, error) {
	filters.Hashtag = strings.ToLower(strings.TrimPrefix(strings.TrimSpace(filters.Hashtag), "#"))
	query = strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(query)
	clauses := []string{`p.content ILIKE '%' || $1 || '%' ESCAPE '\'`}
	args := []any{query}
	if filters.From != "" {
		clauses = append(clauses, `EXISTS (SELECT 1 FROM users u WHERE u.username = $`+strconv.Itoa(len(args)+1)+` AND u.user_id = p.author_id)`)
		args = append(args, filters.From)
	}
	if filters.Hashtag != "" {
		clauses = append(clauses, `EXISTS (SELECT 1 FROM post_hashtags ph JOIN hashtags h ON h.hashtag_id = ph.hashtag_id WHERE ph.post_id = p.post_id AND h.name = $`+strconv.Itoa(len(args)+1)+`)`)
		args = append(args, filters.Hashtag)
	}
	if filters.HasMedia {
		clauses = append(clauses, `EXISTS (SELECT 1 FROM post_media pm WHERE pm.post_id = p.post_id)`)
	}
	if filters.MinLikes > 0 {
		clauses = append(clauses, `p.likes_count >= $`+strconv.Itoa(len(args)+1))
		args = append(args, filters.MinLikes)
	}
	if filters.Since != nil {
		clauses = append(clauses, `p.created_at >= $`+strconv.Itoa(len(args)+1))
		args = append(args, *filters.Since)
	}
	if filters.Until != nil {
		clauses = append(clauses, `p.created_at < $`+strconv.Itoa(len(args)+1))
		args = append(args, *filters.Until)
	}

	base := "p.soft_deleted = FALSE"
	if !filters.IncludeReplies {
		base += " AND p.parent_id IS NULL"
	}
	return store.listDiscoverablePosts(ctx, "FROM posts p WHERE "+base+" AND "+strings.Join(clauses, " AND "), args, limit, cursor)
}

func (store *postStore) ListByHashtag(ctx context.Context, name string, limit int, cursor string) (*models.PostFeed, error) {
	return store.listDiscoverablePosts(ctx, `
		FROM posts p
		JOIN post_hashtags ph ON ph.post_id = p.post_id
		JOIN hashtags h ON h.hashtag_id = ph.hashtag_id
		WHERE p.soft_deleted = FALSE AND p.parent_id IS NULL AND h.name = $1
	`, []any{name}, limit, cursor)
}

func (store *postStore) ListMentionedBy(ctx context.Context, userID int, limit int, cursor string) (*models.PostFeed, error) {
	return store.listDiscoverablePosts(ctx, `
		FROM posts p
		JOIN post_mentions pm ON pm.post_id = p.post_id
		WHERE p.soft_deleted = FALSE AND p.parent_id IS NULL AND pm.user_id = $1
	`, []any{userID}, limit, cursor)
}

func (store *postStore) listDiscoverablePosts(ctx context.Context, filters string, args []any, limit int, cursor string) (*models.PostFeed, error) {
	query := `SELECT p.post_id, p.created_at ` + filters
	if cursor != "" {
		decoded, err := util.DecodeCursor(cursor)
		if err != nil {
			return nil, apperrors.BadRequestError("invalid search cursor", err)
		}
		id, ok := decoded.ID.(float64)
		if !ok {
			return nil, apperrors.BadRequestError("invalid search cursor", nil)
		}
		timestamp, err := time.Parse(time.RFC3339Nano, decoded.Timestamp)
		if err != nil {
			return nil, apperrors.BadRequestError("invalid search cursor", err)
		}
		query += " AND (p.created_at, p.post_id) < ($" + strconv.Itoa(len(args)+1) + ", $" + strconv.Itoa(len(args)+2) + ")"
		args = append(args, timestamp, int(id))
	}
	query += " ORDER BY p.created_at DESC, p.post_id DESC LIMIT $" + strconv.Itoa(len(args)+1)
	args = append(args, limit+1)
	rows, err := store.db.QueryContext(ctx, query, args...)
	if err != nil {
		store.logger.Error("search post query failed", "query", query, "args", args, "error", err)
		return nil, apperrors.InternalServerError(err)
	}
	defer rows.Close()

	items := make([]*models.FullPost, 0, limit)
	for rows.Next() {
		var id int
		var createdAt time.Time
		if err := rows.Scan(&id, &createdAt); err != nil {
			return nil, apperrors.InternalServerError(err)
		}
		items = append(items, &models.FullPost{Post: models.Post{ID: id, CreatedAt: createdAt}})
	}
	if err := rows.Err(); err != nil {
		return nil, apperrors.InternalServerError(err)
	}
	feed := &models.PostFeed{Items: items}
	if len(items) > limit {
		feed.HasMore = true
		feed.Items = items[:limit]
		last := feed.Items[len(feed.Items)-1]
		encoded, err := util.EncodeCursor(util.PaginationCursor{ID: last.ID, Timestamp: last.CreatedAt.Format(time.RFC3339Nano), Order: "desc"})
		if err != nil {
			return nil, apperrors.InternalServerError(err)
		}
		feed.NextCursor = encoded
	}
	return feed, nil
}
