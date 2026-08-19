package handlers

import (
	"log/slog"
	"net/http"
	"strconv"

	"github.com/ba-reynolds/gaggle/internal/apperrors"
	"github.com/ba-reynolds/gaggle/internal/middleware"
	"github.com/ba-reynolds/gaggle/internal/service"
	"github.com/ba-reynolds/gaggle/internal/util"
)

type NotificationHandler struct {
	service *service.Service
	logger  *slog.Logger
}

func NewNotificationHandler(service *service.Service, logger *slog.Logger) *NotificationHandler {
	return &NotificationHandler{service: service, logger: logger}
}

func (h *NotificationHandler) List(w http.ResponseWriter, r *http.Request) {
	user, err := middleware.GetAuthenticatedUserFromContext(r)
	if err != nil {
		util.RespondWithAppError(w, apperrors.InternalServerError(err))
		return
	}
	limit, cursor, _ := util.ParsePaginationParams(r.URL.Query().Get("limit"), r.URL.Query().Get("cursor"), 20, 100)
	feed, err := h.service.Notifications.List(r.Context(), user.ID, limit, cursor)
	if err != nil {
		h.respondError(w, err)
		return
	}
	util.RespondWithJson(w, http.StatusOK, feed)
}

func (h *NotificationHandler) UnreadCount(w http.ResponseWriter, r *http.Request) {
	user, err := middleware.GetAuthenticatedUserFromContext(r)
	if err != nil {
		util.RespondWithAppError(w, apperrors.InternalServerError(err))
		return
	}
	count, err := h.service.Notifications.UnreadCount(r.Context(), user.ID)
	if err != nil {
		h.respondError(w, err)
		return
	}
	util.RespondWithJson(w, http.StatusOK, map[string]int{"count": count})
}

func (h *NotificationHandler) MarkAllRead(w http.ResponseWriter, r *http.Request) {
	user, err := middleware.GetAuthenticatedUserFromContext(r)
	if err != nil {
		util.RespondWithAppError(w, apperrors.InternalServerError(err))
		return
	}
	if err := h.service.Notifications.MarkAllRead(r.Context(), user.ID); err != nil {
		h.respondError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *NotificationHandler) MarkRead(w http.ResponseWriter, r *http.Request) {
	user, err := middleware.GetAuthenticatedUserFromContext(r)
	if err != nil {
		util.RespondWithAppError(w, apperrors.InternalServerError(err))
		return
	}
	notificationID, err := strconv.Atoi(r.PathValue("notificationID"))
	if err != nil {
		util.RespondWithAppError(w, apperrors.BadRequestError("invalid notification ID", err))
		return
	}
	if err := h.service.Notifications.MarkRead(r.Context(), user.ID, notificationID); err != nil {
		h.respondError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *NotificationHandler) respondError(w http.ResponseWriter, err error) {
	if appErr, ok := err.(*apperrors.AppError); ok {
		util.RespondWithAppError(w, appErr)
		return
	}
	util.RespondWithAppError(w, apperrors.InternalServerError(err))
}
