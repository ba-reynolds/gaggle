package service

import (
	"context"
	"log/slog"
	"strings"

	"github.com/ba-reynolds/gaggle/internal/apperrors"
	"github.com/ba-reynolds/gaggle/internal/models"
	"github.com/ba-reynolds/gaggle/internal/realtime"
	"github.com/ba-reynolds/gaggle/internal/store"
)

type DmService struct {
	store  *store.Store
	hub    *realtime.Hub
	logger *slog.Logger
}

func NewDmService(st *store.Store, hub *realtime.Hub, logger *slog.Logger) *DmService {
	return &DmService{store: st, hub: hub, logger: logger}
}

// Send delivers a 1:1 message to the recipient, creating the conversation on
// first contact. Blocked pairs (either direction) cannot message.
func (s *DmService) Send(ctx context.Context, senderID int, recipientUsername, body string) (*models.Message, error) {
	body = strings.TrimSpace(body)
	if body == "" || len(body) > 2000 {
		return nil, apperrors.BadRequestError("message body is required (max 2000 characters)", nil)
	}
	recipient, err := s.store.Users.GetByUsername(ctx, recipientUsername)
	if err != nil {
		return nil, err
	}
	if recipient.SoftDeleted {
		return nil, apperrors.NotFoundError("user not found", nil)
	}

	for _, blocked := range []struct {
		subjectID int
		targetID  int
	}{
		{subjectID: recipient.ID, targetID: senderID},
		{subjectID: senderID, targetID: recipient.ID},
	} {
		rel, err := s.store.UserRelationships.GetByIDs(ctx, blocked.subjectID, blocked.targetID)
		if err == nil && rel.RelationshipType == "block" {
			return nil, apperrors.ForbiddenError("you cannot message this user", nil)
		}
		if err != nil && !apperrors.Is(err, apperrors.NotFound) {
			return nil, err
		}
	}

	conversation, err := s.store.DMs.GetOrCreateConversation(ctx, senderID, recipient.ID)
	if err != nil {
		return nil, err
	}

	// Insert the message and bump last_message_at.
	message, err := s.store.DMs.AddMessage(ctx, conversation.ID, senderID, body)
	if err != nil {
		return nil, err
	}

	// Publish only after the writes land; the hub's stream.resync overflow path
	// covers dropped events.
	s.hub.Publish(recipient.ID, realtime.Event{
		Type: "dm.new",
		Payload: map[string]any{
			"conversation_id": conversation.ID,
			"sender_id":       senderID,
			"body":            body,
		},
	})
	s.publishUnreadCount(ctx, recipient.ID)
	s.publishUnreadCount(ctx, senderID)
	return message, nil
}

// ListConversations returns the viewer's inbox.
func (s *DmService) ListConversations(ctx context.Context, viewerID int) (*models.ConversationFeed, error) {
	return s.store.DMs.ListConversations(ctx, viewerID)
}

// ListMessages returns a page of a conversation's history, guarded to
// participants only.
func (s *DmService) ListMessages(ctx context.Context, viewerID, conversationID, limit int, cursor string) (*models.MessageFeed, error) {
	participant, err := s.store.DMs.IsParticipant(ctx, conversationID, viewerID)
	if err != nil {
		return nil, err
	}
	if !participant {
		return nil, apperrors.NotFoundError("conversation not found", nil)
	}
	normalized := validateLimit(limit, 20, 100)
	return s.store.DMs.ListMessages(ctx, conversationID, normalized, cursor)
}

// GetConversation guards participant access and returns a single conversation.
func (s *DmService) GetConversation(ctx context.Context, viewerID, conversationID int) (*models.Conversation, error) {
	conversation, err := s.store.DMs.GetConversation(ctx, conversationID, viewerID)
	if err != nil {
		return nil, err
	}
	if conversation.ParticipantA != viewerID && conversation.ParticipantB != viewerID {
		return nil, apperrors.NotFoundError("conversation not found", nil)
	}
	return conversation, nil
}

// UnreadCount returns the viewer's total unread DM count.
func (s *DmService) UnreadCount(ctx context.Context, viewerID int) (int, error) {
	return s.store.DMs.UnreadCount(ctx, viewerID)
}

// MarkRead marks incoming messages in a conversation as read and publishes the
// updated unread count to the reader.
func (s *DmService) MarkRead(ctx context.Context, viewerID, conversationID int) error {
	participant, err := s.store.DMs.IsParticipant(ctx, conversationID, viewerID)
	if err != nil {
		return err
	}
	if !participant {
		return apperrors.NotFoundError("conversation not found", nil)
	}
	if _, err := s.store.DMs.MarkRead(ctx, conversationID, viewerID); err != nil {
		return err
	}
	s.publishUnreadCount(ctx, viewerID)
	return nil
}

func (s *DmService) publishUnreadCount(ctx context.Context, viewerID int) {
	count, err := s.store.DMs.UnreadCount(ctx, viewerID)
	if err != nil {
		s.logger.Warn("failed to publish DM unread count", "viewer_id", viewerID, "error", err)
		return
	}
	s.hub.Publish(viewerID, realtime.Event{Type: "dm.unread", Payload: map[string]int{"unread_count": count}})
}
