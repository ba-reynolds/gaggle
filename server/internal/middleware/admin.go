package middleware

import (
	"net/http"

	"github.com/ba-reynolds/gaggle/internal/apperrors"
	"github.com/ba-reynolds/gaggle/internal/util"
)

// AdminOnlyMiddleware requires the authenticated user to be an admin.
// It must be mounted after AuthTokenMiddleware so the user is in context.
func AdminOnlyMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, err := GetAuthenticatedUserFromContext(r)
		if err != nil {
			util.RespondWithAppError(w, apperrors.UnauthorizedError("authentication required", err))
			return
		}
		if !user.IsAdmin {
			util.RespondWithAppError(w, apperrors.ForbiddenError("admin access required", nil))
			return
		}
		next.ServeHTTP(w, r)
	})
}
