package models

import "time"

// List is a public, owner-managed collection of users.
type List struct {
	ID            int       `json:"id"`
	OwnerID       int       `json:"owner_id"`
	OwnerUsername string    `json:"owner_username"`
	Name          string    `json:"name"`
	Description   string    `json:"description"`
	MemberCount   int       `json:"member_count"`
	CreatedAt     time.Time `json:"created_at"`
}

// CreateListPayload is the body for creating a list.
type CreateListPayload struct {
	Name        string `json:"name" validate:"required,min=1,max=100"`
	Description string `json:"description" validate:"max=300"`
}

// ListMembersResponse is a paginated list of the users in a list.
type ListMembersResponse struct {
	Items      []UserProfileResponse `json:"items"`
	NextCursor string                `json:"next_cursor,omitempty"`
	HasMore    bool                  `json:"has_more"`
}

// GetHasMore implements the PaginatedResponse interface.
func (l *ListMembersResponse) GetHasMore() bool { return l.HasMore }

// GetNextCursor implements the PaginatedResponse interface.
func (l *ListMembersResponse) GetNextCursor() string { return l.NextCursor }
