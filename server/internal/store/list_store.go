package store

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"time"

	"github.com/ba-reynolds/gaggle/internal/apperrors"
	"github.com/ba-reynolds/gaggle/internal/models"
	"github.com/ba-reynolds/gaggle/internal/util"
	"github.com/jackc/pgx/v5/pgconn"
)

type listStore struct {
	db     *sql.DB
	logger *slog.Logger
}

const listColumns = `l.list_id, l.owner_id, u.username, l.name, l.description, l.created_at`

func (s *listStore) scanList(row interface{ Scan(...any) error }, includeCount bool) (models.List, error) {
	var l models.List
	var err error
	if includeCount {
		err = row.Scan(&l.ID, &l.OwnerID, &l.OwnerUsername, &l.Name, &l.Description, &l.CreatedAt, &l.MemberCount)
	} else {
		l.MemberCount = 0
		err = row.Scan(&l.ID, &l.OwnerID, &l.OwnerUsername, &l.Name, &l.Description, &l.CreatedAt)
	}
	return l, err
}

// Create registers a new list owned by the given user.
func (s *listStore) Create(ctx context.Context, list *models.List) error {
	err := s.db.QueryRowContext(ctx, `
		INSERT INTO lists (owner_id, name, description)
		VALUES ($1, $2, $3)
		RETURNING list_id, created_at`, list.OwnerID, list.Name, list.Description).
		Scan(&list.ID, &list.CreatedAt)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return apperrors.AlreadyExistsError("you already have a list with that name", err)
		}
		return apperrors.InternalServerError(err)
	}
	return nil
}

// GetByID fetches a list by id.
func (s *listStore) GetByID(ctx context.Context, listID int) (*models.List, error) {
	var l models.List
	err := s.db.QueryRowContext(ctx, `
		SELECT `+listColumns+`, COUNT(lm.user_id)::int
		FROM lists l
		JOIN users u ON u.user_id = l.owner_id
		LEFT JOIN list_members lm ON lm.list_id = l.list_id
		WHERE l.list_id = $1
		GROUP BY l.list_id, u.username`, listID).
		Scan(&l.ID, &l.OwnerID, &l.OwnerUsername, &l.Name, &l.Description, &l.CreatedAt, &l.MemberCount)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, apperrors.NotFoundError("list not found", err)
		}
		return nil, apperrors.InternalServerError(err)
	}
	return &l, nil
}

// ListByOwner returns a user's lists (newest first).
func (s *listStore) ListByOwner(ctx context.Context, ownerID int) ([]models.List, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT `+listColumns+`, COUNT(lm.user_id)::int
		FROM lists l
		JOIN users u ON u.user_id = l.owner_id
		LEFT JOIN list_members lm ON lm.list_id = l.list_id
		WHERE l.owner_id = $1
		GROUP BY l.list_id, u.username
		ORDER BY l.created_at DESC`, ownerID)
	if err != nil {
		return nil, apperrors.InternalServerError(err)
	}
	defer rows.Close()
	var out []models.List
	for rows.Next() {
		l, err := s.scanList(rows, true)
		if err != nil {
			return nil, apperrors.InternalServerError(err)
		}
		out = append(out, l)
	}
	if err := rows.Err(); err != nil {
		return nil, apperrors.InternalServerError(err)
	}
	return out, nil
}

// Delete removes a list (members cascade).
func (s *listStore) Delete(ctx context.Context, listID, ownerID int) error {
	result, err := s.db.ExecContext(ctx, `DELETE FROM lists WHERE list_id = $1 AND owner_id = $2`, listID, ownerID)
	if err != nil {
		return apperrors.InternalServerError(err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return apperrors.InternalServerError(err)
	}
	if affected == 0 {
		return apperrors.NotFoundError("list not found", nil)
	}
	return nil
}

// Update edits a list's name and description.
func (s *listStore) Update(ctx context.Context, listID int, name, description string) (*models.List, error) {
	if _, err := s.db.ExecContext(ctx, `UPDATE lists SET name = $1, description = $2 WHERE list_id = $3`, name, description, listID); err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil, apperrors.AlreadyExistsError("you already have a list with that name", err)
		}
		return nil, apperrors.InternalServerError(err)
	}
	return s.GetByID(ctx, listID)
}

// AddMember adds a user to a list. Returns AlreadyExists on duplicate.
func (s *listStore) AddMember(ctx context.Context, listID, memberID int) error {
	var ownerID int
	if err := s.db.QueryRowContext(ctx, `SELECT owner_id FROM lists WHERE list_id = $1`, listID).Scan(&ownerID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return apperrors.NotFoundError("list not found", err)
		}
		return apperrors.InternalServerError(err)
	}
	if ownerID == memberID {
		return apperrors.BadRequestError("you cannot add yourself to your own list", nil)
	}
	if _, err := s.db.ExecContext(ctx, `INSERT INTO list_members (list_id, user_id) VALUES ($1, $2)`, listID, memberID); err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return apperrors.AlreadyExistsError("user is already in this list", err)
		}
		return apperrors.InternalServerError(err)
	}
	return nil
}

// RemoveMember removes a user from a list.
func (s *listStore) RemoveMember(ctx context.Context, listID, memberID int) error {
	result, err := s.db.ExecContext(ctx, `DELETE FROM list_members WHERE list_id = $1 AND user_id = $2`, listID, memberID)
	if err != nil {
		return apperrors.InternalServerError(err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return apperrors.InternalServerError(err)
	}
	if affected == 0 {
		return apperrors.NotFoundError("user is not in this list", nil)
	}
	return nil
}

// GetMembers returns the users in a list, newest additions first.
func (s *listStore) GetMembers(ctx context.Context, listID, limit int, cursor string) (*models.ListMembersResponse, error) {
	base := `
		SELECT u.user_id, u.username, u.soft_deleted, u.soft_deleted_at, u.created_at, u.updated_at,
		       up.display_name, up.bio, up.profile_picture_uuid, up.banner_uuid, up.birth_date, up.location, up.website,
		       up.followers_count, up.following_count, lm.added_at
		FROM list_members lm
		JOIN users u ON u.user_id = lm.user_id
		LEFT JOIN user_profiles up ON up.user_id = u.user_id
		WHERE lm.list_id = $1 AND u.soft_deleted = FALSE`
	order := ` ORDER BY lm.added_at DESC, u.username ASC`

	var query string
	var args []interface{}
	if cursor == "" {
		query = base + order + ` LIMIT $2`
		args = []interface{}{listID, limit + 1}
	} else {
		cursorData, err := util.DecodeCursor(cursor)
		if err != nil {
			return nil, apperrors.BadRequestError("invalid cursor", err)
		}
		query = base + ` AND lm.added_at < $2` + order + ` LIMIT $3`
		args = []interface{}{listID, cursorData.Timestamp, limit + 1}
	}

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, apperrors.InternalServerError(err)
	}
	defer rows.Close()

	items := make([]models.UserProfileResponse, 0, limit+1)
	count := 0
	var lastAddedAt time.Time
	for rows.Next() {
		var u models.UserWithProfile
		var addedAt time.Time
		var isDeleted bool
		var deletedAt *time.Time
		if err := rows.Scan(&u.ID, &u.Username, &isDeleted, &deletedAt, &u.CreatedAt, &u.UpdatedAt,
			&u.Profile.DisplayName, &u.Profile.Bio, &u.Profile.ProfilePictureUUID, &u.Profile.BannerUUID,
			&u.Profile.BirthDate, &u.Profile.Location, &u.Profile.Website,
			&u.Profile.FollowersCount, &u.Profile.FollowingCount, &addedAt); err != nil {
			return nil, apperrors.InternalServerError(err)
		}
		count++
		if count > limit {
			break
		}
		lastAddedAt = addedAt
		resp := u.ToProfileResponse()
		items = append(items, resp)
	}
	if err := rows.Err(); err != nil {
		return nil, apperrors.InternalServerError(err)
	}

	result := &models.ListMembersResponse{Items: items}
	if count > limit {
		result.HasMore = true
		if len(items) > 0 && len(items) == limit {
			cursorData, err := util.CreateTimestampCursor(items[len(items)-1].UserID, lastAddedAt.Format(time.RFC3339Nano))
			if err == nil {
				if encoded, err := util.EncodeCursor(*cursorData); err == nil {
					result.NextCursor = encoded
				}
			}
		}
	}
	return result, nil
}
