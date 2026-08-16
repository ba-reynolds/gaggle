package handlers

import (
	"log/slog"
	"net/http"

	"github.com/ba-reynolds/vitrilium/internal/apperrors"
	"github.com/ba-reynolds/vitrilium/internal/middleware"
	"github.com/ba-reynolds/vitrilium/internal/models"
	"github.com/ba-reynolds/vitrilium/internal/service"
	"github.com/ba-reynolds/vitrilium/internal/util"
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

	if err := util.RespondWithJson(w, http.StatusOK, user.ToProfileResponse()); err != nil {
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
