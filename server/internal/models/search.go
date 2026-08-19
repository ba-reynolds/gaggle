package models

import "time"

type Trend struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

// PostSearchFilters narrows the post search endpoint. Empty values are
// ignored; Since/Until bound the post's created_at (inclusive start,
// exclusive end).
type PostSearchFilters struct {
	From           string
	Hashtag        string
	HasMedia       bool
	MinLikes       int
	IncludeReplies bool
	Since          *time.Time
	Until          *time.Time
}
