package store

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"strconv"
	"time"

	"github.com/ba-reynolds/gophersocial/internal/apperrors"
	"github.com/ba-reynolds/gophersocial/internal/models"
	"github.com/ba-reynolds/gophersocial/internal/util"
	"github.com/google/uuid"
)

type dmStore struct {
	db     *sql.DB
	logger *slog.Logger
}

// GetOrCreateConversation returns the conversation for a participant pair,
// creating it if missing. Participant IDs are normalized (a < b).
func (s *dmStore) GetOrCreateConversation(ctx context.Context, participantA, participantB int) (*models.Conversation, error) {
	if participantA == participantB {
		return nil, apperrors.BadRequestError("you cannot message yourself", nil)
	}
	a, b := minInt(participantA, participantB), maxInt(participantA, participantB)
	var c models.Conversation
	err := s.db.QueryRowContext(ctx, `
		INSERT INTO conversations (participant_a, participant_b)
		VALUES ($1, $2)
		ON CONFLICT (participant_a, participant_b) DO NOTHING
		RETURNING conversation_id, created_at, last_message_at, participant_a, participant_b`,
		a, b).Scan(&c.ID, &c.CreatedAt, &c.LastMessageAt, &c.ParticipantA, &c.ParticipantB)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, apperrors.InternalServerError(err)
	}
	if c.ID == 0 {
		// Already existed; fetch it.
		if err := s.db.QueryRowContext(ctx, `
			SELECT conversation_id, created_at, last_message_at, participant_a, participant_b
			FROM conversations WHERE participant_a = $1 AND participant_b = $2`,
			a, b).Scan(&c.ID, &c.CreatedAt, &c.LastMessageAt, &c.ParticipantA, &c.ParticipantB); err != nil {
			return nil, apperrors.InternalServerError(err)
		}
	}
	return &c, nil
}

func (s *dmStore) GetConversation(ctx context.Context, conversationID, viewerID int) (*models.Conversation, error) {
	var c models.Conversation
	if err := s.db.QueryRowContext(ctx, `
		SELECT conversation_id, created_at, last_message_at, participant_a, participant_b
		FROM conversations WHERE conversation_id = $1`, conversationID).
		Scan(&c.ID, &c.CreatedAt, &c.LastMessageAt, &c.ParticipantA, &c.ParticipantB); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, apperrors.NotFoundError("conversation not found", err)
		}
		return nil, apperrors.InternalServerError(err)
	}
	otherID := c.ParticipantA
	if otherID == viewerID {
		otherID = c.ParticipantB
	}
	if err := s.attachParticipantSummary(ctx, &c, otherID); err != nil {
		return nil, err
	}
	return &c, nil
}

// attachParticipantSummary fills OtherParticipant (and unread count) for a
// single conversation viewed by viewerID.
func (s *dmStore) attachParticipantSummary(ctx context.Context, c *models.Conversation, otherID int) error {
	var other models.UserAPISummary
	var profilePicture sql.NullString
	if err := s.db.QueryRowContext(ctx, `
		SELECT u.user_id, u.username, p.display_name, p.profile_picture_uuid
		FROM users u JOIN user_profiles p ON p.user_id = u.user_id
		WHERE u.user_id = $1`, otherID).
		Scan(&other.UserID, &other.Username, &other.DisplayName, &profilePicture); err != nil {
		return apperrors.InternalServerError(err)
	}
	if profilePicture.Valid {
		if parsed, err := uuid.Parse(profilePicture.String); err == nil {
			other.ProfilePictureUUID = &parsed
		}
	}
	c.OtherParticipant = &other
	var unread int
	if err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM messages
		WHERE conversation_id = $1 AND sender_id <> $2 AND read_at IS NULL`,
		c.ID, c.OtherParticipant.UserID).Scan(&unread); err != nil {
		return apperrors.InternalServerError(err)
	}
	c.UnreadCount = unread
	return nil
}

// ListConversations returns the viewer's inbox, newest activity first, with the
// other participant's summary, the last message, and the unread count.
func (s *dmStore) ListConversations(ctx context.Context, viewerID int) (*models.ConversationFeed, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT c.conversation_id, c.created_at, c.last_message_at,
		       u.user_id, u.username, p.display_name, p.profile_picture_uuid,
		       m.message_id, m.body, m.read_at, m.created_at,
		       (SELECT COUNT(*) FROM messages mu WHERE mu.conversation_id = c.conversation_id
		        AND mu.sender_id <> $1 AND mu.read_at IS NULL)::int AS unread
		FROM conversations c
		JOIN users u ON u.user_id = CASE WHEN c.participant_a = $1 THEN c.participant_b ELSE c.participant_a END
		JOIN user_profiles p ON p.user_id = u.user_id
		LEFT JOIN LATERAL (
			SELECT message_id, body, read_at, created_at FROM messages m
			WHERE m.conversation_id = c.conversation_id
			ORDER BY m.created_at DESC, m.message_id DESC
			LIMIT 1
		) m ON TRUE
		WHERE c.participant_a = $1 OR c.participant_b = $1
		ORDER BY c.last_message_at DESC`, viewerID)
	if err != nil {
		s.logger.Error("failed to list conversations", "viewer_id", viewerID, "error", err)
		return nil, apperrors.InternalServerError(err)
	}
	defer rows.Close()

	feed := &models.ConversationFeed{Items: make([]models.Conversation, 0)}
	for rows.Next() {
		var c models.Conversation
		var other models.UserAPISummary
		var last models.Message
		var profilePicture sql.NullString
		var hasLast bool
		if err := rows.Scan(&c.ID, &c.CreatedAt, &c.LastMessageAt,
			&other.UserID, &other.Username, &other.DisplayName, &profilePicture,
			&last.ID, &last.Body, &last.ReadAt, &last.CreatedAt, &c.UnreadCount); err != nil {
			return nil, apperrors.InternalServerError(err)
		}
		if profilePicture.Valid {
			if parsed, err := uuid.Parse(profilePicture.String); err == nil {
				other.ProfilePictureUUID = &parsed
			}
		}
		c.OtherParticipant = &other
		if last.ID != 0 {
			hasLast = true
		}
		if hasLast {
			c.LastMessage = &last
		}
		feed.Items = append(feed.Items, c)
	}
	if err := rows.Err(); err != nil {
		return nil, apperrors.InternalServerError(err)
	}
	return feed, nil
}

// AddMessage inserts a message, updates the conversation timestamp, and returns
// the created message with its sender summary.
func (s *dmStore) AddMessage(ctx context.Context, conversationID, senderID int, body string) (*models.Message, error) {
	var m models.Message
	err := s.db.QueryRowContext(ctx, `
		INSERT INTO messages (conversation_id, sender_id, body)
		VALUES ($1, $2, $3)
		RETURNING message_id, conversation_id, sender_id, body, read_at, created_at`,
		conversationID, senderID, body).
		Scan(&m.ID, &m.ConversationID, &m.SenderID, &m.Body, &m.ReadAt, &m.CreatedAt)
	if err != nil {
		return nil, apperrors.InternalServerError(err)
	}
	if _, err := s.db.ExecContext(ctx, `UPDATE conversations SET last_message_at = CURRENT_TIMESTAMP WHERE conversation_id = $1`, conversationID); err != nil {
		return nil, apperrors.InternalServerError(err)
	}
	if err := s.hydrateSenders(ctx, []*models.Message{&m}); err != nil {
		return nil, err
	}
	return &m, nil
}

// ListMessages returns a conversation's history, newest first, cursor-paginated.
func (s *dmStore) ListMessages(ctx context.Context, conversationID, limit int, cursor string) (*models.MessageFeed, error) {
	query := `
		SELECT m.message_id, m.conversation_id, m.sender_id, m.body, m.read_at, m.created_at,
		       u.username, p.display_name, p.profile_picture_uuid
		FROM messages m
		JOIN users u ON u.user_id = m.sender_id
		JOIN user_profiles p ON p.user_id = u.user_id
		WHERE m.conversation_id = $1
	`
	args := []any{conversationID}
	if cursor != "" {
		decoded, err := util.DecodeCursor(cursor)
		if err != nil {
			return nil, apperrors.BadRequestError("invalid message cursor", err)
		}
		id, ok := decoded.ID.(float64)
		if !ok {
			return nil, apperrors.BadRequestError("invalid message cursor", nil)
		}
		timestamp, err := time.Parse(time.RFC3339Nano, decoded.Timestamp)
		if err != nil {
			return nil, apperrors.BadRequestError("invalid message cursor", err)
		}
		query += " AND (m.created_at, m.message_id) < ($2, $3)"
		args = append(args, timestamp, int(id))
	}
	query += " ORDER BY m.created_at DESC, m.message_id DESC LIMIT $" + strconv.Itoa(len(args)+1)
	args = append(args, limit+1)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, apperrors.InternalServerError(err)
	}
	defer rows.Close()

	items := make([]models.Message, 0, limit)
	for rows.Next() {
		var m models.Message
		var profilePicture sql.NullString
		if err := rows.Scan(&m.ID, &m.ConversationID, &m.SenderID, &m.Body, &m.ReadAt, &m.CreatedAt,
			&m.Sender.Username, &m.Sender.DisplayName, &profilePicture); err != nil {
			return nil, apperrors.InternalServerError(err)
		}
		if profilePicture.Valid {
			if parsed, err := uuid.Parse(profilePicture.String); err == nil {
				m.Sender.ProfilePictureUUID = &parsed
			}
		}
		items = append(items, m)
	}
	if err := rows.Err(); err != nil {
		return nil, apperrors.InternalServerError(err)
	}

	feed := &models.MessageFeed{Items: items}
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

// UnreadCount returns how many messages from other participants are unread
// across all the viewer's conversations.
func (s *dmStore) UnreadCount(ctx context.Context, viewerID int) (int, error) {
	var count int
	err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM messages m
		JOIN conversations c ON c.conversation_id = m.conversation_id
		WHERE (c.participant_a = $1 OR c.participant_b = $1)
		  AND m.sender_id <> $1 AND m.read_at IS NULL`, viewerID).Scan(&count)
	if err != nil {
		return 0, apperrors.InternalServerError(err)
	}
	return count, nil
}

// MarkRead marks all messages in a conversation sent by the other participant
// as read, returning the number affected.
func (s *dmStore) MarkRead(ctx context.Context, conversationID, readerID int) (int, error) {
	result, err := s.db.ExecContext(ctx, `
		UPDATE messages m SET read_at = CURRENT_TIMESTAMP
		FROM conversations c
		WHERE m.conversation_id = c.conversation_id
		  AND c.conversation_id = $1
		  AND (c.participant_a = $2 OR c.participant_b = $2)
		  AND m.sender_id <> $2 AND m.read_at IS NULL`,
		conversationID, readerID)
	if err != nil {
		return 0, apperrors.InternalServerError(err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return 0, apperrors.InternalServerError(err)
	}
	return int(affected), nil
}

// IsParticipant reports whether the viewer belongs to the conversation.
func (s *dmStore) IsParticipant(ctx context.Context, conversationID, userID int) (bool, error) {
	var n int
	if err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM conversations
		WHERE conversation_id = $1 AND (participant_a = $2 OR participant_b = $2)`,
		conversationID, userID).Scan(&n); err != nil {
		return false, apperrors.InternalServerError(err)
	}
	return n > 0, nil
}

func (s *dmStore) hydrateSenders(ctx context.Context, messages []*models.Message) error {
	for _, m := range messages {
		var username, displayName string
		var profilePicture sql.NullString
		if err := s.db.QueryRowContext(ctx, `
			SELECT u.username, p.display_name, p.profile_picture_uuid
			FROM users u JOIN user_profiles p ON p.user_id = u.user_id
			WHERE u.user_id = $1`, m.SenderID).Scan(&username, &displayName, &profilePicture); err != nil {
			return apperrors.InternalServerError(err)
		}
		m.Sender = models.MessageSender{UserID: m.SenderID, Username: username, DisplayName: displayName}
		if profilePicture.Valid {
			if parsed, err := uuid.Parse(profilePicture.String); err == nil {
				m.Sender.ProfilePictureUUID = &parsed
			}
		}
	}
	return nil
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
