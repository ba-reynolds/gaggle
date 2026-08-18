package middleware

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/ba-reynolds/gophersocial/internal/apperrors"
	"github.com/ba-reynolds/gophersocial/internal/auth"
	"github.com/ba-reynolds/gophersocial/internal/models"
	"github.com/ba-reynolds/gophersocial/internal/service"
	"github.com/ba-reynolds/gophersocial/internal/util"
)

type authUserContextKey string

const authUserContext authUserContextKey = "auth_user"

// AuthTokenMiddleware requires authentication for all requests
func AuthTokenMiddleware(service *service.Service, logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				// Log authentication errors - these are HTTP layer concerns
				logger.Warn("authorization header missing",
					"path", r.URL.Path,
					"method", r.Method,
				)
				util.RespondWithAppError(w, apperrors.UnauthorizedError("authorization header missing", nil))
				return
			}

			parts := strings.Split(authHeader, " ")
			if len(parts) != 2 || parts[0] != "Bearer" {
				// Log authentication errors - these are HTTP layer concerns
				logger.Warn("authorization header malformed",
					"header", authHeader,
					"path", r.URL.Path,
					"method", r.Method,
				)
				util.RespondWithAppError(w, apperrors.UnauthorizedError("authorization header malformed", nil))
				return
			}

			accessTokenString := parts[1]
			parsedAccessToken, err := service.Auth.ValidateToken(accessTokenString, auth.AccessToken)
			if err != nil {
				// Log token validation errors - these are HTTP layer concerns
				logger.Warn("access token validation failed",
					"error", err,
					"path", r.URL.Path,
					"method", r.Method,
				)
				util.RespondWithAppError(w, apperrors.UnauthorizedError("token expired or invalid", err))
				return
			}

			// Get user ID from token
			userID, err := service.Auth.GetUserIDFromToken(parsedAccessToken)
			if err != nil {
				// Log token parsing errors - these are HTTP layer concerns
				logger.Error("failed to get user ID from token",
					"error", err,
					"path", r.URL.Path,
					"method", r.Method,
				)
				util.RespondWithAppError(w, apperrors.UnauthorizedError("malformed jwt", err))
				return
			}

			// Log the authentication request for debugging
			logger.Debug("authentication middleware request",
				"userID", userID,
				"path", r.URL.Path,
				"method", r.Method,
			)

			// Get user from database
			user, apperr := service.Users.GetByID(r.Context(), userID)
			if apperr != nil {
				// Don't log service errors - they're already logged at appropriate layer
				// Just handle HTTP response mapping
				if appErr, ok := apperr.(*apperrors.AppError); ok {
					util.RespondWithAppError(w, appErr)
					return
				}
				util.RespondWithAppError(w, apperrors.InternalServerError(apperr))
				return
			}

			// A soft-deleted (e.g. banned) user is rejected on the very next
			// request instead of staying authorized for the access-token
			// lifetime. GetByID already hits the DB, so this costs nothing.
			if user.SoftDeleted {
				logger.Warn("soft-deleted user rejected in auth middleware",
					"userID", user.ID,
					"path", r.URL.Path,
					"method", r.Method,
				)
				util.RespondWithAppError(w, apperrors.UnauthorizedError("user is not active", nil))
				return
			}

			// Set user in request context
			ctx := context.WithValue(r.Context(), authUserContext, user)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// GetAuthenticatedUserFromContext retrieves the authenticated user from the request context
func GetAuthenticatedUserFromContext(r *http.Request) (*models.User, error) {
	user, ok := r.Context().Value(authUserContext).(*models.User)
	if !ok {
		return nil, errors.New("user not found in context")
	}
	return user, nil
}

// IsAuthenticated checks if the request has an authenticated user
func IsAuthenticated(r *http.Request) bool {
	_, ok := r.Context().Value(authUserContext).(*models.User)
	return ok
}
