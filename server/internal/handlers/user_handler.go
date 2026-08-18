package handlers

import (
	"log/slog"
	"net/http"

	"github.com/ba-reynolds/gophersocial/internal/apperrors"
	"github.com/ba-reynolds/gophersocial/internal/middleware"
	"github.com/ba-reynolds/gophersocial/internal/models"
	"github.com/ba-reynolds/gophersocial/internal/service"
	"github.com/ba-reynolds/gophersocial/internal/util"
)

// UserHandler handles HTTP requests for user operations
type UserHandler struct {
	service *service.Service
	logger  *slog.Logger
}

func NewUserHandler(service *service.Service, logger *slog.Logger) *UserHandler {
	return &UserHandler{
		service: service,
		logger:  logger,
	}
}

func (h *UserHandler) respondError(w http.ResponseWriter, err error) {
	if appErr, ok := err.(*apperrors.AppError); ok {
		util.RespondWithAppError(w, appErr)
		return
	}
	util.RespondWithAppError(w, apperrors.InternalServerError(err))
}

// hydrateProfileRelationship fills the viewer-relative is_following /
// is_blocked / is_muted flags on the given user's profile response.
func (h *UserHandler) hydrateProfileRelationship(r *http.Request, viewerID, targetID int, resp *models.UserProfileResponse) error {
	if viewerID == targetID {
		return nil
	}
	status, err := h.service.UserRelationships.GetRelationshipStatus(r.Context(), viewerID, targetID)
	if err != nil {
		h.logger.Error("failed to get relationship status",
			"error", err,
			"viewerID", viewerID,
			"targetID", targetID,
		)
		return apperrors.InternalServerError(err)
	}
	resp.IsFollowing = status.IsFollowing
	resp.IsBlocked = status.IsBlocked
	resp.IsMuted = status.IsMuted
	return nil
}

// UpdateUserProfile godoc
//
//	@Summary		Update user profile
//	@Description	Updates a user's profile information
//	@Tags			users
//	@Accept			json
//	@Produce		json
//	@Param			id	path		int	true	"User ID"
//	@Param			payload	body		models.UpdateUserProfileRequest	true	"User profile information"
//	@Success		200	{object}	models.Envelope{data=models.UserWithProfile,error=nil}
//	@Failure		400	{object}	models.Envelope{data=nil,error=apperrors.AppError}
//	@Failure		404	{object}	models.Envelope{data=nil,error=apperrors.AppError}
//	@Failure		500	{object}	models.Envelope{data=nil,error=apperrors.AppError}
//	@Security		ApiKeyAuth
//	@Router			/users/me [patch]
func (h *UserHandler) UpdateUserProfile(w http.ResponseWriter, r *http.Request) {
	// Get user from context
	authUser, err := middleware.GetAuthenticatedUserFromContext(r)
	if err != nil {
		// Log authentication/middleware errors - these are HTTP layer concerns
		h.logger.Error("authentication middleware error",
			"error", err,
			"path", r.URL.Path,
			"method", r.Method,
		)
		util.RespondWithAppError(w, apperrors.InternalServerError(err))
		return
	}

	// Log the update request for debugging
	h.logger.Debug("user profile update request", "userID", authUser.ID)

	var payload models.UpdateUserProfileRequest
	if err := util.ReadJSON(r, &payload); err != nil {
		// Log JSON parsing errors - these are HTTP layer concerns
		h.logger.Warn("invalid JSON payload in request",
			"error", err,
			"userID", authUser.ID,
			"content_type", r.Header.Get("Content-Type"),
		)
		util.RespondWithAppError(w, apperrors.PayloadValidationError(err))
		return
	}

	if err := util.Validate.Struct(payload); err != nil {
		// Log validation errors - these are HTTP layer concerns
		h.logger.Warn("payload validation failed",
			"error", err,
			"userID", authUser.ID,
		)
		util.RespondWithAppError(w, apperrors.PayloadValidationError(err))
		return
	}

	// Create UserWithProfile from the authenticated user
	user := &models.UserWithProfile{
		User:    *authUser,
		Profile: models.UserProfile(payload),
	}

	if err := h.service.Users.UpdateUserProfile(r.Context(), user); err != nil {
		// Don't log service errors - they're already logged at appropriate layer
		// Just handle HTTP response mapping
		if appErr, ok := err.(*apperrors.AppError); ok {
			util.RespondWithAppError(w, appErr)
			return
		}
		util.RespondWithAppError(w, apperrors.InternalServerError(err))
		return
	}

	// Log successful operations
	h.logger.Info("user profile updated successfully", "userID", user.ID)

	if err := h.service.Badges.HydrateUserWithProfile(r.Context(), user); err != nil {
		h.logger.Error("failed to hydrate badges", "error", err, "userID", user.ID)
		util.RespondWithAppError(w, apperrors.InternalServerError(err))
		return
	}
	if err := util.RespondWithJson(w, http.StatusOK, user.ToProfileResponse()); err != nil {
		// Log HTTP response errors - these are HTTP layer concerns
		h.logger.Error("failed to write HTTP response",
			"error", err,
			"userID", user.ID,
			"status", http.StatusOK,
		)
		util.RespondWithAppError(w, apperrors.InternalServerError(err))
		return
	}
}

// GetMe godoc
//
//	@Summary		Get current user
//	@Description	Retrieves the current authenticated user's information
//	@Tags			users
//	@Accept			json
//	@Produce		json
//	@Success		200	{object}	models.Envelope{data=models.User,error=nil}
//	@Failure		401	{object}	models.Envelope{data=nil,error=apperrors.AppError}
//	@Failure		500	{object}	models.Envelope{data=nil,error=apperrors.AppError}
//	@Security		ApiKeyAuth
//	@Router			/users/me [get]
func (h *UserHandler) GetMe(w http.ResponseWriter, r *http.Request) {
	user, err := middleware.GetAuthenticatedUserFromContext(r)
	if err != nil {
		// Log authentication/middleware errors - these are HTTP layer concerns
		h.logger.Error("authentication middleware error",
			"error", err,
			"path", r.URL.Path,
			"method", r.Method,
		)
		util.RespondWithAppError(w, apperrors.InternalServerError(err))
		return
	}

	// Log the request for debugging
	h.logger.Debug("get current user request", "userID", user.ID, "username", user.Username)

	userWithProfile, err := h.service.Users.GetUserProfileByUsername(r.Context(), user.Username)
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

	if err := h.service.Badges.HydrateUserWithProfile(r.Context(), userWithProfile); err != nil {
		h.logger.Error("failed to hydrate badges", "error", err, "userID", userWithProfile.ID)
		util.RespondWithAppError(w, apperrors.InternalServerError(err))
		return
	}
	if err := util.RespondWithJson(w, http.StatusOK, userWithProfile.ToProfileResponse()); err != nil {
		// Log HTTP response errors - these are HTTP layer concerns
		h.logger.Error("failed to write HTTP response",
			"error", err,
			"userID", user.ID,
			"status", http.StatusOK,
		)
		util.RespondWithAppError(w, apperrors.InternalServerError(err))
		return
	}
}

// GetUserProfileByUsername godoc
//
//	@Summary		Get user profile by username
//	@Description	Retrieves a user's profile information by username
//	@Tags			users
//	@Accept			json
//	@Produce		json
//	@Param			username	path		string	true	"Username"
//	@Success		200	{object}	models.Envelope{data=models.UserWithProfile,error=nil}
//	@Failure		404	{object}	models.Envelope{data=nil,error=apperrors.AppError}
//	@Failure		500	{object}	models.Envelope{data=nil,error=apperrors.AppError}
//	@Security		ApiKeyAuth
//	@Router			/users/{username} [get]
func (h *UserHandler) GetUserProfileByUsername(w http.ResponseWriter, r *http.Request) {
	username := r.PathValue("username")

	// Log the request for debugging
	h.logger.Debug("get user profile request", "username", username)

	viewer, viewerErr := middleware.GetAuthenticatedUserFromContext(r)
	if viewerErr != nil {
		h.logger.Error("authentication middleware error",
			"error", viewerErr,
			"path", r.URL.Path,
			"method", r.Method,
		)
		util.RespondWithAppError(w, apperrors.InternalServerError(viewerErr))
		return
	}

	user, err := h.service.Users.GetUserProfileByUsername(r.Context(), username)
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

	if err := h.service.Badges.HydrateUserWithProfile(r.Context(), user); err != nil {
		h.logger.Error("failed to hydrate badges", "error", err, "username", username)
		util.RespondWithAppError(w, apperrors.InternalServerError(err))
		return
	}

	resp := user.ToProfileResponse()
	if err := h.hydrateProfileRelationship(r, viewer.ID, user.ID, &resp); err != nil {
		if appErr, ok := err.(*apperrors.AppError); ok {
			util.RespondWithAppError(w, appErr)
			return
		}
		util.RespondWithAppError(w, apperrors.InternalServerError(err))
		return
	}
	if err := util.RespondWithJson(w, http.StatusOK, resp); err != nil {
		// Log HTTP response errors - these are HTTP layer concerns
		h.logger.Error("failed to write HTTP response",
			"error", err,
			"username", username,
			"userID", user.ID,
			"status", http.StatusOK,
		)
		util.RespondWithAppError(w, apperrors.InternalServerError(err))
		return
	}
}

// GetSuggestedUsers godoc
//
//	@Summary		Get suggested users
//	@Description	Returns accounts the authenticated user might want to follow, ordered by follower count.
//	@Tags			users
//	@Produce		json
//	@Param			limit	query		int	false	"Maximum number of users to retrieve"
//	@Success		200		{object}	models.Envelope{data=models.UserList,error=nil}
//	@Failure		500		{object}	models.Envelope{data=nil,error=apperrors.AppError}
//	@Security		ApiKeyAuth
//	@Router			/users/suggested [get]
func (h *UserHandler) GetSuggestedUsers(w http.ResponseWriter, r *http.Request) {
	user, err := middleware.GetAuthenticatedUserFromContext(r)
	if err != nil {
		h.logger.Error("authentication middleware error", "error", err)
		util.RespondWithAppError(w, apperrors.InternalServerError(err))
		return
	}
	limit, _, _ := util.ParsePaginationParams(r.URL.Query().Get("limit"), "", 5, 20)
	users, err := h.service.Users.Suggested(r.Context(), user.ID, limit)
	if err != nil {
		h.respondError(w, err)
		return
	}
	if err := h.service.Badges.HydrateProfiles(r.Context(), users.Items); err != nil {
		h.logger.Error("failed to hydrate badges", "error", err)
		h.respondError(w, err)
		return
	}
	util.RespondWithJson(w, http.StatusOK, users)
}
