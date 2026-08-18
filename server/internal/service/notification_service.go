package service

import (
	"context"
	"log/slog"
	"regexp"

	"github.com/ba-reynolds/gophersocial/internal/apperrors"
	"github.com/ba-reynolds/gophersocial/internal/models"
	"github.com/ba-reynolds/gophersocial/internal/realtime"
	"github.com/ba-reynolds/gophersocial/internal/store"
)

type NotificationService struct {
	store  *store.Store
	hub    *realtime.Hub
	logger *slog.Logger
}

var mentionPattern = regexp.MustCompile(`@([A-Za-z0-9_]{3,16})\b`)

func NewNotificationService(st *store.Store, hub *realtime.Hub, logger *slog.Logger) *NotificationService {
	return &NotificationService{store: st, hub: hub, logger: logger}
}

func (s *NotificationService) CreateForPost(ctx context.Context, actorID, postID int, notificationType string) error {
	post, err := s.store.Posts.GetByID(ctx, postID)
	if err != nil {
		return err
	}
	return s.Create(ctx, actorID, post.AuthorID, notificationType, &postID)
}

func (s *NotificationService) Create(ctx context.Context, actorID, recipientID int, notificationType string, postID *int) error {
	if actorID == recipientID {
		return nil
	}
	relationship, err := s.store.UserRelationships.GetByIDs(ctx, recipientID, actorID)
	if err == nil && relationship.RelationshipType == "block" {
		return nil
	}
	if err != nil && !apperrors.Is(err, apperrors.NotFound) {
		return err
	}
	// Muting silences the muted user's notifications (likes, replies,
	// mentions, follows) without blocking their content.
	muted, err := s.store.UserRelationships.Exists(ctx, recipientID, actorID, "mute")
	if err != nil {
		return err
	}
	if muted {
		return nil
	}
	if notificationType == "mention" {
		settings, err := s.store.Users.GetSettings(ctx, recipientID)
		if err != nil {
			return err
		}
		if !settings.Notifications.Mentions {
			return nil
		}
	}
	tx, err := s.store.DB.BeginTx(ctx, nil)
	if err != nil {
		return apperrors.InternalServerError(err)
	}
	defer tx.Rollback()

	notification := &models.Notification{}
	if err := s.store.Notifications.Create(ctx, tx, notification, recipientID, actorID, notificationType, postID); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return apperrors.InternalServerError(err)
	}

	s.publishUnreadCount(ctx, recipientID)
	return nil
}

func (s *NotificationService) CreateMentionNotifications(ctx context.Context, actorID, postID int, content string) error {
	seen := make(map[int]struct{})
	for _, match := range mentionPattern.FindAllStringSubmatch(content, -1) {
		user, err := s.store.Users.GetByUsername(ctx, match[1])
		if err != nil {
			if apperrors.Is(err, apperrors.NotFound) {
				continue
			}
			return err
		}
		if _, ok := seen[user.ID]; ok {
			continue
		}
		seen[user.ID] = struct{}{}
		if err := s.Create(ctx, actorID, user.ID, "mention", &postID); err != nil {
			return err
		}
	}
	return nil
}

func (s *NotificationService) List(ctx context.Context, recipientID, limit int, cursor string) (*models.NotificationFeed, error) {
	return s.store.Notifications.List(ctx, recipientID, limit, cursor)
}

func (s *NotificationService) UnreadCount(ctx context.Context, recipientID int) (int, error) {
	return s.store.Notifications.UnreadCount(ctx, recipientID)
}

func (s *NotificationService) MarkRead(ctx context.Context, recipientID, notificationID int) error {
	if err := s.store.Notifications.MarkRead(ctx, recipientID, notificationID); err != nil {
		return err
	}
	s.publishUnreadCount(ctx, recipientID)
	return nil
}

func (s *NotificationService) MarkAllRead(ctx context.Context, recipientID int) error {
	if err := s.store.Notifications.MarkAllRead(ctx, recipientID); err != nil {
		return err
	}
	s.publishUnreadCount(ctx, recipientID)
	return nil
}

func (s *NotificationService) PublishFeedPost(ctx context.Context, authorID, postID int) error {
	recipients, err := s.store.UserRelationships.GetFollowerIDs(ctx, authorID)
	if err != nil {
		return err
	}
	recipients = append(recipients, authorID)
	for _, recipientID := range recipients {
		s.hub.Publish(recipientID, realtime.Event{
			Type:    "feed.post_created",
			Payload: map[string]int{"post_id": postID, "author_id": authorID},
		})
	}
	return nil
}

func (s *NotificationService) publishUnreadCount(ctx context.Context, recipientID int) {
	count, err := s.store.Notifications.UnreadCount(ctx, recipientID)
	if err != nil {
		s.logger.Warn("failed to publish notification count", "recipient_id", recipientID, "error", err)
		return
	}
	s.hub.Publish(recipientID, realtime.Event{Type: "notification.new", Payload: map[string]int{"unread_count": count}})
}
