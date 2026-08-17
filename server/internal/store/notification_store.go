package store

import (
	"context"
	"database/sql"
	"log/slog"
	"strconv"
	"time"

	"github.com/ba-reynolds/gophersocial/internal/apperrors"
	"github.com/ba-reynolds/gophersocial/internal/models"
	"github.com/ba-reynolds/gophersocial/internal/util"
	"github.com/google/uuid"
)

type notificationStore struct {
	db     *sql.DB
	logger *slog.Logger
}

func (s *notificationStore) Create(ctx context.Context, tx *sql.Tx, notification *models.Notification, recipientID, actorID int, notificationType string, postID *int) error {
	query := `
		INSERT INTO notifications (recipient_id, actor_id, notification_type, post_id)
		VALUES ($1, $2, $3, $4)
		RETURNING notification_id, created_at
	`
	row := s.db.QueryRowContext
	if tx != nil {
		row = tx.QueryRowContext
	}
	if err := row(ctx, query, recipientID, actorID, notificationType, postID).Scan(&notification.ID, &notification.CreatedAt); err != nil {
		s.logger.Error("failed to create notification", "recipient_id", recipientID, "actor_id", actorID, "type", notificationType, "error", err)
		return apperrors.InternalServerError(err)
	}
	notification.Type = notificationType
	notification.PostID = postID
	return nil
}

func (s *notificationStore) List(ctx context.Context, recipientID, limit int, cursor string) (*models.NotificationFeed, error) {
	query := `
		SELECT n.notification_id, n.notification_type, n.post_id, n.read_at, n.created_at,
		       u.username, p.display_name, p.profile_picture_uuid
		FROM notifications n
		JOIN users u ON u.user_id = n.actor_id
		JOIN user_profiles p ON p.user_id = u.user_id
		WHERE n.recipient_id = $1
	`
	args := []any{recipientID}
	if cursor != "" {
		decoded, err := util.DecodeCursor(cursor)
		if err != nil {
			return nil, apperrors.BadRequestError("invalid notification cursor", err)
		}
		id, ok := decoded.ID.(float64)
		if !ok {
			return nil, apperrors.BadRequestError("invalid notification cursor", nil)
		}
		timestamp, err := time.Parse(time.RFC3339Nano, decoded.Timestamp)
		if err != nil {
			return nil, apperrors.BadRequestError("invalid notification cursor", err)
		}
		query += " AND (n.created_at, n.notification_id) < ($2, $3)"
		args = append(args, timestamp, int(id))
	}
	query += " ORDER BY n.created_at DESC, n.notification_id DESC LIMIT $" + strconv.Itoa(len(args)+1)
	args = append(args, limit+1)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		s.logger.Error("failed to list notifications", "recipient_id", recipientID, "error", err)
		return nil, apperrors.InternalServerError(err)
	}
	defer rows.Close()

	items := make([]models.Notification, 0, limit)
	for rows.Next() {
		var item models.Notification
		var profilePicture sql.NullString
		if err := rows.Scan(&item.ID, &item.Type, &item.PostID, &item.ReadAt, &item.CreatedAt,
			&item.Actor.Username, &item.Actor.DisplayName, &profilePicture); err != nil {
			return nil, apperrors.InternalServerError(err)
		}
		if profilePicture.Valid {
			// UUID is represented as a string in the query to keep NULL profile images simple.
			if parsed, err := uuid.Parse(profilePicture.String); err == nil {
				item.Actor.ProfilePictureUUID = &parsed
			}
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, apperrors.InternalServerError(err)
	}

	feed := &models.NotificationFeed{Items: items}
	if len(items) > limit {
		feed.HasMore = true
		feed.Items = items[:limit]
		last := feed.Items[len(feed.Items)-1]
		cursor, err := util.EncodeCursor(util.PaginationCursor{ID: last.ID, Timestamp: last.CreatedAt.Format(time.RFC3339Nano), Order: "desc"})
		if err != nil {
			return nil, apperrors.InternalServerError(err)
		}
		feed.NextCursor = cursor
	}
	return feed, nil
}

func (s *notificationStore) UnreadCount(ctx context.Context, recipientID int) (int, error) {
	var count int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM notifications WHERE recipient_id = $1 AND read_at IS NULL`, recipientID).Scan(&count); err != nil {
		return 0, apperrors.InternalServerError(err)
	}
	return count, nil
}

func (s *notificationStore) MarkRead(ctx context.Context, recipientID, notificationID int) error {
	result, err := s.db.ExecContext(ctx, `UPDATE notifications SET read_at = COALESCE(read_at, CURRENT_TIMESTAMP) WHERE notification_id = $1 AND recipient_id = $2`, notificationID, recipientID)
	if err != nil {
		return apperrors.InternalServerError(err)
	}
	if affected, _ := result.RowsAffected(); affected == 0 {
		return apperrors.NotFoundError("notification not found", nil)
	}
	return nil
}

func (s *notificationStore) MarkAllRead(ctx context.Context, recipientID int) error {
	if _, err := s.db.ExecContext(ctx, `UPDATE notifications SET read_at = CURRENT_TIMESTAMP WHERE recipient_id = $1 AND read_at IS NULL`, recipientID); err != nil {
		return apperrors.InternalServerError(err)
	}
	return nil
}
