package service

import (
	"context"
	"database/sql"
	"log/slog"
	"mime/multipart"

	"github.com/ba-reynolds/gophersocial/internal/auth"
	"github.com/ba-reynolds/gophersocial/internal/models"
	"github.com/ba-reynolds/gophersocial/internal/store"
	"github.com/ba-reynolds/gophersocial/pkg/config"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

type Service struct {
	// TODO: add tx for very CUD operation
	DB     *sql.DB
	Config config.AppConfig
	Auth   interface {
		Login(ctx context.Context, identifier string, password string, ipAddress string, userAgent string) (*models.Token, *models.Token, error)
		Register(ctx context.Context, username string, email string, password string, ipAddress string, userAgent string) (*models.User, *models.Token, *models.Token, error)
		RefreshToken(ctx context.Context, refreshTokenString string) (*models.Token, error)
		Logout(ctx context.Context, tokenHash string) error
		ValidateToken(tokenString string, tokenType auth.TokenType) (*jwt.Token, error)
		GetUserIDFromToken(token *jwt.Token) (int, error)
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
		GetParentChain(ctx context.Context, postID int, viewerID int, limit int, cursor string) (*models.PostChain, error)
		GetDescendants(ctx context.Context, postID int, viewerID int, limit int, cursor string) (*models.PostDescendants, error)
		GetHomeFeed(ctx context.Context, userID int, limit int, cursor string) (*models.PostFeed, error)
		GetUserFeed(ctx context.Context, userID int, viewerID int, includeReplies bool, limit int, cursor string) (*models.PostFeed, error)
		GetBookmarkedPostsFeed(ctx context.Context, userID int, viewerID int, categoryIDs []int, limit int, cursor string) (*models.PostFeed, error)
		GetLikedPostsFeed(ctx context.Context, userID int, viewerID int, limit int, cursor string) (*models.PostFeed, error)
		GetQuotesFeed(ctx context.Context, postID int, viewerID int, limit int, cursor string) (*models.PostFeed, error)
		GetPostLikers(ctx context.Context, postID int, limit int, cursor string) (*models.UserList, error)
		GetPostReposters(ctx context.Context, postID int, limit int, cursor string) (*models.UserList, error)
	}
	PostEngagements interface {
		Like(ctx context.Context, postID, userID int) error
		Unlike(ctx context.Context, postID, userID int) error
		Repost(ctx context.Context, postID, userID int) error
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
		DeleteRelationship(ctx context.Context, followerID, followingID int) error
		GetFollowers(ctx context.Context, userID int, limit int, cursor string) (*models.UserFollowersResponse, error)
		GetFollowing(ctx context.Context, userID int, limit int, cursor string) (*models.UserFollowingResponse, error)
		GetRelationshipStatus(ctx context.Context, followerID, followingID int) (*models.RelationshipStatus, error)
		GetFollowerIDs(ctx context.Context, userID int) ([]int, error)
	}
}

// NewService creates a new service with all required dependencies
func NewService(store *store.Store, logger *slog.Logger, authenticator *auth.JWTAuthenticator, config config.AppConfig) *Service {
	return &Service{
		DB:                store.DB,
		Config:            config,
		Auth:              &AuthService{store, authenticator, logger},
		Users:             &UserService{store, logger},
		Media:             &MediaService{store, logger},
		Posts:             &PostService{store, logger, config},
		PostEngagements:   NewPostEngagementService(store, logger),
		UserRelationships: &UserRelationshipService{store, logger},
	}
}
