package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/ba-reynolds/gaggle/internal/apperrors"
	"github.com/ba-reynolds/gaggle/internal/models"
	"github.com/google/uuid"
	"github.com/lib/pq"
)

type userStore struct {
	db     *sql.DB
	logger *slog.Logger
}

func (store *userStore) GetByID(ctx context.Context, id int) (*models.User, error) {
	query := `
		SELECT user_id, username, email, password, soft_deleted, soft_deleted_at, created_at, updated_at, is_admin, is_private
		FROM users
		WHERE user_id = $1
	`

	var user models.User
	err := store.db.QueryRowContext(ctx, query, id).Scan(
		&user.ID,
		&user.Username,
		&user.Email,
		&user.Password,
		&user.SoftDeleted,
		&user.SoftDeletedAt,
		&user.CreatedAt,
		&user.UpdatedAt,
		&user.IsAdmin,
		&user.IsPrivate,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, apperrors.NotFoundError(fmt.Sprintf("user with id %d not found", id), err)
		}
		store.logger.Error("database query failed",
			"operation", "get_user_by_id",
			"userID", id,
			"query", query,
			"error", err,
		)
		return nil, apperrors.InternalServerError(err)
	}

	return &user, nil
}

func (store *userStore) GetByEmail(ctx context.Context, email string) (*models.User, error) {
	query := `
		SELECT user_id, username, email, password, soft_deleted, soft_deleted_at, created_at, updated_at, is_admin, is_private
		FROM users
		WHERE email = $1
	`

	var user models.User
	err := store.db.QueryRowContext(ctx, query, email).Scan(
		&user.ID,
		&user.Username,
		&user.Email,
		&user.Password,
		&user.SoftDeleted,
		&user.SoftDeletedAt,
		&user.CreatedAt,
		&user.UpdatedAt,
		&user.IsAdmin,
		&user.IsPrivate,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, apperrors.NotFoundError(fmt.Sprintf("user with email %s not found", email), err)
		}
		store.logger.Error("database query failed",
			"operation", "get_user_by_email",
			"email", email,
			"query", query,
			"error", err,
		)
		return nil, apperrors.InternalServerError(err)
	}

	return &user, nil
}

func (store *userStore) GetByUsername(ctx context.Context, username string) (*models.User, error) {
	query := `
		SELECT user_id, username, email, password, soft_deleted, soft_deleted_at, created_at, updated_at, is_admin, is_private
		FROM users
		WHERE username = $1
	`

	var user models.User
	err := store.db.QueryRowContext(ctx, query, username).Scan(
		&user.ID,
		&user.Username,
		&user.Email,
		&user.Password,
		&user.SoftDeleted,
		&user.SoftDeletedAt,
		&user.CreatedAt,
		&user.UpdatedAt,
		&user.IsAdmin,
		&user.IsPrivate,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, apperrors.NotFoundError(fmt.Sprintf("user with username %s not found", username), err)
		}
		store.logger.Error("database query failed",
			"operation", "get_user_by_username",
			"username", username,
			"query", query,
			"error", err,
		)
		return nil, apperrors.InternalServerError(err)
	}

	return &user, nil
}

func (store *userStore) Create(ctx context.Context, tx *sql.Tx, user *models.User) error {
	queryCreateUser := `
		INSERT INTO users (username, email, password)
		VALUES ($1, $2, $3)
		RETURNING user_id
	`

	var id int
	err := tx.QueryRowContext(ctx, queryCreateUser, user.Username, user.Email, user.Password).Scan(&id)
	if err != nil {
		// Check if it's a unique constraint violation by searching the error string
		errStr := err.Error()
		if strings.Contains(errStr, "unique_violation") || strings.Contains(errStr, "duplicate key") {
			// Check which specific constraint was violated
			if strings.Contains(errStr, "unique_username") {
				return apperrors.UsernameExistsError("username already exists", err)
			}
			if strings.Contains(errStr, "unique_email_case_insensitive") {
				return apperrors.EmailExistsError("email already exists", err)
			}
			// Fallback for any other unique constraint violations
			return apperrors.AlreadyExistsError("user already exists", err)
		}

		store.logger.Error("database insert failed",
			"operation", "create_user",
			"username", user.Username,
			"email", user.Email,
			"query", queryCreateUser,
			"error", err,
		)
		return apperrors.InternalServerError(err)
	}

	user.ID = id

	// Create user profile
	queryCreateProfile := `
		INSERT INTO user_profiles (user_id, display_name, bio, profile_picture_uuid, banner_uuid, birth_date, location, website)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`

	exec := store.db.ExecContext
	if tx != nil {
		exec = tx.ExecContext
	}

	_, err = exec(ctx, queryCreateProfile, id, "", "", nil, nil, nil, "", "")
	if err != nil {
		store.logger.Error("database insert failed",
			"operation", "create_user_profile",
			"userID", id,
			"username", user.Username,
			"query", queryCreateProfile,
			"error", err,
		)
		return apperrors.InternalServerError(err)
	}

	return nil
}

func (store *userStore) GetUserProfileByUsername(ctx context.Context, username string) (*models.UserWithProfile, error) {
	query := `
		SELECT u.user_id, u.username, u.email, u.password, u.soft_deleted, u.soft_deleted_at, u.created_at, u.updated_at, u.is_admin, u.is_private,
			   up.display_name, up.bio, up.profile_picture_uuid, up.banner_uuid, up.birth_date, up.location, up.website,
			   up.followers_count, up.following_count
		FROM users u
		LEFT JOIN user_profiles up ON u.user_id = up.user_id
		WHERE LOWER(u.username) = LOWER($1)
	`

	var user models.UserWithProfile
	err := store.db.QueryRowContext(ctx, query, username).Scan(
		&user.ID,
		&user.Username,
		&user.Email,
		&user.Password,
		&user.SoftDeleted,
		&user.SoftDeletedAt,
		&user.CreatedAt,
		&user.UpdatedAt,
		&user.IsAdmin,
		&user.IsPrivate,
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
		if errors.Is(err, sql.ErrNoRows) {
			return nil, apperrors.NotFoundError(fmt.Sprintf("user profile for username %s not found", username), err)
		}
		store.logger.Error("database query failed",
			"operation", "get_user_profile_by_username",
			"username", username,
			"query", query,
			"error", err,
		)
		return nil, apperrors.InternalServerError(err)
	}

	return &user, nil
}

func (store *userStore) Search(ctx context.Context, query string, limit int) (*models.UserList, error) {
	query = strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(query)
	rows, err := store.db.QueryContext(ctx, `
		SELECT u.user_id, u.username, up.display_name, up.bio, up.profile_picture_uuid, up.banner_uuid,
		       up.birth_date, up.location, up.website, up.followers_count, up.following_count, u.created_at
		FROM users u
		JOIN user_profiles up ON up.user_id = u.user_id
		WHERE u.soft_deleted = FALSE
		  AND (u.username ILIKE '%' || $1 || '%' ESCAPE '\' OR up.display_name ILIKE '%' || $1 || '%' ESCAPE '\')
		ORDER BY up.followers_count DESC, u.username ASC
		LIMIT $2`, query, limit+1)
	if err != nil {
		return nil, apperrors.InternalServerError(err)
	}
	defer rows.Close()

	users := make([]models.UserProfileResponse, 0, limit+1)
	for rows.Next() {
		var profile models.UserProfileResponse
		if err := rows.Scan(&profile.UserID, &profile.Username, &profile.DisplayName, &profile.Bio,
			&profile.ProfilePictureUUID, &profile.BannerUUID, &profile.BirthDate,
			&profile.Location, &profile.Website, &profile.FollowersCount,
			&profile.FollowingCount, &profile.CreatedAt); err != nil {
			return nil, apperrors.InternalServerError(err)
		}
		users = append(users, profile)
	}
	if err := rows.Err(); err != nil {
		return nil, apperrors.InternalServerError(err)
	}
	result := &models.UserList{Items: users}
	if len(users) > limit {
		result.HasMore = true
		result.Items = users[:limit]
	}
	return result, nil
}

// Suggested returns users to follow: highest followers first, excluding the
// viewer, accounts the viewer already follows, and accounts the viewer blocked.
func (store *userStore) Suggested(ctx context.Context, viewerID int, limit int) (*models.UserList, error) {
	rows, err := store.db.QueryContext(ctx, `
		SELECT u.user_id, u.username, up.display_name, up.bio, up.profile_picture_uuid, up.banner_uuid,
		       up.birth_date, up.location, up.website, up.followers_count, up.following_count, u.created_at
		FROM users u
		JOIN user_profiles up ON up.user_id = u.user_id
		WHERE u.soft_deleted = FALSE
		  AND u.user_id <> $1
		  AND NOT EXISTS (
		      SELECT 1 FROM user_relationships ur
		      WHERE ur.follower_id = $1 AND ur.following_id = u.user_id
		        AND ur.relationship_type IN ('follow', 'block')
		  )
		ORDER BY up.followers_count DESC, u.username ASC
		LIMIT $2`, viewerID, limit+1)
	if err != nil {
		return nil, apperrors.InternalServerError(err)
	}
	defer rows.Close()

	users := make([]models.UserProfileResponse, 0, limit+1)
	for rows.Next() {
		var profile models.UserProfileResponse
		if err := rows.Scan(&profile.UserID, &profile.Username, &profile.DisplayName, &profile.Bio,
			&profile.ProfilePictureUUID, &profile.BannerUUID, &profile.BirthDate,
			&profile.Location, &profile.Website, &profile.FollowersCount,
			&profile.FollowingCount, &profile.CreatedAt); err != nil {
			return nil, apperrors.InternalServerError(err)
		}
		users = append(users, profile)
	}
	if err := rows.Err(); err != nil {
		return nil, apperrors.InternalServerError(err)
	}
	result := &models.UserList{Items: users}
	if len(users) > limit {
		result.HasMore = true
		result.Items = users[:limit]
	}
	return result, nil
}

// SetPrivate toggles the query-time account privacy flag. Mirrors the
// profileVisibility preference stored in the JSONB settings row.
func (store *userStore) SetPrivate(ctx context.Context, userID int, isPrivate bool) error {
	if _, err := store.db.ExecContext(ctx, `UPDATE users SET is_private = $1 WHERE user_id = $2`, isPrivate, userID); err != nil {
		store.logger.Error("database update failed",
			"operation", "set_user_private",
			"userID", userID,
			"error", err,
		)
		return apperrors.InternalServerError(err)
	}
	return nil
}

// GetIsPrivate returns the account-privacy flag for each user ID, keyed by ID.
// Used by the service layer to gate content per author in batch.
func (store *userStore) GetIsPrivate(ctx context.Context, userIDs []int) (map[int]bool, error) {
	if len(userIDs) == 0 {
		return map[int]bool{}, nil
	}
	rows, err := store.db.QueryContext(ctx, `SELECT user_id, is_private FROM users WHERE user_id = ANY($1)`, pq.Array(userIDs))
	if err != nil {
		store.logger.Error("database query failed",
			"operation", "get_users_is_private",
			"userIDs", userIDs,
			"error", err,
		)
		return nil, apperrors.InternalServerError(err)
	}
	defer rows.Close()
	result := make(map[int]bool, len(userIDs))
	for rows.Next() {
		var id int
		var isPrivate bool
		if err := rows.Scan(&id, &isPrivate); err != nil {
			store.logger.Error("failed to scan user is_private", "operation", "get_users_is_private", "error", err)
			return nil, apperrors.InternalServerError(err)
		}
		result[id] = isPrivate
	}
	if err := rows.Err(); err != nil {
		return nil, apperrors.InternalServerError(err)
	}
	return result, nil
}

// UpdateUserProfile updates a user's profile
func (store *userStore) UpdateUserProfile(ctx context.Context, tx *sql.Tx, user *models.UserWithProfile) error {
	query := `
		UPDATE user_profiles
		SET display_name = $1, bio = $2, profile_picture_uuid = $3, banner_uuid = $4, birth_date = $5, location = $6, website = $7,
		    updated_at = CURRENT_TIMESTAMP
		WHERE user_id = $8
	`

	exec := store.db.ExecContext
	if tx != nil {
		exec = tx.ExecContext
	}

	var profilePictureUUID any = user.Profile.ProfilePictureUUID
	if profilePictureUUID == uuid.Nil {
		profilePictureUUID = nil
	}

	var bannerUUID any = user.Profile.BannerUUID
	if bannerUUID == uuid.Nil {
		bannerUUID = nil
	}

	result, err := exec(ctx, query, user.Profile.DisplayName, user.Profile.Bio, profilePictureUUID, bannerUUID, user.Profile.BirthDate, user.Profile.Location, user.Profile.Website, user.ID)
	if err != nil {
		store.logger.Error("database update failed",
			"operation", "update_user_profile",
			"userID", user.ID,
			"username", user.Username,
			"query", query,
			"error", err,
		)
		return apperrors.InternalServerError(err)
	}

	// Check if any rows were actually updated
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		// Log errors checking rows affected
		store.logger.Error("failed to check rows affected",
			"operation", "update_user_profile",
			"userID", user.ID,
			"error", err,
		)
		return apperrors.InternalServerError(err)
	}

	if rowsAffected == 0 {
		// Log when no rows were updated (could indicate user not found or soft deleted)
		store.logger.Warn("no rows updated",
			"operation", "update_user_profile",
			"userID", user.ID,
			"username", user.Username,
		)
		return apperrors.NotFoundError("user profile not found", nil)
	}

	return nil
}

// GetSettings returns the settings row for a user, creating it with defaults
// if it doesn't exist yet.
func (store *userStore) GetSettings(ctx context.Context, userID int) (*models.UserSettings, error) {
	query := `
		INSERT INTO user_settings (user_id)
		VALUES ($1)
		ON CONFLICT (user_id) DO NOTHING
	`
	if _, err := store.db.ExecContext(ctx, query, userID); err != nil {
		store.logger.Error("database insert failed",
			"operation", "ensure_user_settings",
			"userID", userID,
			"error", err,
		)
		return nil, apperrors.InternalServerError(err)
	}

	var raw []byte
	err := store.db.QueryRowContext(ctx, `SELECT settings FROM user_settings WHERE user_id = $1`, userID).Scan(&raw)
	if err != nil {
		store.logger.Error("database query failed",
			"operation", "get_user_settings",
			"userID", userID,
			"error", err,
		)
		return nil, apperrors.InternalServerError(err)
	}

	var settings models.UserSettings
	if err := json.Unmarshal(raw, &settings); err != nil {
		store.logger.Error("failed to unmarshal user settings",
			"operation", "get_user_settings",
			"userID", userID,
			"error", err,
		)
		return nil, apperrors.InternalServerError(err)
	}
	return &settings, nil
}

// UpdateSettings writes the settings row for a user.
func (store *userStore) UpdateSettings(ctx context.Context, userID int, settings *models.UserSettings) error {
	raw, err := json.Marshal(settings)
	if err != nil {
		return apperrors.InternalServerError(err)
	}

	query := `
		INSERT INTO user_settings (user_id, settings, updated_at)
		VALUES ($1, $2, CURRENT_TIMESTAMP)
		ON CONFLICT (user_id) DO UPDATE SET settings = EXCLUDED.settings, updated_at = CURRENT_TIMESTAMP
	`
	if _, err := store.db.ExecContext(ctx, query, userID, raw); err != nil {
		store.logger.Error("database update failed",
			"operation", "update_user_settings",
			"userID", userID,
			"error", err,
		)
		return apperrors.InternalServerError(err)
	}
	return nil
}

// CreateSettings seeds the settings row for a brand-new user during
// registration. Existing row keys are preserved via jsonb merge so defaults and
// any prior writes survive a re-run.
func (store *userStore) CreateSettings(ctx context.Context, tx *sql.Tx, userID int, settings *models.UserSettings) error {
	raw, err := json.Marshal(settings)
	if err != nil {
		return apperrors.InternalServerError(err)
	}

	query := `
		INSERT INTO user_settings (user_id, settings, updated_at)
		VALUES ($1, $2, CURRENT_TIMESTAMP)
		ON CONFLICT (user_id) DO UPDATE SET settings = user_settings.settings || EXCLUDED.settings, updated_at = CURRENT_TIMESTAMP
	`
	exec := store.db.ExecContext
	if tx != nil {
		exec = tx.ExecContext
	}
	if _, err := exec(ctx, query, userID, raw); err != nil {
		store.logger.Error("database insert failed",
			"operation", "create_user_settings",
			"userID", userID,
			"error", err,
		)
		return apperrors.InternalServerError(err)
	}
	return nil
}
