package models

import (
	"time"

	"github.com/google/uuid"
)

// Conversation is a 1:1 private conversation between two users.
type Conversation struct {
	ID               int             `json:"id"`
	ParticipantA     int             `json:"-"`
	ParticipantB     int             `json:"-"`
	CreatedAt        time.Time       `json:"created_at"`
	LastMessageAt    time.Time       `json:"last_message_at"`
	OtherParticipant *UserAPISummary `json:"other_participant,omitempty"`
	LastMessage      *Message        `json:"last_message,omitempty"`
	UnreadCount      int             `json:"unread_count"`
}

// UserAPISummary is the lean author-shaped object attached to conversations.
type UserAPISummary struct {
	UserID             int        `json:"-"`
	Username           string     `json:"username"`
	DisplayName        string     `json:"display_name"`
	ProfilePictureUUID *uuid.UUID `json:"profile_picture_uuid,omitempty"`
}

// MessageSender is the author field on a message.
type MessageSender struct {
	UserID             int        `json:"-"`
	Username           string     `json:"username"`
	DisplayName        string     `json:"display_name"`
	ProfilePictureUUID *uuid.UUID `json:"profile_picture_uuid,omitempty"`
}

// Message is a single DM with its sender summary.
type Message struct {
	ID             int           `json:"id"`
	ConversationID int           `json:"conversation_id"`
	SenderID       int           `json:"sender_id"`
	Sender         MessageSender `json:"sender"`
	Body           string        `json:"body"`
	ReadAt         *time.Time    `json:"read_at,omitempty"`
	CreatedAt      time.Time     `json:"created_at"`
}

// SendMessagePayload is the body for POST /dms/{username}.
type SendMessagePayload struct {
	Body string `json:"body" validate:"required,min=1,max=2000"`
}

// ConversationFeed is the list of a user's conversations (inbox).
type ConversationFeed struct {
	Items []Conversation `json:"items"`
}

// MessageFeed is a paginated message history for one conversation.
type MessageFeed struct {
	Items      []Message `json:"items"`
	NextCursor string    `json:"next_cursor,omitempty"`
	HasMore    bool      `json:"has_more"`
}

// GetHasMore implements the PaginatedResponse interface.
func (mf *MessageFeed) GetHasMore() bool { return mf.HasMore }

// GetNextCursor implements the PaginatedResponse interface.
func (mf *MessageFeed) GetNextCursor() string { return mf.NextCursor }
