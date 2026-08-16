package models

// PaginationParams represents common pagination parameters
type PaginationParams struct {
	Limit  int    `json:"limit"`
	Cursor string `json:"cursor,omitempty"`
}

// PaginationResponse represents a common pagination response structure
type PaginationResponse struct {
	HasMore    bool   `json:"has_more"`
	NextCursor string `json:"next_cursor,omitempty"`
}

// CursorPagination represents cursor-based pagination with metadata
type CursorPagination struct {
	Limit      int    `json:"limit"`
	Cursor     string `json:"cursor,omitempty"`
	HasMore    bool   `json:"has_more"`
	NextCursor string `json:"next_cursor,omitempty"`
}

// PaginatedResponse is a generic interface for paginated responses
type PaginatedResponse interface {
	GetHasMore() bool
	GetNextCursor() string
}
