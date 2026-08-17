package store

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"mime/multipart"
	"time"

	"github.com/ba-reynolds/gophersocial/internal/models"
	"github.com/ba-reynolds/gophersocial/pkg/config"
	"github.com/google/uuid"
	_ "github.com/jackc/pgx/v5/stdlib"
)

type Store struct {
	// Raw db for transactions
	DB   *sql.DB
	Auth interface {
		CreateRefreshToken(ctx context.Context, tx *sql.Tx, refreshToken *models.RefreshToken) error
		GetRefreshToken(ctx context.Context, tokenHash string) (*models.RefreshToken, error)
		MarkRefreshTokenAsRevoked(ctx context.Context, tx *sql.Tx, tokenHash string) error
	}
	Users interface {
		Create(context.Context, *sql.Tx, *models.User) error
		GetByID(context.Context, int) (*models.User, error)
		GetByEmail(context.Context, string) (*models.User, error)
		GetByUsername(context.Context, string) (*models.User, error)
		GetUserProfileByUsername(ctx context.Context, username string) (*models.UserWithProfile, error)
		UpdateUserProfile(ctx context.Context, tx *sql.Tx, user *models.UserWithProfile) error
		GetSettings(ctx context.Context, userID int) (*models.UserSettings, error)
		UpdateSettings(ctx context.Context, userID int, settings *models.UserSettings) error
		Search(ctx context.Context, query string, limit int) (*models.UserList, error)
		Suggested(ctx context.Context, viewerID int, limit int) (*models.UserList, error)
	}
	Media interface {
		Create(context.Context, *sql.Tx, *models.Media) error
		Delete(context.Context, *sql.Tx, uuid.UUID) error
		GetByID(context.Context, uuid.UUID) (*models.Media, error)
		SaveFile(uuid.UUID, multipart.File) error
		DeleteFile(uuid.UUID) error
		LinkMediaToPost(ctx context.Context, tx *sql.Tx, pm models.PostMedia) error
		FetchPostMedia(ctx context.Context, posts []*models.FullPost) error
	}
	Posts interface {
		GetByID(ctx context.Context, id int) (*models.Post, error)
		GetFullPostByID(ctx context.Context, id int) (*models.FullPost, error)
		Create(ctx context.Context, tx *sql.Tx, post *models.Post) error
		CreateQuotedPost(ctx context.Context, tx *sql.Tx, post *models.Post) error
		Update(ctx context.Context, tx *sql.Tx, postID, authorID int, content string) (*models.Post, error)
		DeleteByID(ctx context.Context, id int) error
		DeleteCascade(ctx context.Context, tx *sql.Tx, id int) error
		Pin(ctx context.Context, tx *sql.Tx, postID, authorID int) error
		Unpin(ctx context.Context, tx *sql.Tx, postID, authorID int) error
		GetPinned(ctx context.Context, authorID int) (*models.Post, error)
		ListEdits(ctx context.Context, postID int) (*models.PostEditHistory, error)
		CreateEdit(ctx context.Context, tx *sql.Tx, postID int, contentBefore string) error
		GetParentChain(ctx context.Context, postID int, limit int, cursor string) (*models.PostChain, error)
		GetDescendants(ctx context.Context, postID int, limit int, cursor string) (*models.PostDescendants, error)
		GetHomeFeed(ctx context.Context, userID int, limit int, cursor string) (*models.PostFeed, error)
		GetUserFeed(ctx context.Context, userID int, includeReplies bool, limit int, cursor string) (*models.PostFeed, error)
		GetBookmarkedPostsFeed(ctx context.Context, userID int, categoryIDs []int, limit int, cursor string) (*models.PostFeed, error)
		GetLikedPostsFeed(ctx context.Context, userID int, limit int, cursor string) (*models.PostFeed, error)
		GetQuotesFeed(ctx context.Context, postID int, limit int, cursor string) (*models.PostFeed, error)
		Search(ctx context.Context, query string, limit int, cursor string) (*models.PostFeed, error)
		ListByHashtag(ctx context.Context, name string, limit int, cursor string) (*models.PostFeed, error)
		GetListFeed(ctx context.Context, listID int, limit int, cursor string) (*models.PostFeed, error)
	}
	PostEngagements interface {
		Like(ctx context.Context, tx *sql.Tx, postID, userID int) (bool, error)
		Unlike(ctx context.Context, tx *sql.Tx, postID, userID int) error
		Repost(ctx context.Context, tx *sql.Tx, postID, userID int) (bool, error)
		Unrepost(ctx context.Context, tx *sql.Tx, postID, userID int) error
		Bookmark(ctx context.Context, tx *sql.Tx, postID, userID int, categoryID *int) error
		Unbookmark(ctx context.Context, tx *sql.Tx, postID, userID int) error
		AddView(ctx context.Context, postID int, userID *int, ipAddress, userAgent string) error
		CreateBookmarkCategory(ctx context.Context, tx *sql.Tx, userID int, categoryName, color string) (*models.BookmarkCategory, error)
		ListBookmarkCategories(ctx context.Context, userID int) ([]models.BookmarkCategory, error)
		DeleteBookmarkCategory(ctx context.Context, tx *sql.Tx, userID, categoryID int) error
		GetEngagementForPosts(ctx context.Context, postIDs []int, viewerID int) (map[int]*models.PostEngagement, error)
		GetPostLikers(ctx context.Context, postID int, limit int, cursor string) (*models.UserList, error)
		GetPostReposters(ctx context.Context, postID int, limit int, cursor string) (*models.UserList, error)
	}
	UserRelationships interface {
		Create(context.Context, *sql.Tx, *models.UserRelationship) error
		GetByIDs(context.Context, int, int) (*models.UserRelationship, error)
		Update(context.Context, *sql.Tx, *models.UserRelationship) error
		Delete(context.Context, *sql.Tx, int, int) error
		GetFollowers(context.Context, int, int, string) (*models.UserFollowersResponse, error)
		GetFollowing(context.Context, int, int, string) (*models.UserFollowingResponse, error)
		GetRelationshipStatus(context.Context, int, int) (*models.RelationshipStatus, error)
		GetFollowerIDs(ctx context.Context, userID int) ([]int, error)
	}
	Notifications interface {
		Create(ctx context.Context, tx *sql.Tx, notification *models.Notification, recipientID, actorID int, notificationType string, postID *int) error
		List(ctx context.Context, recipientID, limit int, cursor string) (*models.NotificationFeed, error)
		UnreadCount(ctx context.Context, recipientID int) (int, error)
		MarkRead(ctx context.Context, recipientID, notificationID int) error
		MarkAllRead(ctx context.Context, recipientID int) error
	}
	Hashtags interface {
		SyncPost(ctx context.Context, tx *sql.Tx, postID int, content string) error
		Trends(ctx context.Context, limit int) ([]models.Trend, error)
	}
	Polls interface {
		Create(ctx context.Context, tx *sql.Tx, postID int, payload *models.CreatePollPayload) error
		GetForPost(ctx context.Context, postID, viewerID int) (*models.Poll, error)
		GetForPosts(ctx context.Context, postIDs []int, viewerID int) (map[int]*models.Poll, error)
		Vote(ctx context.Context, tx *sql.Tx, postID, optionID, userID int) error
	}
	Badges interface {
		ListCatalog(ctx context.Context) ([]models.Badge, error)
		CreateBadge(ctx context.Context, payload models.CreateBadgePayload) (*models.Badge, error)
		UpdateBadge(ctx context.Context, badgeID int, payload models.CreateBadgePayload) (*models.Badge, error)
		DeleteBadge(ctx context.Context, badgeID int) error
		GrantBadge(ctx context.Context, userID, badgeID, grantedBy int) error
		RevokeBadge(ctx context.Context, userID, badgeID int) error
		GetBadgesForUsers(ctx context.Context, ids []int) (map[int][]models.UserBadge, error)
	}
	Lists interface {
		Create(ctx context.Context, list *models.List) error
		GetByID(ctx context.Context, listID int) (*models.List, error)
		ListByOwner(ctx context.Context, ownerID int) ([]models.List, error)
		Delete(ctx context.Context, listID, ownerID int) error
		AddMember(ctx context.Context, listID, memberID int) error
		RemoveMember(ctx context.Context, listID, memberID int) error
		GetMembers(ctx context.Context, listID, limit int, cursor string) (*models.ListMembersResponse, error)
	}
	DMs interface {
		GetOrCreateConversation(ctx context.Context, participantA, participantB int) (*models.Conversation, error)
		GetConversation(ctx context.Context, conversationID, viewerID int) (*models.Conversation, error)
		ListConversations(ctx context.Context, viewerID int) (*models.ConversationFeed, error)
		AddMessage(ctx context.Context, conversationID, senderID int, body string) (*models.Message, error)
		ListMessages(ctx context.Context, conversationID, limit int, cursor string) (*models.MessageFeed, error)
		UnreadCount(ctx context.Context, viewerID int) (int, error)
		MarkRead(ctx context.Context, conversationID, readerID int) (int, error)
		IsParticipant(ctx context.Context, conversationID, userID int) (bool, error)
	}
}

func NewStore(db *sql.DB, logger *slog.Logger, mediaDir string) *Store {
	return &Store{
		DB:                db,
		Auth:              &authStore{db: db, logger: logger},
		Users:             &userStore{db: db, logger: logger},
		Media:             &mediaStore{db: db, mediaDir: mediaDir, logger: logger},
		Posts:             &postStore{db: db, logger: logger},
		PostEngagements:   &postEngagementStore{db: db, logger: logger},
		UserRelationships: &userRelationshipStore{db: db, logger: logger},
		Notifications:     &notificationStore{db: db, logger: logger},
		Hashtags:          &hashtagStore{db: db, logger: logger},
		Polls:             &pollStore{db: db, logger: logger},
		Badges:            &badgeStore{db: db, logger: logger},
		Lists:             &listStore{db: db, logger: logger},
		DMs:               &dmStore{db: db, logger: logger},
	}
}

func NewPostgresStorage(cfg config.DBConfig) (*sql.DB, error) {
	// Create the connection string
	dsn := fmt.Sprintf("postgres://%s:%s@%s/%s?sslmode=disable", cfg.DBUser, cfg.DBPassword, cfg.DBAddress, cfg.DBName)

	// Open the database connection
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, err
	}

	db.SetMaxOpenConns(cfg.DBMaxOpenConns)
	db.SetMaxIdleConns(cfg.DBMaxIdleConns)
	db.SetConnMaxIdleTime(cfg.DBMaxIdleTime)

	// What we are doing here is creating a new context `ctx` that will be
	// cancelled in 5 seconds, and is also tied to `context.Background()`.
	// If `context.Background()` were ever cancellable (it's not), and it were
	// cancelled, `ctx` would be cancelled too.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Test the connection
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, err
	}

	return db, nil
}
