package api

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	swagger "github.com/swaggo/http-swagger"

	"github.com/ba-reynolds/gophersocial/internal/cache"
	"github.com/ba-reynolds/gophersocial/internal/handlers"
	mid "github.com/ba-reynolds/gophersocial/internal/middleware"
	"github.com/ba-reynolds/gophersocial/internal/service"
)

// NewRouter creates a new chi router
func NewRouter(
	service *service.Service,
	logger *slog.Logger,
	rdb *cache.Client,
	rateLimitMaxRequests int,
	rateLimitWindow time.Duration,
	cookieSecure bool,
) http.Handler {
	router := chi.NewRouter()

	// Global middleware stack (replaces gin.Default())
	router.Use(middleware.RequestID)
	router.Use(middleware.Logger)
	router.Use(middleware.Recoverer)
	router.Use(mid.CorsMiddleware)

	// Swagger documentation (public access)
	router.Get("/swagger/*", swagger.Handler(
		swagger.URL("/swagger/doc.json"),
		swagger.DocExpansion("none"),
		swagger.DeepLinking(true),
		swagger.DomID("swagger-ui"),
	))

	authHandler := handlers.NewAuthHandler(service, logger, cookieSecure)
	mediaHandler := handlers.NewMediaHandler(service, logger)
	userHandler := handlers.NewUserHandler(service, logger)
	postHandler := handlers.NewPostHandler(service, logger, rdb)
	userRelationshipHandler := handlers.NewUserRelationshipHandler(service, logger)
	postEngagementHandler := handlers.NewPostEngagementHandler(service, logger, rdb)
	settingsHandler := handlers.NewSettingsHandler(service, logger)

	// API v1 routes
	router.Route("/api/v1", func(r chi.Router) {
		// Auth routes (public access - no auth required).
		// Rate limiting targets brute-force vectors (login/register) only;
		// refresh-token and logout are the recovery paths and must not be
		// locked out.
		r.Route("/auth", func(r chi.Router) {
			r.Group(func(limited chi.Router) {
				limited.Use(mid.RateLimitMiddleware(rdb, rateLimitMaxRequests, rateLimitWindow))
				limited.Post("/register", authHandler.Register)
				limited.Post("/login", authHandler.Login)
			})
			r.Post("/refresh-token", authHandler.RefreshToken)
			r.Post("/logout", authHandler.Logout)
		})

		// Media files are served publicly because <img> tags cannot send the
		// Authorization header; the UUIDs act as unguessable access tokens.
		r.Get("/media/{uuid}", mediaHandler.GetMediaByID)

		// Protected routes (require authentication)
		r.Group(func(protected chi.Router) {
			protected.Use(mid.AuthTokenMiddleware(service, logger))

			protected.Route("/users", func(r chi.Router) {
				r.Route("/me", func(r chi.Router) {
					r.Get("/", userHandler.GetMe)
					r.Patch("/", userHandler.UpdateUserProfile)
				})

				r.Get("/settings", settingsHandler.GetSettings)
				r.Patch("/settings", settingsHandler.UpdateSettings)

				r.Route("/{username}", func(r chi.Router) {
					r.Get("/", userHandler.GetUserProfileByUsername)
					r.Get("/posts", postHandler.GetUserFeed)
					r.Get("/followers", userRelationshipHandler.GetFollowers)
					r.Get("/following", userRelationshipHandler.GetFollowing)
					r.Post("/follow", userRelationshipHandler.FollowUser)
					r.Delete("/follow", userRelationshipHandler.UnfollowUser)
					r.Post("/block", userRelationshipHandler.BlockUser)
					r.Delete("/block", userRelationshipHandler.UnblockUser)
					// Change likes feed for user to use username instead of userID
					r.Get("/likes", postHandler.LikedPostsFeed)
				})
			})

			protected.Route("/posts", func(r chi.Router) {
				r.Post("/", postHandler.CreatePost)
				r.Get("/feed", postHandler.GetHomeFeed)
				r.Get("/bookmarks", postHandler.BookmarkedPostsFeed)

				// Apply middleware only to routes with {postID}
				r.Route("/{postID}", func(r chi.Router) {
					r.Use(mid.SetPostContextMiddleware(service, logger))
					r.Get("/", postHandler.GetPostByID)
					r.Delete("/", postHandler.DeletePostByID)
					r.Get("/quotes", postHandler.GetPostQuotesFeed)
					r.Get("/likers", postHandler.GetPostLikers)
					r.Get("/reposters", postHandler.GetPostReposters)

					// Post engagement endpoints
					r.Post("/like", postEngagementHandler.Like)
					r.Delete("/like", postEngagementHandler.Unlike)
					r.Post("/repost", postEngagementHandler.Repost)
					r.Delete("/repost", postEngagementHandler.Unrepost)
					r.Post("/quote", postEngagementHandler.Quote)
					r.Post("/bookmark", postEngagementHandler.Bookmark)
					r.Delete("/bookmark", postEngagementHandler.Unbookmark)
				})
			})

			protected.Route("/media", func(r chi.Router) {
				r.Post("/upload", mediaHandler.UploadMedia)
			})

			protected.Route("/bookmarks", func(r chi.Router) {
				r.Post("/category", postEngagementHandler.CreateBookmarkCategory)
				r.Get("/category", postEngagementHandler.ListBookmarkCategories)
				r.Delete("/category/{categoryID}", postEngagementHandler.DeleteBookmarkCategory)
			})
		})
	})

	return router
}
