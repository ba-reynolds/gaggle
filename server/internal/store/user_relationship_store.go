package store

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"

	"github.com/ba-reynolds/gophersocial/internal/apperrors"
	"github.com/ba-reynolds/gophersocial/internal/models"
	"github.com/ba-reynolds/gophersocial/internal/util"
	"github.com/lib/pq"
)

type userRelationshipStore struct {
	db     *sql.DB
	logger *slog.Logger
}

// Create creates a new user relationship
func (store *userRelationshipStore) Create(ctx context.Context, tx *sql.Tx, relationship *models.UserRelationship) error {
	query := `
		INSERT INTO user_relationships (follower_id, following_id, relationship_type)
		VALUES ($1, $2, $3)
		RETURNING relationship_id, created_at, updated_at
	`

	exec := store.db.QueryRowContext
	if tx != nil {
		exec = tx.QueryRowContext
	}

	err := exec(ctx, query, relationship.FollowerID, relationship.FollowingID, relationship.RelationshipType).Scan(
		&relationship.RelationshipID,
		&relationship.CreatedAt,
		&relationship.UpdatedAt,
	)
	if err != nil {
		var pqErr *pq.Error
		if errors.As(err, &pqErr) {
			if pqErr.Code == "23505" { // unique_violation
				// Don't log - this is expected business logic, not an error
				return apperrors.AlreadyExistsError("relationship already exists between these users", err)
			}
			if pqErr.Code == "23514" { // check_violation
				// Don't log - this is expected business logic, not an error
				return apperrors.BadRequestError("invalid relationship type", err)
			}
		}
		// Log unexpected database errors with full context
		store.logger.Error("database insert failed",
			"operation", "create_user_relationship",
			"follower_id", relationship.FollowerID,
			"following_id", relationship.FollowingID,
			"relationship_type", relationship.RelationshipType,
			"query", query,
			"error", err,
			"pq_code", func() string {
				if pqErr != nil {
					return string(pqErr.Code)
				}
				return "unknown"
			}(),
		)
		return apperrors.InternalServerError(err)
	}

	return nil
}

// GetByIDs retrieves a relationship between two users
func (store *userRelationshipStore) GetByIDs(ctx context.Context, followerID, followingID int) (*models.UserRelationship, error) {
	query := `
		SELECT relationship_id, follower_id, following_id, relationship_type, created_at, updated_at
		FROM user_relationships
		WHERE follower_id = $1 AND following_id = $2
	`

	var relationship models.UserRelationship
	err := store.db.QueryRowContext(ctx, query, followerID, followingID).Scan(
		&relationship.RelationshipID,
		&relationship.FollowerID,
		&relationship.FollowingID,
		&relationship.RelationshipType,
		&relationship.CreatedAt,
		&relationship.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			// Don't log - this is expected behavior, not an error
			return nil, apperrors.NotFoundError("relationship not found between these users", err)
		}
		// Log actual database errors with full context
		store.logger.Error("database query failed",
			"operation", "get_user_relationship_by_ids",
			"follower_id", followerID,
			"following_id", followingID,
			"query", query,
			"error", err,
		)
		return nil, apperrors.InternalServerError(err)
	}

	return &relationship, nil
}

// Update updates an existing relationship
func (store *userRelationshipStore) Update(ctx context.Context, tx *sql.Tx, relationship *models.UserRelationship) error {
	query := `
		UPDATE user_relationships 
		SET relationship_type = $1, updated_at = CURRENT_TIMESTAMP
		WHERE follower_id = $2 AND following_id = $3
		RETURNING relationship_id, created_at, updated_at
	`

	exec := store.db.QueryRowContext
	if tx != nil {
		exec = tx.QueryRowContext
	}

	err := exec(ctx, query, relationship.RelationshipType, relationship.FollowerID, relationship.FollowingID).Scan(
		&relationship.RelationshipID,
		&relationship.CreatedAt,
		&relationship.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			// Don't log - this is expected behavior, not an error
			return apperrors.NotFoundError("relationship not found between these users", err)
		}
		// Log actual database errors with full context
		store.logger.Error("database update failed",
			"operation", "update_user_relationship",
			"follower_id", relationship.FollowerID,
			"following_id", relationship.FollowingID,
			"relationship_type", relationship.RelationshipType,
			"query", query,
			"error", err,
		)
		return apperrors.InternalServerError(err)
	}

	return nil
}

// Delete deletes a relationship between two users
func (store *userRelationshipStore) Delete(ctx context.Context, tx *sql.Tx, followerID, followingID int) error {
	query := `
		DELETE FROM user_relationships
		WHERE follower_id = $1 AND following_id = $2
	`

	exec := store.db.ExecContext
	if tx != nil {
		exec = tx.ExecContext
	}

	result, err := exec(ctx, query, followerID, followingID)
	if err != nil {
		// Log actual database errors with full context
		store.logger.Error("database delete failed",
			"operation", "delete_user_relationship",
			"follower_id", followerID,
			"following_id", followingID,
			"query", query,
			"error", err,
		)
		return apperrors.InternalServerError(err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		store.logger.Error("failed to get rows affected",
			"operation", "delete_user_relationship",
			"error", err,
		)
		return apperrors.InternalServerError(err)
	}

	if rowsAffected == 0 {
		// Don't log - this is expected behavior, not an error
		return apperrors.NotFoundError("relationship not found between these users", nil)
	}

	return nil
}

// GetFollowers retrieves a paginated list of followers for a user
func (store *userRelationshipStore) GetFollowers(ctx context.Context, userID int, limit int, cursor string) (*models.UserFollowersResponse, error) {
	var query string
	var args []interface{}

	// Build the query based on whether we have a cursor
	if cursor == "" {
		// Initial query - get first batch of followers
		query = `
			SELECT u.user_id, u.username, u.email, u.password, u.soft_deleted, u.soft_deleted_at, u.created_at, u.updated_at, u.is_admin,
				   up.display_name, up.bio, up.profile_picture_uuid, up.banner_uuid, up.birth_date, up.location, up.website,
				   up.followers_count, up.following_count, ur.created_at as relationship_created_at
			FROM user_relationships ur
			JOIN users u ON ur.follower_id = u.user_id
			LEFT JOIN user_profiles up ON u.user_id = up.user_id
			WHERE ur.following_id = $1 AND ur.relationship_type = 'follow' AND u.soft_deleted = false
			ORDER BY ur.created_at DESC
			LIMIT $2
		`
		args = []interface{}{userID, limit + 1}
	} else {
		// Decode cursor to get the timestamp
		cursorData, err := util.DecodeCursor(cursor)
		if err != nil {
			store.logger.Error("failed to decode cursor",
				"operation", "get_followers",
				"user_id", userID,
				"cursor", cursor,
				"error", err,
			)
			return nil, apperrors.BadRequestError("invalid cursor format", err)
		}

		if cursorData == nil || cursorData.Timestamp == "" {
			store.logger.Error("invalid cursor data",
				"operation", "get_followers",
				"user_id", userID,
				"cursor", cursor,
			)
			return nil, apperrors.BadRequestError("invalid cursor data", nil)
		}

		// Cursor-based query - get followers after the cursor timestamp
		query = `
			SELECT u.user_id, u.username, u.email, u.password, u.soft_deleted, u.soft_deleted_at, u.created_at, u.updated_at, u.is_admin,
				   up.display_name, up.bio, up.profile_picture_uuid, up.banner_uuid, up.birth_date, up.location, up.website,
				   up.followers_count, up.following_count, ur.created_at as relationship_created_at
			FROM user_relationships ur
			JOIN users u ON ur.follower_id = u.user_id
			LEFT JOIN user_profiles up ON u.user_id = up.user_id
			WHERE ur.following_id = $1 AND ur.relationship_type = 'follow' AND u.soft_deleted = false
			AND ur.created_at < $2
			ORDER BY ur.created_at DESC
			LIMIT $3
		`
		args = []interface{}{userID, cursorData.Timestamp, limit + 1}
	}

	rows, err := store.db.QueryContext(ctx, query, args...)
	if err != nil {
		store.logger.Error("database query failed",
			"operation", "get_followers",
			"user_id", userID,
			"limit", limit,
			"cursor", cursor,
			"query", query,
			"error", err,
		)
		return nil, apperrors.InternalServerError(err)
	}
	defer rows.Close()

	followers := make([]models.UserWithProfile, 0)
	var hasMore bool
	count := 0

	for rows.Next() {
		var user models.UserWithProfile
		var relationshipCreatedAt string

		err := rows.Scan(
			&user.ID,
			&user.Username,
			&user.Email,
			&user.Password,
			&user.SoftDeleted,
			&user.SoftDeletedAt,
			&user.CreatedAt,
			&user.UpdatedAt,
			&user.IsAdmin,
			&user.Profile.DisplayName,
			&user.Profile.Bio,
			&user.Profile.ProfilePictureUUID,
			&user.Profile.BannerUUID,
			&user.Profile.BirthDate,
			&user.Profile.Location,
			&user.Profile.Website,
			&user.Profile.FollowersCount,
			&user.Profile.FollowingCount,
			&relationshipCreatedAt,
		)
		if err != nil {
			store.logger.Error("failed to scan follower row",
				"operation", "get_followers",
				"error", err,
			)
			return nil, apperrors.InternalServerError(err)
		}

		count++
		if count > limit {
			break
		}

		followers = append(followers, user)
	}

	if err = rows.Err(); err != nil {
		store.logger.Error("error iterating followers rows",
			"operation", "get_followers",
			"error", err,
		)
		return nil, apperrors.InternalServerError(err)
	}

	// Check if there are more followers
	if len(followers) == limit {
		hasMore = true
	}

	// Generate next cursor if there are more followers
	var nextCursor string
	if hasMore && len(followers) > 0 {
		// For cursor generation, we'll use the last follower's ID and current timestamp
		// In a real implementation, you might want to store the relationship timestamp
		cursorData, err := util.CreateIDCursor(followers[len(followers)-1].ID)
		if err == nil {
			if encodedCursor, err := util.EncodeCursor(*cursorData); err == nil {
				nextCursor = encodedCursor
			}
		}
	}

	items := make([]models.UserProfileResponse, len(followers))
	for i, f := range followers {
		items[i] = f.ToProfileResponse()
	}

	return &models.UserFollowersResponse{
		Items:      items,
		HasMore:    hasMore,
		NextCursor: nextCursor,
	}, nil
}

// GetFollowing retrieves a paginated list of users that a user is following
func (store *userRelationshipStore) GetFollowing(ctx context.Context, userID int, limit int, cursor string) (*models.UserFollowingResponse, error) {
	var query string
	var args []interface{}

	// Build the query based on whether we have a cursor
	if cursor == "" {
		// Initial query - get first batch of following users
		query = `
			SELECT u.user_id, u.username, u.email, u.password, u.soft_deleted, u.soft_deleted_at, u.created_at, u.updated_at, u.is_admin,
				   up.display_name, up.bio, up.profile_picture_uuid, up.banner_uuid, up.birth_date, up.location, up.website,
				   up.followers_count, up.following_count
			FROM user_relationships ur
			JOIN users u ON ur.following_id = u.user_id
			LEFT JOIN user_profiles up ON u.user_id = up.user_id
			WHERE ur.follower_id = $1 AND ur.relationship_type = 'follow' AND u.soft_deleted = false
			ORDER BY ur.created_at DESC
			LIMIT $2
		`
		args = []interface{}{userID, limit + 1}
	} else {
		// Decode cursor to get the user ID
		cursorData, err := util.DecodeCursor(cursor)
		if err != nil {
			store.logger.Error("failed to decode cursor",
				"operation", "get_following",
				"user_id", userID,
				"cursor", cursor,
				"error", err,
			)
			return nil, apperrors.BadRequestError("invalid cursor format", err)
		}

		if cursorData == nil || cursorData.ID == nil {
			store.logger.Error("invalid cursor data",
				"operation", "get_following",
				"user_id", userID,
				"cursor", cursor,
			)
			return nil, apperrors.BadRequestError("invalid cursor data", nil)
		}

		// Cursor-based query - get following users after the cursor ID
		query = `
			SELECT u.user_id, u.username, u.email, u.password, u.soft_deleted, u.soft_deleted_at, u.created_at, u.updated_at, u.is_admin,
				   up.display_name, up.bio, up.profile_picture_uuid, up.banner_uuid, up.birth_date, up.location, up.website,
				   up.followers_count, up.following_count
			FROM user_relationships ur
			JOIN users u ON ur.following_id = u.user_id
			LEFT JOIN user_profiles up ON u.user_id = up.user_id
			WHERE ur.follower_id = $1 AND ur.relationship_type = 'follow' AND u.soft_deleted = false
			AND u.user_id < $2
			ORDER BY ur.created_at DESC
			LIMIT $3
		`
		args = []interface{}{userID, cursorData.ID, limit + 1}
	}

	rows, err := store.db.QueryContext(ctx, query, args...)
	if err != nil {
		store.logger.Error("database query failed",
			"operation", "get_following",
			"user_id", userID,
			"limit", limit,
			"cursor", cursor,
			"query", query,
			"error", err,
		)
		return nil, apperrors.InternalServerError(err)
	}
	defer rows.Close()

	following := make([]models.UserWithProfile, 0)
	var hasMore bool
	count := 0

	for rows.Next() {
		var user models.UserWithProfile
		err := rows.Scan(
			&user.ID,
			&user.Username,
			&user.Email,
			&user.Password,
			&user.SoftDeleted,
			&user.SoftDeletedAt,
			&user.CreatedAt,
			&user.UpdatedAt,
			&user.IsAdmin,
			&user.Profile.DisplayName,
			&user.Profile.Bio,
			&user.Profile.ProfilePictureUUID,
			&user.Profile.BannerUUID,
			&user.Profile.BirthDate,
			&user.Profile.Location,
			&user.Profile.Website,
			&user.Profile.FollowersCount,
			&user.Profile.FollowingCount,
		)
		if err != nil {
			store.logger.Error("failed to scan following row",
				"operation", "get_following",
				"error", err,
			)
			return nil, apperrors.InternalServerError(err)
		}

		count++
		if count > limit {
			break
		}

		following = append(following, user)
	}

	if err = rows.Err(); err != nil {
		store.logger.Error("error iterating following rows",
			"operation", "get_following",
			"error", err,
		)
		return nil, apperrors.InternalServerError(err)
	}

	// Check if there are more following users
	if len(following) == limit {
		hasMore = true
	}

	// Generate next cursor if there are more following users
	var nextCursor string
	if hasMore && len(following) > 0 {
		cursorData, err := util.CreateIDCursor(following[len(following)-1].ID)
		if err == nil {
			if encodedCursor, err := util.EncodeCursor(*cursorData); err == nil {
				nextCursor = encodedCursor
			}
		}
	}

	items := make([]models.UserProfileResponse, len(following))
	for i, f := range following {
		items[i] = f.ToProfileResponse()
	}

	return &models.UserFollowingResponse{
		Items:      items,
		HasMore:    hasMore,
		NextCursor: nextCursor,
	}, nil
}

// GetFollowerIDs returns the user IDs of everyone following the given user.
func (store *userRelationshipStore) GetFollowerIDs(ctx context.Context, userID int) ([]int, error) {
	rows, err := store.db.QueryContext(ctx,
		`SELECT follower_id FROM user_relationships WHERE following_id = $1 AND relationship_type = 'follow'`,
		userID)
	if err != nil {
		store.logger.Error("database query failed", "operation", "get_follower_ids", "userID", userID, "error", err)
		return nil, apperrors.InternalServerError(err)
	}
	defer rows.Close()

	var ids []int
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			return nil, apperrors.InternalServerError(err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, apperrors.InternalServerError(err)
	}
	return ids, nil
}

// DeleteByType deletes relationships of a specific type between two users
func (store *userRelationshipStore) DeleteByType(ctx context.Context, tx *sql.Tx, followerID, followingID int, relationshipType string) error {
	query := `
		DELETE FROM user_relationships
		WHERE follower_id = $1 AND following_id = $2 AND relationship_type = $3
	`

	exec := store.db.ExecContext
	if tx != nil {
		exec = tx.ExecContext
	}

	result, err := exec(ctx, query, followerID, followingID, relationshipType)
	if err != nil {
		store.logger.Error("database delete failed",
			"operation", "delete_user_relationship_by_type",
			"follower_id", followerID,
			"following_id", followingID,
			"relationship_type", relationshipType,
			"query", query,
			"error", err,
		)
		return apperrors.InternalServerError(err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		store.logger.Error("failed to get rows affected",
			"operation", "delete_user_relationship_by_type",
			"error", err,
		)
		return apperrors.InternalServerError(err)
	}

	if rowsAffected == 0 {
		return apperrors.NotFoundError("relationship not found between these users", nil)
	}

	return nil
}

// Exists reports whether a relationship of the given type exists between two users
func (store *userRelationshipStore) Exists(ctx context.Context, followerID, followingID int, relationshipType string) (bool, error) {
	var exists bool
	err := store.db.QueryRowContext(ctx,
		`SELECT EXISTS (
			SELECT 1 FROM user_relationships
			WHERE follower_id = $1 AND following_id = $2 AND relationship_type = $3
		)`,
		followerID, followingID, relationshipType,
	).Scan(&exists)
	if err != nil {
		store.logger.Error("database query failed",
			"operation", "exists_user_relationship",
			"follower_id", followerID,
			"following_id", followingID,
			"relationship_type", relationshipType,
			"error", err,
		)
		return false, apperrors.InternalServerError(err)
	}
	return exists, nil
}

// GetRelationshipStatus gets the relationship status between two users. Because
// a pair may hold several relationship types at once (e.g. follow + mute), every
// row is read.
func (store *userRelationshipStore) GetRelationshipStatus(ctx context.Context, followerID, followingID int) (*models.RelationshipStatus, error) {
	rows, err := store.db.QueryContext(ctx,
		`SELECT relationship_type FROM user_relationships
		 WHERE follower_id = $1 AND following_id = $2`,
		followerID, followingID,
	)
	if err != nil {
		store.logger.Error("database query failed",
			"operation", "get_relationship_status",
			"follower_id", followerID,
			"following_id", followingID,
			"query", "get_relationship_types",
			"error", err,
		)
		return nil, apperrors.InternalServerError(err)
	}
	defer rows.Close()

	status := &models.RelationshipStatus{}
	for rows.Next() {
		var relationshipType string
		if err := rows.Scan(&relationshipType); err != nil {
			store.logger.Error("failed to scan relationship status row",
				"operation", "get_relationship_status",
				"error", err,
			)
			return nil, apperrors.InternalServerError(err)
		}
		switch relationshipType {
		case "follow":
			status.IsFollowing = true
		case "block":
			status.IsBlocked = true
		case "mute":
			status.IsMuted = true
		}
	}
	if err := rows.Err(); err != nil {
		store.logger.Error("error iterating relationship status rows",
			"operation", "get_relationship_status",
			"error", err,
		)
		return nil, apperrors.InternalServerError(err)
	}

	return status, nil
}

// GetRelationshipStatuses returns the relationship status between the viewer and
// each target user, keyed by target user ID.
func (store *userRelationshipStore) GetRelationshipStatuses(ctx context.Context, viewerID int, targetIDs []int) (map[int]*models.RelationshipStatus, error) {
	if len(targetIDs) == 0 {
		return map[int]*models.RelationshipStatus{}, nil
	}

	statusMap := make(map[int]*models.RelationshipStatus, len(targetIDs))
	for _, id := range targetIDs {
		statusMap[id] = &models.RelationshipStatus{}
	}

	rows, err := store.db.QueryContext(ctx,
		`SELECT following_id, relationship_type FROM user_relationships
		 WHERE follower_id = $1 AND following_id = ANY($2)`,
		viewerID, pq.Array(targetIDs),
	)
	if err != nil {
		store.logger.Error("database query failed",
			"operation", "get_relationship_statuses",
			"viewer_id", viewerID,
			"target_ids", targetIDs,
			"error", err,
		)
		return nil, apperrors.InternalServerError(err)
	}
	defer rows.Close()

	for rows.Next() {
		var targetID int
		var relationshipType string
		if err := rows.Scan(&targetID, &relationshipType); err != nil {
			store.logger.Error("failed to scan relationship statuses row",
				"operation", "get_relationship_statuses",
				"error", err,
			)
			return nil, apperrors.InternalServerError(err)
		}
		status := statusMap[targetID]
		if status == nil {
			status = &models.RelationshipStatus{}
			statusMap[targetID] = status
		}
		switch relationshipType {
		case "follow":
			status.IsFollowing = true
		case "block":
			status.IsBlocked = true
		case "mute":
			status.IsMuted = true
		}
	}
	if err := rows.Err(); err != nil {
		store.logger.Error("error iterating relationship statuses rows",
			"operation", "get_relationship_statuses",
			"error", err,
		)
		return nil, apperrors.InternalServerError(err)
	}

	return statusMap, nil
}
