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

// Referenced so swag can resolve the models package for the annotations below.
var _ models.UserSettings

// SettingsHandler handles HTTP requests for user settings
type SettingsHandler struct {
	service *service.Service
	logger  *slog.Logger
}

func NewSettingsHandler(service *service.Service, logger *slog.Logger) *SettingsHandler {
	return &SettingsHandler{service: service, logger: logger}
}

// GetSettings godoc
//
//	@Summary		Get user settings
//	@Description	Retrieves the current user's settings
//	@Tags			users
//	@Accept			json
//	@Produce		json
//	@Success		200	{object}	models.Envelope{data=models.UserSettings,error=nil}
//	@Failure		500	{object}	models.Envelope{data=nil,error=apperrors.AppError}
//	@Security		ApiKeyAuth
//	@Router			/users/settings [get]
func (h *SettingsHandler) GetSettings(w http.ResponseWriter, r *http.Request) {
	user, err := middleware.GetAuthenticatedUserFromContext(r)
	if err != nil {
		h.logger.Error("authentication middleware error", "error", err)
		util.RespondWithAppError(w, apperrors.InternalServerError(err))
		return
	}

	settings, err := h.service.Users.GetSettings(r.Context(), user.ID)
	if err != nil {
		if appErr, ok := err.(*apperrors.AppError); ok {
			util.RespondWithAppError(w, appErr)
			return
		}
		util.RespondWithAppError(w, apperrors.InternalServerError(err))
		return
	}

	if err := util.RespondWithJson(w, http.StatusOK, settings); err != nil {
		h.logger.Error("failed to write HTTP response", "error", err, "userID", user.ID, "status", http.StatusOK)
		util.RespondWithAppError(w, apperrors.InternalServerError(err))
		return
	}
}

// UpdateSettings godoc
//
//	@Summary		Update user settings
//	@Description	Updates the current user's settings with the provided fields
//	@Tags			users
//	@Accept			json
//	@Produce		json
//	@Param			payload	body		models.UserSettings	true	"Settings payload"
//	@Success		200		{object}	models.Envelope{data=models.UserSettings,error=nil}
//	@Failure		400		{object}	models.Envelope{data=nil,error=apperrors.AppError}
//	@Failure		500		{object}	models.Envelope{data=nil,error=apperrors.AppError}
//	@Security		ApiKeyAuth
//	@Router			/users/settings [patch]
func (h *SettingsHandler) UpdateSettings(w http.ResponseWriter, r *http.Request) {
	user, err := middleware.GetAuthenticatedUserFromContext(r)
	if err != nil {
		h.logger.Error("authentication middleware error", "error", err)
		util.RespondWithAppError(w, apperrors.InternalServerError(err))
		return
	}

	// Load current settings and overlay the patch so partial updates merge cleanly.
	current, err := h.service.Users.GetSettings(r.Context(), user.ID)
	if err != nil {
		if appErr, ok := err.(*apperrors.AppError); ok {
			util.RespondWithAppError(w, appErr)
			return
		}
		util.RespondWithAppError(w, apperrors.InternalServerError(err))
		return
	}

	if err := util.ReadJSON(r, &current); err != nil {
		h.logger.Warn("invalid JSON payload in request", "error", err, "userID", user.ID)
		util.RespondWithAppError(w, apperrors.PayloadValidationError(err))
		return
	}

	if err := h.service.Users.UpdateSettings(r.Context(), user.ID, current); err != nil {
		if appErr, ok := err.(*apperrors.AppError); ok {
			util.RespondWithAppError(w, appErr)
			return
		}
		util.RespondWithAppError(w, apperrors.InternalServerError(err))
		return
	}

	// Keep the query-time account-privacy flag in sync with the UI preference.
	// "private" and "friends" both mean followers-only in this app.
	isPrivate := current.Privacy.ProfileVisibility == "private" || current.Privacy.ProfileVisibility == "friends"
	if err := h.service.Users.SetPrivate(r.Context(), user.ID, isPrivate); err != nil {
		if appErr, ok := err.(*apperrors.AppError); ok {
			util.RespondWithAppError(w, appErr)
			return
		}
		util.RespondWithAppError(w, apperrors.InternalServerError(err))
		return
	}

	h.logger.Info("user settings updated successfully", "userID", user.ID)

	if err := util.RespondWithJson(w, http.StatusOK, current); err != nil {
		h.logger.Error("failed to write HTTP response", "error", err, "userID", user.ID, "status", http.StatusOK)
		util.RespondWithAppError(w, apperrors.InternalServerError(err))
		return
	}
}
