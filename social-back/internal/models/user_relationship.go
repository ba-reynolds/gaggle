package models

import (
	"time"
)

// UserRelationship represents a relationship between two users
type UserRelationship struct {
	RelationshipID   int       `json:"relationship_id"`
	FollowerID       int       `json:"follower_id"`
	FollowingID      int       `json:"following_id"`
	RelationshipType string    `json:"relationship_type" validate:"required,oneof=follow block"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

// UserRelationshipRequest represents a request to create or update a relationship
type UserRelationshipRequest struct {
	RelationshipType string `json:"relationship_type" validate:"required,oneof=follow block"`
}

// UserRelationshipResponse represents a response with relationship information
type UserRelationshipResponse struct {
	Success bool `json:"success"`
}

// UserFollowersResponse represents a paginated response of followers
type UserFollowersResponse struct {
	Followers  []UserWithProfile `json:"followers"`
	HasMore    bool              `json:"has_more"`
	NextCursor string            `json:"next_cursor,omitempty"`
}

// UserFollowingResponse represents a paginated response of following users
type UserFollowingResponse struct {
	Following  []UserWithProfile `json:"following"`
	HasMore    bool              `json:"has_more"`
	NextCursor string            `json:"next_cursor,omitempty"`
}

// Implement PaginatedResponse interface
func (ufr *UserFollowersResponse) GetHasMore() bool {
	return ufr.HasMore
}

func (ufr *UserFollowersResponse) GetNextCursor() string {
	return ufr.NextCursor
}

func (ufr *UserFollowingResponse) GetHasMore() bool {
	return ufr.HasMore
}

func (ufr *UserFollowingResponse) GetNextCursor() string {
	return ufr.NextCursor
}

// RelationshipStatus represents the current relationship status between two users
type RelationshipStatus struct {
	IsFollowing bool `json:"is_following"`
	IsBlocked   bool `json:"is_blocked"`
}
