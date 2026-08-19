package service

import (
	"context"
	"database/sql"
	"log/slog"
	"mime/multipart"

	"github.com/ba-reynolds/gophersocial/internal/auth"
	"github.com/ba-reynolds/gophersocial/internal/models"
	"github.com/ba-reynolds/gophersocial/internal/realtime"
	"github.com/ba-reynolds/gophersocial/internal/store"
	"github.com/ba-reynolds/gophersocial/pkg/config"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

type Service struct {
	// TODO: add tx for very CUD operation
	DB       *sql.DB
	Config   config.AppConfig
	Realtime *realtime.Hub
	Auth     interface {
		Login(ctx context.Context, identifier string, password string, ipAddress string, userAgent string) (*models.Token, *models.Token, error)
		Register(ctx context.Context, username string, email string, password string, ipAddress string, userAgent string) (*models.User, *models.Token, *models.Token, error)
		RefreshToken(ctx context.Context, refreshTokenString string, ipAddress string, userAgent string) (*models.Token, *models.Token, error)
		Logout(ctx context.Context, tokenHash string) error
		ValidateToken(tokenString string, tokenType auth.TokenType) (*jwt.Token, error)
		GetUserIDFromToken(token *jwt.Token) (int, error)
		GetUserIDFromRefreshToken(ctx context.Context, tokenString string) (int, error)
	}
	Users interface {
		GetByID(ctx context.Context, id int) (*models.User, error)
		GetByEmail(ctx context.Context, email string) (*models.User, error)
		GetByUsername(ctx context.Context, username string) (*models.User, error)
		CreateUser(ctx context.Context, user *models.User) error
		UpdateUserProfile(ctx context.Context, user *models.UserWithProfile) error
		GetUserProfileByUsername(ctx context.Context, username string) (*models.UserWithProfile, error)
		GetSettings(ctx context.Context, userID int) (*models.UserSettings, error)
		UpdateSettings(ctx context.Context, userID int, settings *models.UserSettings) error
		Suggested(ctx context.Context, viewerID int, limit int) (*models.UserList, error)
	}
	Media interface {
		Create(ctx context.Context, media *models.Media, file multipart.File) error
		GetByID(ctx context.Context, id uuid.UUID) (*models.Media, error)
		DeleteByID(ctx context.Context, id uuid.UUID) error
	}
	Posts interface {
		GetByID(ctx context.Context, id int) (*models.Post, error)
		GetFullPostByID(ctx context.Context, id int, viewerID int) (*models.FullPost, error)
		GetFullPostByIDWithAncestors(ctx context.Context, id int, viewerID int, includeAncestors bool, ancestorLimit int) (*models.FullPost, *models.PostChain, error)
		GetFullPostByIDWithAncestorsAndDescendants(ctx context.Context, id int, viewerID int, includeAncestors bool, ancestorLimit int, includeDescendants bool, descendantLimit int) (*models.FullPost, *models.PostChain, *models.PostDescendants, error)
		Create(ctx context.Context, post *models.Post, mediaItems []models.PostMediaRequest) error
		QuotePost(ctx context.Context, post *models.Post, mediaItems []models.PostMediaRequest) error
		DeleteByID(ctx context.Context, post *models.Post, actorID int) error
		Update(ctx context.Context, post *models.Post, actorID int, content string) (*models.Post, error)
		Pin(ctx context.Context, post *models.Post, actorID int) error
		Unpin(ctx context.Context, post *models.Post, actorID int) error
		GetPinned(ctx context.Context, authorID, viewerID int) (*models.FullPost, error)
		ListEdits(ctx context.Context, postID int) (*models.PostEditHistory, error)
		VotePoll(ctx context.Context, postID, optionID, userID int) (*models.Poll, error)
		GetParentChain(ctx context.Context, postID int, viewerID int, limit int, cursor string) (*models.PostChain, error)
		GetDescendants(ctx context.Context, postID int, viewerID int, limit int, cursor string) (*models.PostDescendants, error)
		GetHomeFeed(ctx context.Context, userID int, limit int, cursor string) (*models.PostFeed, error)
		GetUserFeed(ctx context.Context, userID int, viewerID int, includeReplies bool, limit int, cursor string) (*models.PostFeed, error)
		GetUserRepliesFeed(ctx context.Context, userID int, viewerID int, limit int, cursor string) (*models.PostFeed, error)
		GetUserMediaFeed(ctx context.Context, userID int, viewerID int, limit int, cursor string) (*models.PostFeed, error)
		GetBookmarkedPostsFeed(ctx context.Context, userID int, viewerID int, categoryIDs []int, limit int, cursor string) (*models.PostFeed, error)
		GetLikedPostsFeed(ctx context.Context, userID int, viewerID int, limit int, cursor string) (*models.PostFeed, error)
		GetQuotesFeed(ctx context.Context, postID int, viewerID int, limit int, cursor string) (*models.PostFeed, error)
		GetPostLikers(ctx context.Context, postID int, limit int, cursor string) (*models.UserList, error)
		GetPostReposters(ctx context.Context, postID int, limit int, cursor string) (*models.UserList, error)
	}
	PostEngagements interface {
		Like(ctx context.Context, postID, userID int) (bool, error)
		Unlike(ctx context.Context, postID, userID int) error
		Repost(ctx context.Context, postID, userID int) (bool, error)
		Unrepost(ctx context.Context, postID, userID int) error
		Bookmark(ctx context.Context, postID, userID int, categoryID *int) error
		Unbookmark(ctx context.Context, postID, userID int) error
		AddView(ctx context.Context, postID int, userID *int, ipAddress, userAgent string) error
		CreateBookmarkCategory(ctx context.Context, userID int, categoryName, color string) (*models.BookmarkCategory, error)
		ListBookmarkCategories(ctx context.Context, userID int) ([]models.BookmarkCategory, error)
		DeleteBookmarkCategory(ctx context.Context, userID, categoryID int) error
	}
	UserRelationships interface {
		CreateRelationship(ctx context.Context, followerID, followingID int, relationshipType string) (*models.UserRelationship, error)
		DeleteRelationship(ctx context.Context, followerID, followingID int, relationshipType string) error
		GetFollowers(ctx context.Context, userID int, limit int, cursor string) (*models.UserFollowersResponse, error)
		GetFollowing(ctx context.Context, userID int, limit int, cursor string) (*models.UserFollowingResponse, error)
		GetRelationshipStatus(ctx context.Context, followerID, followingID int) (*models.RelationshipStatus, error)
		GetRelationshipStatuses(ctx context.Context, viewerID int, targetIDs []int) (map[int]*models.RelationshipStatus, error)
		GetFollowerIDs(ctx context.Context, userID int) ([]int, error)
	}
	Notifications interface {
		CreateForPost(ctx context.Context, actorID, postID int, notificationType string) error
		Create(ctx context.Context, actorID, recipientID int, notificationType string, postID *int) error
		List(ctx context.Context, recipientID, limit int, cursor string) (*models.NotificationFeed, error)
		UnreadCount(ctx context.Context, recipientID int) (int, error)
		MarkRead(ctx context.Context, recipientID, notificationID int) error
		MarkAllRead(ctx context.Context, recipientID int) error
		PublishFeedPost(ctx context.Context, authorID, postID int) error
		CreateMentionNotifications(ctx context.Context, actorID, postID int, content string) error
	}
	Search interface {
		Posts(ctx context.Context, viewerID int, query string, filters models.PostSearchFilters, limit int, cursor string) (*models.PostFeed, error)
		Users(ctx context.Context, query string, limit int) (*models.UserList, error)
		HashtagPosts(ctx context.Context, viewerID int, name string, limit int, cursor string) (*models.PostFeed, error)
		Mentions(ctx context.Context, viewerID int, limit int, cursor string) (*models.PostFeed, error)
		Trends(ctx context.Context, limit int) ([]models.Trend, error)
	}
	Badges interface {
		GetBadgesForUsers(ctx context.Context, ids []int) (map[int][]models.UserBadge, error)
		HydrateProfiles(ctx context.Context, profiles []models.UserProfileResponse) error
		HydrateUserWithProfile(ctx context.Context, user *models.UserWithProfile) error
		HydrateUserWithProfiles(ctx context.Context, users []models.UserWithProfile) error
		ListCatalog(ctx context.Context) ([]models.Badge, error)
		CreateBadge(ctx context.Context, payload models.CreateBadgePayload) (*models.Badge, error)
		UpdateBadge(ctx context.Context, badgeID int, payload models.CreateBadgePayload) (*models.Badge, error)
		DeleteBadge(ctx context.Context, badgeID int) error
		GrantBadge(ctx context.Context, username string, badgeID, grantedBy int) error
		RevokeBadge(ctx context.Context, username string, badgeID int) error
	}
	Lists interface {
		Create(ctx context.Context, ownerID int, payload models.CreateListPayload) (*models.List, error)
		Get(ctx context.Context, listID int) (*models.List, error)
		ListForUser(ctx context.Context, userID int) ([]models.List, error)
		Update(ctx context.Context, listID, actorID int, payload models.CreateListPayload) (*models.List, error)
		Delete(ctx context.Context, listID, actorID int) error
		AddMember(ctx context.Context, listID, actorID, memberID int) error
		RemoveMember(ctx context.Context, listID, actorID, memberID int) error
		GetMembers(ctx context.Context, listID, limit int, cursor string) (*models.ListMembersResponse, error)
		GetListFeed(ctx context.Context, viewerID, listID, limit int, cursor string) (*models.PostFeed, error)
	}
	DMs interface {
		Send(ctx context.Context, senderID int, recipientUsername, body string) (*models.Message, error)
		ListConversations(ctx context.Context, viewerID int) (*models.ConversationFeed, error)
		ListMessages(ctx context.Context, viewerID, conversationID, limit int, cursor string) (*models.MessageFeed, error)
		GetConversation(ctx context.Context, viewerID, conversationID int) (*models.Conversation, error)
		UnreadCount(ctx context.Context, viewerID int) (int, error)
		MarkRead(ctx context.Context, viewerID, conversationID int) error
	}
}

// NewService creates a new service with all required dependencies
func NewService(store *store.Store, logger *slog.Logger, authenticator *auth.JWTAuthenticator, config config.AppConfig) *Service {
	hub := realtime.NewHub()
	return &Service{
		DB:                store.DB,
		Config:            config,
		Realtime:          hub,
		Auth:              &AuthService{store, authenticator, logger},
		Users:             &UserService{store, logger},
		Media:             &MediaService{store, logger},
		Posts:             &PostService{store, logger, config},
		PostEngagements:   NewPostEngagementService(store, logger),
		UserRelationships: &UserRelationshipService{store, logger},
		Notifications:     NewNotificationService(store, hub, logger),
		Search:            NewSearchService(store, logger, config),
		Badges:            NewBadgeService(store, logger),
		Lists:             NewListService(store, logger),
		DMs:               NewDmService(store, hub, logger),
	}
}
