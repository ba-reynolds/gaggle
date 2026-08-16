package store

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"mime/multipart"
	"time"

	"github.com/ba-reynolds/vitrilium/internal/models"
	"github.com/ba-reynolds/vitrilium/pkg/config"
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
		DeleteByID(ctx context.Context, id int) error
		GetParentChain(ctx context.Context, postID int, limit int, cursor string) (*models.PostChain, error)
		GetDescendants(ctx context.Context, postID int, limit int, cursor string) (*models.PostDescendants, error)
		GetHomeFeed(ctx context.Context, userID int, limit int, cursor string) (*models.PostFeed, error)
		GetUserFeed(ctx context.Context, userID int, includeReplies bool, limit int, cursor string) (*models.PostFeed, error)
		GetBookmarkedPostsFeed(ctx context.Context, userID int, categoryIDs []int, limit int, cursor string) (*models.PostFeed, error)
		GetLikedPostsFeed(ctx context.Context, userID int, limit int, cursor string) (*models.PostFeed, error)
		GetQuotesFeed(ctx context.Context, postID int, limit int, cursor string) (*models.PostFeed, error)
	}
	PostEngagements interface {
		Like(ctx context.Context, tx *sql.Tx, postID, userID int) error
		Unlike(ctx context.Context, tx *sql.Tx, postID, userID int) error
		Repost(ctx context.Context, tx *sql.Tx, postID, userID int) error
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
