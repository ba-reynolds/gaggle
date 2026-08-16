package models

import (
	"time"
)

// Like represents a like on a post
type PostLike struct {
	LikeID    int       `json:"like_id"`
	PostID    int       `json:"post_id"`
	UserID    int       `json:"user_id"`
	CreatedAt time.Time `json:"created_at"`
}

type LikeRequest struct {
	PostID int `json:"post_id" validate:"required"`
}

type LikeResponse struct {
	Success bool `json:"success"`
}

// Repost represents a repost (not a quote)
type PostRepost struct {
	RepostID       int       `json:"repost_id"`
	OriginalPostID int       `json:"original_post_id"`
	UserID         int       `json:"user_id"`
	CreatedAt      time.Time `json:"created_at"`
}

type RepostRequest struct {
	OriginalPostID int `json:"original_post_id" validate:"required"`
}

type RepostResponse struct {
	Success bool `json:"success"`
}

// Bookmark represents a bookmark on a post
// category_id is optional
type PostBookmark struct {
	BookmarkID int       `json:"bookmark_id"`
	PostID     int       `json:"post_id"`
	UserID     int       `json:"user_id"`
	CategoryID *int      `json:"category_id,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
}

type BookmarkRequest struct {
	CategoryID *int `json:"category_id,omitempty"`
}

type BookmarkResponse struct {
	Success bool `json:"success"`
}

// View represents a view on a post
type PostView struct {
	ViewID    int       `json:"view_id"`
	PostID    int       `json:"post_id"`
	UserID    *int      `json:"user_id,omitempty"`
	IPAddress string    `json:"ip_address,omitempty"`
	UserAgent string    `json:"user_agent,omitempty"`
	ViewedAt  time.Time `json:"viewed_at"`
}

// No request/response for view, as it's usually tracked passively

// BookmarkCategory represents a user's bookmark category
// See: 000007_create-post-engagement.up.sql
// color is a hex string (e.g. #1DA1F2)
type BookmarkCategory struct {
	CategoryID     int       `json:"id"`
	UserID         int       `json:"user_id"`
	CategoryName   string    `json:"name"`
	Color          string    `json:"color"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
	BookmarksCount int       `json:"post_count"`
}

type CreateBookmarkCategoryRequest struct {
	CategoryName string `json:"name" validate:"required,min=1,max=50"`
	Color        string `json:"color" validate:"omitempty,len=7"`
}

type CreateBookmarkCategoryResponse struct {
	Success  bool             `json:"success"`
	Category BookmarkCategory `json:"category"`
}
