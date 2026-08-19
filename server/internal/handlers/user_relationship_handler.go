package handlers

import (
	"log/slog"
	"net/http"
	"strconv"

	"github.com/ba-reynolds/gaggle/internal/apperrors"
	"github.com/ba-reynolds/gaggle/internal/middleware"
	"github.com/ba-reynolds/gaggle/internal/models"
	"github.com/ba-reynolds/gaggle/internal/service"
	"github.com/ba-reynolds/gaggle/internal/util"
)

// UserRelationshipHandler handles HTTP requests for user relationship operations
type UserRelationshipHandler struct {
	service *service.Service
	logger  *slog.Logger
}

func NewUserRelationshipHandler(service *service.Service, logger *slog.Logger) *UserRelationshipHandler {
	return &UserRelationshipHandler{
		service: service,
		logger:  logger,
	}
}

// FollowUser godoc
//
//	@Summary		Follow a user
//	@Description	Follows a user by creating a follow relationship
//	@Tags			relationships
//	@Accept			json
//	@Produce		json
//	@Param			username	path		string	true	"Username of the user to follow"
//	@Success		200	{object}	models.Envelope{data=models.UserRelationshipResponse,error=nil}
//	@Failure		400	{object}	models.Envelope{data=nil,error=apperrors.AppError}
//	@Failure		404	{object}	models.Envelope{data=nil,error=apperrors.AppError}
//	@Failure		500	{object}	models.Envelope{data=nil,error=apperrors.AppError}
//	@Security		ApiKeyAuth
//	@Router			/users/{username}/follow [post]
func (h *UserRelationshipHandler) FollowUser(w http.ResponseWriter, r *http.Request) {
	// Get authenticated user from context
	authUser, err := middleware.GetAuthenticatedUserFromContext(r)
	if err != nil {
		h.logger.Error("authentication middleware error",
			"error", err,
			"path", r.URL.Path,
			"method", r.Method,
		)
		util.RespondWithAppError(w, apperrors.InternalServerError(err))
		return
	}

	username := r.PathValue("username")
	if username == "" {
		util.RespondWithAppError(w, apperrors.BadRequestError("username is required", nil))
		return
	}

	// Get target user by username
	targetUser, err := h.service.Users.GetByUsername(r.Context(), username)
	if err != nil {
		if appErr, ok := err.(*apperrors.AppError); ok {
			util.RespondWithAppError(w, appErr)
			return
		}
		util.RespondWithAppError(w, apperrors.InternalServerError(err))
		return
	}

	// Only notify on a transition into the following state. Repeating the
	// follow request is idempotent and must not spam the recipient.
	status, err := h.service.UserRelationships.GetRelationshipStatus(r.Context(), authUser.ID, targetUser.ID)
	if err != nil {
		if appErr, ok := err.(*apperrors.AppError); ok {
			util.RespondWithAppError(w, appErr)
			return
		}
		util.RespondWithAppError(w, apperrors.InternalServerError(err))
		return
	}
	wasFollowing := status.IsFollowing

	// Create follow relationship
	_, err = h.service.UserRelationships.CreateRelationship(r.Context(), authUser.ID, targetUser.ID, "follow")
	if err != nil {
		if appErr, ok := err.(*apperrors.AppError); ok {
			util.RespondWithAppError(w, appErr)
			return
		}
		util.RespondWithAppError(w, apperrors.InternalServerError(err))
		return
	}
	if !wasFollowing {
		if err := h.service.Notifications.Create(r.Context(), authUser.ID, targetUser.ID, "follow", nil); err != nil {
			h.logger.Warn("failed to create follow notification", "actorID", authUser.ID, "recipientID", targetUser.ID, "error", err)
		}
	}

	// Create response
	response := models.UserRelationshipResponse{
		Success: true,
	}

	h.logger.Info("user followed successfully",
		"userID", authUser.ID,
		"target_userID", targetUser.ID,
	)

	if err := util.RespondWithJson(w, http.StatusOK, response); err != nil {
		h.logger.Error("failed to write HTTP response",
			"error", err,
			"userID", authUser.ID,
			"status", http.StatusOK,
		)
		util.RespondWithAppError(w, apperrors.InternalServerError(err))
		return
	}
}

// UnfollowUser godoc
//
//	@Summary		Unfollow a user
//	@Description	Unfollows a user by deleting the follow relationship
//	@Tags			relationships
//	@Accept			json
//	@Produce		json
//	@Param			username	path		string	true	"Username of the user to unfollow"
//	@Success		204	{object}	nil
//	@Failure		404	{object}	models.Envelope{data=nil,error=apperrors.AppError}
//	@Failure		500	{object}	models.Envelope{data=nil,error=apperrors.AppError}
//	@Security		ApiKeyAuth
//	@Router			/users/{username}/follow [delete]
func (h *UserRelationshipHandler) UnfollowUser(w http.ResponseWriter, r *http.Request) {
	// Get authenticated user from context
	authUser, err := middleware.GetAuthenticatedUserFromContext(r)
	if err != nil {
		h.logger.Error("authentication middleware error",
			"error", err,
			"path", r.URL.Path,
			"method", r.Method,
		)
		util.RespondWithAppError(w, apperrors.InternalServerError(err))
		return
	}

	username := r.PathValue("username")
	if username == "" {
		util.RespondWithAppError(w, apperrors.BadRequestError("username is required", nil))
		return
	}

	// Get target user by username
	targetUser, err := h.service.Users.GetByUsername(r.Context(), username)
	if err != nil {
		if appErr, ok := err.(*apperrors.AppError); ok {
			util.RespondWithAppError(w, appErr)
			return
		}
		util.RespondWithAppError(w, apperrors.InternalServerError(err))
		return
	}

	// Delete follow relationship
	if err := h.service.UserRelationships.DeleteRelationship(r.Context(), authUser.ID, targetUser.ID, "follow"); err != nil {
		if appErr, ok := err.(*apperrors.AppError); ok {
			util.RespondWithAppError(w, appErr)
			return
		}
		util.RespondWithAppError(w, apperrors.InternalServerError(err))
		return
	}

	h.logger.Info("user unfollowed successfully",
		"userID", authUser.ID,
		"target_userID", targetUser.ID,
	)

	w.WriteHeader(http.StatusNoContent)
}

// BlockUser godoc
//
//	@Summary		Block a user
//	@Description	Blocks a user by creating a block relationship
//	@Tags			relationships
//	@Accept			json
//	@Produce		json
//	@Param			username	path		string	true	"Username of the user to block"
//	@Success		200	{object}	models.Envelope{data=models.UserRelationshipResponse,error=nil}
//	@Failure		400	{object}	models.Envelope{data=nil,error=apperrors.AppError}
//	@Failure		404	{object}	models.Envelope{data=nil,error=apperrors.AppError}
//	@Failure		500	{object}	models.Envelope{data=nil,error=apperrors.AppError}
//	@Security		ApiKeyAuth
//	@Router			/users/{username}/block [post]
func (h *UserRelationshipHandler) BlockUser(w http.ResponseWriter, r *http.Request) {
	// Get authenticated user from context
	authUser, err := middleware.GetAuthenticatedUserFromContext(r)
	if err != nil {
		h.logger.Error("authentication middleware error",
			"error", err,
			"path", r.URL.Path,
			"method", r.Method,
		)
		util.RespondWithAppError(w, apperrors.InternalServerError(err))
		return
	}

	username := r.PathValue("username")
	if username == "" {
		util.RespondWithAppError(w, apperrors.BadRequestError("username is required", nil))
		return
	}

	// Get target user by username
	targetUser, err := h.service.Users.GetByUsername(r.Context(), username)
	if err != nil {
		if appErr, ok := err.(*apperrors.AppError); ok {
			util.RespondWithAppError(w, appErr)
			return
		}
		util.RespondWithAppError(w, apperrors.InternalServerError(err))
		return
	}

	// Create block relationship
	_, err = h.service.UserRelationships.CreateRelationship(r.Context(), authUser.ID, targetUser.ID, "block")
	if err != nil {
		if appErr, ok := err.(*apperrors.AppError); ok {
			util.RespondWithAppError(w, appErr)
			return
		}
		util.RespondWithAppError(w, apperrors.InternalServerError(err))
		return
	}

	// Create response
	response := models.UserRelationshipResponse{
		Success: true,
	}

	h.logger.Info("user blocked successfully",
		"userID", authUser.ID,
		"target_userID", targetUser.ID,
	)

	if err := util.RespondWithJson(w, http.StatusOK, response); err != nil {
		h.logger.Error("failed to write HTTP response",
			"error", err,
			"userID", authUser.ID,
			"status", http.StatusOK,
		)
		util.RespondWithAppError(w, apperrors.InternalServerError(err))
		return
	}
}

// UnblockUser godoc
//
//	@Summary		Unblock a user
//	@Description	Unblocks a user by deleting the block relationship
//	@Tags			relationships
//	@Accept			json
//	@Produce		json
//	@Param			username	path		string	true	"Username of the user to unblock"
//	@Success		204	{object}	nil
//	@Failure		404	{object}	models.Envelope{data=nil,error=apperrors.AppError}
//	@Failure		500	{object}	models.Envelope{data=nil,error=apperrors.AppError}
//	@Security		ApiKeyAuth
//	@Router			/users/{username}/block [delete]
func (h *UserRelationshipHandler) UnblockUser(w http.ResponseWriter, r *http.Request) {
	// Get authenticated user from context
	authUser, err := middleware.GetAuthenticatedUserFromContext(r)
	if err != nil {
		h.logger.Error("authentication middleware error",
			"error", err,
			"path", r.URL.Path,
			"method", r.Method,
		)
		util.RespondWithAppError(w, apperrors.InternalServerError(err))
		return
	}

	username := r.PathValue("username")
	if username == "" {
		util.RespondWithAppError(w, apperrors.BadRequestError("username is required", nil))
		return
	}

	// Get target user by username
	targetUser, err := h.service.Users.GetByUsername(r.Context(), username)
	if err != nil {
		if appErr, ok := err.(*apperrors.AppError); ok {
			util.RespondWithAppError(w, appErr)
			return
		}
		util.RespondWithAppError(w, apperrors.InternalServerError(err))
		return
	}

	// Delete block relationship
	if err := h.service.UserRelationships.DeleteRelationship(r.Context(), authUser.ID, targetUser.ID, "block"); err != nil {
		if appErr, ok := err.(*apperrors.AppError); ok {
			util.RespondWithAppError(w, appErr)
			return
		}
		util.RespondWithAppError(w, apperrors.InternalServerError(err))
		return
	}

	h.logger.Info("user unblocked successfully",
		"userID", authUser.ID,
		"target_userID", targetUser.ID,
	)

	w.WriteHeader(http.StatusNoContent)
}

// MuteUser godoc
//
//	@Summary		Mute a user
//	@Description	Mutes a user so their notifications are silenced
//	@Tags			relationships
//	@Accept			json
//	@Produce		json
//	@Param			username	path		string	true	"Username of the user to mute"
//	@Success		200	{object}	models.Envelope{data=models.UserRelationshipResponse,error=nil}
//	@Failure		400	{object}	models.Envelope{data=nil,error=apperrors.AppError}
//	@Failure		404	{object}	models.Envelope{data=nil,error=apperrors.AppError}
//	@Failure		500	{object}	models.Envelope{data=nil,error=apperrors.AppError}
//	@Security		ApiKeyAuth
//	@Router			/users/{username}/mute [post]
func (h *UserRelationshipHandler) MuteUser(w http.ResponseWriter, r *http.Request) {
	authUser, err := middleware.GetAuthenticatedUserFromContext(r)
	if err != nil {
		h.logger.Error("authentication middleware error",
			"error", err,
			"path", r.URL.Path,
			"method", r.Method,
		)
		util.RespondWithAppError(w, apperrors.InternalServerError(err))
		return
	}

	username := r.PathValue("username")
	if username == "" {
		util.RespondWithAppError(w, apperrors.BadRequestError("username is required", nil))
		return
	}

	targetUser, err := h.service.Users.GetByUsername(r.Context(), username)
	if err != nil {
		if appErr, ok := err.(*apperrors.AppError); ok {
			util.RespondWithAppError(w, appErr)
			return
		}
		util.RespondWithAppError(w, apperrors.InternalServerError(err))
		return
	}

	_, err = h.service.UserRelationships.CreateRelationship(r.Context(), authUser.ID, targetUser.ID, "mute")
	if err != nil {
		if appErr, ok := err.(*apperrors.AppError); ok {
			util.RespondWithAppError(w, appErr)
			return
		}
		util.RespondWithAppError(w, apperrors.InternalServerError(err))
		return
	}

	h.logger.Info("user muted successfully",
		"userID", authUser.ID,
		"target_userID", targetUser.ID,
	)

	if err := util.RespondWithJson(w, http.StatusOK, models.UserRelationshipResponse{Success: true}); err != nil {
		h.logger.Error("failed to write HTTP response",
			"error", err,
			"userID", authUser.ID,
			"status", http.StatusOK,
		)
		util.RespondWithAppError(w, apperrors.InternalServerError(err))
		return
	}
}

// UnmuteUser godoc
//
//	@Summary		Unmute a user
//	@Description	Unmutes a previously muted user
//	@Tags			relationships
//	@Accept			json
//	@Produce		json
//	@Param			username	path		string	true	"Username of the user to unmute"
//	@Success		204	{object}	nil
//	@Failure		404	{object}	models.Envelope{data=nil,error=apperrors.AppError}
//	@Failure		500	{object}	models.Envelope{data=nil,error=apperrors.AppError}
//	@Security		ApiKeyAuth
//	@Router			/users/{username}/mute [delete]
func (h *UserRelationshipHandler) UnmuteUser(w http.ResponseWriter, r *http.Request) {
	authUser, err := middleware.GetAuthenticatedUserFromContext(r)
	if err != nil {
		h.logger.Error("authentication middleware error",
			"error", err,
			"path", r.URL.Path,
			"method", r.Method,
		)
		util.RespondWithAppError(w, apperrors.InternalServerError(err))
		return
	}

	username := r.PathValue("username")
	if username == "" {
		util.RespondWithAppError(w, apperrors.BadRequestError("username is required", nil))
		return
	}

	targetUser, err := h.service.Users.GetByUsername(r.Context(), username)
	if err != nil {
		if appErr, ok := err.(*apperrors.AppError); ok {
			util.RespondWithAppError(w, appErr)
			return
		}
		util.RespondWithAppError(w, apperrors.InternalServerError(err))
		return
	}

	if err := h.service.UserRelationships.DeleteRelationship(r.Context(), authUser.ID, targetUser.ID, "mute"); err != nil {
		if appErr, ok := err.(*apperrors.AppError); ok {
			util.RespondWithAppError(w, appErr)
			return
		}
		util.RespondWithAppError(w, apperrors.InternalServerError(err))
		return
	}

	h.logger.Info("user unmuted successfully",
		"userID", authUser.ID,
		"target_userID", targetUser.ID,
	)

	w.WriteHeader(http.StatusNoContent)
}

// GetFollowers godoc
//
//	@Summary		Get user followers
//	@Description	Retrieves a paginated list of followers for a user
//	@Tags			relationships
//	@Accept			json
//	@Produce		json
//	@Param			username	path		string	true	"Username"
//	@Param			limit	query		int	false	"Number of followers to return (max 100, default 20)"
//	@Param			cursor	query		string	false	"Cursor for pagination"
//	@Success		200	{object}	models.Envelope{data=models.UserFollowersResponse,error=nil}
//	@Failure		404	{object}	models.Envelope{data=nil,error=apperrors.AppError}
//	@Failure		500	{object}	models.Envelope{data=nil,error=apperrors.AppError}
//	@Security		ApiKeyAuth
//	@Router			/users/{username}/followers [get]
func (h *UserRelationshipHandler) GetFollowers(w http.ResponseWriter, r *http.Request) {
	username := r.PathValue("username")
	if username == "" {
		util.RespondWithAppError(w, apperrors.BadRequestError("username is required", nil))
		return
	}

	viewer, err := middleware.GetAuthenticatedUserFromContext(r)
	if err != nil {
		h.logger.Error("authentication middleware error",
			"error", err,
			"path", r.URL.Path,
			"method", r.Method,
		)
		util.RespondWithAppError(w, apperrors.InternalServerError(err))
		return
	}

	// Get user by username
	user, err := h.service.Users.GetByUsername(r.Context(), username)
	if err != nil {
		if appErr, ok := err.(*apperrors.AppError); ok {
			util.RespondWithAppError(w, appErr)
			return
		}
		util.RespondWithAppError(w, apperrors.InternalServerError(err))
		return
	}

	// Parse pagination parameters
	limitStr := r.URL.Query().Get("limit")
	cursor := r.URL.Query().Get("cursor")

	limit := 20 // default
	if limitStr != "" {
		if parsedLimit, err := strconv.Atoi(limitStr); err == nil && parsedLimit > 0 && parsedLimit <= 100 {
			limit = parsedLimit
		}
	}

	// Get followers
	followers, err := h.service.UserRelationships.GetFollowers(r.Context(), user.ID, limit, cursor)
	if err != nil {
		if appErr, ok := err.(*apperrors.AppError); ok {
			util.RespondWithAppError(w, appErr)
			return
		}
		util.RespondWithAppError(w, apperrors.InternalServerError(err))
		return
	}

	if err := h.service.Badges.HydrateProfiles(r.Context(), followers.Items); err != nil {
		h.logger.Error("failed to hydrate badges", "error", err, "username", username)
		util.RespondWithAppError(w, apperrors.InternalServerError(err))
		return
	}

	if err := h.hydrateRelationshipStatuses(r, viewer.ID, followers.Items); err != nil {
		h.logger.Error("failed to hydrate relationship statuses", "error", err, "username", username)
		util.RespondWithAppError(w, apperrors.InternalServerError(err))
		return
	}

	if err := util.RespondWithJson(w, http.StatusOK, followers); err != nil {
		h.logger.Error("failed to write HTTP response",
			"error", err,
			"username", username,
			"status", http.StatusOK,
		)
		util.RespondWithAppError(w, apperrors.InternalServerError(err))
		return
	}
}

// GetFollowing godoc
//
//	@Summary		Get user following
//	@Description	Retrieves a paginated list of users that a user is following
//	@Tags			relationships
//	@Accept			json
//	@Produce		json
//	@Param			username	path		string	true	"Username"
//	@Param			limit	query		int	false	"Number of following to return (max 100, default 20)"
//	@Param			cursor	query		string	false	"Cursor for pagination"
//	@Success		200	{object}	models.Envelope{data=models.UserFollowingResponse,error=nil}
//	@Failure		404	{object}	models.Envelope{data=nil,error=apperrors.AppError}
//	@Failure		500	{object}	models.Envelope{data=nil,error=apperrors.AppError}
//	@Security		ApiKeyAuth
//	@Router			/users/{username}/following [get]
func (h *UserRelationshipHandler) GetFollowing(w http.ResponseWriter, r *http.Request) {
	username := r.PathValue("username")
	if username == "" {
		util.RespondWithAppError(w, apperrors.BadRequestError("username is required", nil))
		return
	}

	viewer, err := middleware.GetAuthenticatedUserFromContext(r)
	if err != nil {
		h.logger.Error("authentication middleware error",
			"error", err,
			"path", r.URL.Path,
			"method", r.Method,
		)
		util.RespondWithAppError(w, apperrors.InternalServerError(err))
		return
	}

	// Get user by username
	user, err := h.service.Users.GetByUsername(r.Context(), username)
	if err != nil {
		if appErr, ok := err.(*apperrors.AppError); ok {
			util.RespondWithAppError(w, appErr)
			return
		}
		util.RespondWithAppError(w, apperrors.InternalServerError(err))
		return
	}

	// Parse pagination parameters
	limitStr := r.URL.Query().Get("limit")
	cursor := r.URL.Query().Get("cursor")

	limit := 20 // default
	if limitStr != "" {
		if parsedLimit, err := strconv.Atoi(limitStr); err == nil && parsedLimit > 0 && parsedLimit <= 100 {
			limit = parsedLimit
		}
	}

	// Get following
	following, err := h.service.UserRelationships.GetFollowing(r.Context(), user.ID, limit, cursor)
	if err != nil {
		if appErr, ok := err.(*apperrors.AppError); ok {
			util.RespondWithAppError(w, appErr)
			return
		}
		util.RespondWithAppError(w, apperrors.InternalServerError(err))
		return
	}

	if err := h.service.Badges.HydrateProfiles(r.Context(), following.Items); err != nil {
		h.logger.Error("failed to hydrate badges", "error", err, "username", username)
		util.RespondWithAppError(w, apperrors.InternalServerError(err))
		return
	}

	if err := h.hydrateRelationshipStatuses(r, viewer.ID, following.Items); err != nil {
		h.logger.Error("failed to hydrate relationship statuses", "error", err, "username", username)
		util.RespondWithAppError(w, apperrors.InternalServerError(err))
		return
	}

	if err := util.RespondWithJson(w, http.StatusOK, following); err != nil {
		h.logger.Error("failed to write HTTP response",
			"error", err,
			"username", username,
			"status", http.StatusOK,
		)
		util.RespondWithAppError(w, apperrors.InternalServerError(err))
		return
	}
}

// hydrateRelationshipStatuses fills the viewer-relative is_following /
// is_blocked / is_muted flags on a batch of flat profile responses.
func (h *UserRelationshipHandler) hydrateRelationshipStatuses(r *http.Request, viewerID int, items []models.UserProfileResponse) error {
	ids := make([]int, 0, len(items))
	for _, it := range items {
		if it.UserID != 0 {
			ids = append(ids, it.UserID)
		}
	}
	if len(ids) == 0 {
		return nil
	}
	statuses, err := h.service.UserRelationships.GetRelationshipStatuses(r.Context(), viewerID, ids)
	if err != nil {
		return err
	}
	for i := range items {
		if s, ok := statuses[items[i].UserID]; ok {
			items[i].IsFollowing = s.IsFollowing
			items[i].IsBlocked = s.IsBlocked
			items[i].IsMuted = s.IsMuted
		}
	}
	return nil
}
