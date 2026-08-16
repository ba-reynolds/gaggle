package middleware

import (
	"context"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/ba-reynolds/vitrilium/internal/apperrors"
	"github.com/ba-reynolds/vitrilium/internal/models"
	"github.com/ba-reynolds/vitrilium/internal/service"
	"github.com/ba-reynolds/vitrilium/internal/util"
	"github.com/go-chi/chi/v5"
)

type postContextKey string

const postContext postContextKey = "post"

// AuthTokenMiddleware requires authentication for all requests
func SetPostContextMiddleware(service *service.Service, logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			postIDString := chi.URLParam(r, "postID")
			postID, err := strconv.Atoi(postIDString)
			if err != nil {
				// Log parameter parsing errors - these are HTTP layer concerns
				logger.Warn("invalid post ID parameter in middleware",
					"postID", postIDString,
					"error", err,
					"path", r.URL.Path,
				)
				util.RespondWithAppError(w, apperrors.BadRequestError("invalid post ID", err))
				return
			}

			// Log the middleware request for debugging
			logger.Debug("post context middleware request",
				"postID", postID,
				"path", r.URL.Path,
			)

			ctx := r.Context()
			post, err := service.Posts.GetByID(ctx, postID)
			if err != nil {
				// Don't log service errors - they're already logged at appropriate layer
				// Just handle HTTP response mapping
				if appErr, ok := err.(*apperrors.AppError); ok {
					util.RespondWithAppError(w, appErr)
					return
				}
				util.RespondWithAppError(w, apperrors.InternalServerError(err))
				return
			}

			ctx = context.WithValue(ctx, postContext, post)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// GetPostFromContext retrieves the post from the request context
func GetPostFromContext(r *http.Request) *models.Post {
	post, _ := r.Context().Value(postContext).(*models.Post)
	return post
}
