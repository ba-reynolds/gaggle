package models

import "time"

type Notification struct {
	ID        int        `json:"id"`
	Type      string     `json:"type"`
	Actor     PostAuthor `json:"actor"`
	PostID    *int       `json:"post_id,omitempty"`
	ReadAt    *time.Time `json:"read_at,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
}

type NotificationFeed struct {
	Items      []Notification `json:"items"`
	HasMore    bool           `json:"has_more"`
	NextCursor string         `json:"next_cursor,omitempty"`
}

func (n *NotificationFeed) GetHasMore() bool      { return n.HasMore }
func (n *NotificationFeed) GetNextCursor() string { return n.NextCursor }
