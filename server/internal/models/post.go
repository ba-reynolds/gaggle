package models

import (
	"time"

	"github.com/google/uuid"
)

type PostAuthor struct {
	Username           string     `json:"username"`
	DisplayName        string     `json:"display_name"`
	ProfilePictureUUID *uuid.UUID `json:"profile_picture_uuid,omitempty"`
}

// ToPostAuthor converts a user profile into the author representation,
// omitting the zero UUID so clients don't build broken media URLs.
func ToPostAuthor(username, displayName string, profilePicture uuid.UUID) PostAuthor {
	author := PostAuthor{Username: username, DisplayName: displayName}
	if profilePicture != uuid.Nil {
		pp := profilePicture
		author.ProfilePictureUUID = &pp
	}
	return author
}

type Post struct {
	ID      int    `json:"id"`
	Content string `json:"content"`
	// Author ID is used for performance reasons, for example, if the user wants to delete
	// a post, we don't have to make a join with the users table
	AuthorID int `json:"-"`
	// Visibility controls who may see this post: "public" (everyone),
	// "followers" (the author + their followers), or "mentions" (the author +
	// users @mentioned in the content).
	Visibility string `json:"visibility"`
	// MentionedUserIDs is the resolved user id set of @mentions in the content,
	// used by the "mentions" visibility rule. Never serialized.
	MentionedUserIDs []int `json:"-"`
	ParentID         *int  `json:"parent_id"`
	SoftDeleted      bool  `json:"-"`
	SoftDeletedAt  *time.Time         `json:"-"`
	CreatedAt      time.Time          `json:"created_at"`
	UpdatedAt      time.Time          `json:"updated_at"`
	EditedAt       *time.Time         `json:"edited_at,omitempty"`
	IsPinned       bool               `json:"is_pinned"`
	LikesCount     int                `json:"-"`
	RepostsCount   int                `json:"-"`
	QuotesCount    int                `json:"-"`
	BookmarksCount int                `json:"-"`
	ViewsCount     int                `json:"-"`
	RepliesCount   int                `json:"-"`
	QuotedPostID   *int               `json:"quoted_post_id"`
	PollPayload    *CreatePollPayload `json:"-"`
}

// PostEngagement captures the per-viewer engagement state of a post:
// whether the requesting user has liked/reposted/bookmarked it, plus the
// aggregate counts. The flat count fields on Post are omitted from JSON in
// favor of this single, frontend-friendly object.
type PostEngagement struct {
	IsLiked          bool                     `json:"is_liked"`
	IsReposted       bool                     `json:"is_reposted"`
	IsBookmarked     bool                     `json:"is_bookmarked"`
	LikeCount        int                      `json:"like_count"`
	RepostCount      int                      `json:"repost_count"`
	ReplyCount       int                      `json:"reply_count"`
	ViewCount        int                      `json:"view_count"`
	BookmarkCount    int                      `json:"bookmark_count"`
	QuoteCount       int                      `json:"quote_count"`
	BookmarkCategory *BookmarkCategorySummary `json:"bookmark_category,omitempty"`
}

// BookmarkCategorySummary is the lite representation of a bookmark category
// embedded inside a post's engagement object.
type BookmarkCategorySummary struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// Position is inferred from the index of the media in the payload
type PostMediaRequest struct {
	UUID    string `json:"uuid" validate:"required,uuid"`
	AltText string `json:"alt_text" validate:"max=200"`
}

type CreatePostPayload struct {
	Content  string             `json:"content"`
	Media    []PostMediaRequest `json:"media" validate:"dive"`
	ParentID *int               `json:"parent_id"`
	Poll     *CreatePollPayload `json:"poll,omitempty"`
	// Visibility is one of "public" | "followers" | "mentions". Empty defaults
	// to "public" (see PostService.Create).
	Visibility string `json:"visibility"`
}

type UpdatePostPayload struct {
	Content string `json:"content"`
}

type CreatePollPayload struct {
	Question string     `json:"question"`
	Options  []string   `json:"options"`
	EndsAt   *time.Time `json:"ends_at,omitempty"`
}

type Poll struct {
	ID               int          `json:"id"`
	Question         string       `json:"question"`
	EndsAt           *time.Time   `json:"ends_at,omitempty"`
	Options          []PollOption `json:"options"`
	TotalVotes       int          `json:"total_votes"`
	SelectedOptionID *int         `json:"selected_option_id,omitempty"`
	Closed           bool         `json:"closed"`
}

type PollOption struct {
	ID        int    `json:"id"`
	Label     string `json:"label"`
	Position  int    `json:"position"`
	VoteCount int    `json:"vote_count"`
}

type PostEdit struct {
	ID            int       `json:"id"`
	ContentBefore string    `json:"content_before"`
	EditedAt      time.Time `json:"edited_at"`
}

// PostParentInfo describes the post a reply is replying to. Replies carry it
// so the UI can render a "Replying to @author" line without an extra request;
// a reply whose parent was soft-deleted (or is missing) reports Deleted=true
// and no Author so the UI degrades gracefully.
type PostParentInfo struct {
	ID      int         `json:"id"`
	Deleted bool        `json:"deleted"`
	Author  *PostAuthor `json:"author,omitempty"`
}

type PostEditHistory struct {
	Items []PostEdit `json:"items"`
}

// PostMedia represents the association between a Post and a Media
// It records display order (position) and alt text for accessibility
// Stored in the post_media table with a composite PK of (post_id, media_uuid).
type PostMedia struct {
	PostID    int       `json:"-"`
	MediaUUID uuid.UUID `json:"uuid"`
	Position  int       `json:"-"`
	AltText   string    `json:"alt_text"`
}

// FullPost represents a post with author, media, and per-viewer engagement
type FullPost struct {
	Post
	Author     PostAuthor      `json:"author"`
	Media      []PostMedia     `json:"media"`
	Engagement *PostEngagement `json:"engagement"`
	Poll       *Poll           `json:"poll,omitempty"`
	Parent     *PostParentInfo `json:"parent,omitempty"`
}

// PostChain represents a chain of parent posts up to a certain limit
type PostChain struct {
	Items      []*FullPost `json:"items"`
	HasMore    bool        `json:"has_more"`
	NextCursor string      `json:"next_cursor,omitempty"`
}

// PostDescendants represents direct replies to a post
type PostDescendants struct {
	Items      []*FullPost `json:"items"`
	HasMore    bool        `json:"has_more"`
	NextCursor string      `json:"next_cursor,omitempty"`
}

// Implement PaginatedResponse interface
func (pc *PostChain) GetHasMore() bool {
	return pc.HasMore
}

func (pc *PostChain) GetNextCursor() string {
	return pc.NextCursor
}

func (pd *PostDescendants) GetHasMore() bool {
	return pd.HasMore
}

func (pd *PostDescendants) GetNextCursor() string {
	return pd.NextCursor
}

// PostWithAncestors represents a post with its ancestor chain
type PostWithAncestors struct {
	Post      *FullPost  `json:"post"`
	Ancestors *PostChain `json:"ancestors,omitempty"`
}

// PostWithDescendants represents a post with its direct replies
type PostWithDescendants struct {
	Post        *FullPost        `json:"post"`
	Descendants *PostDescendants `json:"descendants,omitempty"`
}

// PostWithAncestorsAndDescendants represents a post with both ancestor and descendant chains
type PostWithAncestorsAndDescendants struct {
	Post        *FullPost        `json:"post"`
	Ancestors   *PostChain       `json:"ancestors,omitempty"`
	Descendants *PostDescendants `json:"descendants,omitempty"`
}

// PostFeed represents a paginated feed of posts
type PostFeed struct {
	Items      []*FullPost `json:"items"`
	HasMore    bool        `json:"has_more"`
	NextCursor string      `json:"next_cursor,omitempty"`
}

// Implement PaginatedResponse interface
func (pf *PostFeed) GetHasMore() bool {
	return pf.HasMore
}

func (pf *PostFeed) GetNextCursor() string {
	return pf.NextCursor
}
