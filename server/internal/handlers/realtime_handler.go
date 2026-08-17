package handlers

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/ba-reynolds/gophersocial/internal/apperrors"
	"github.com/ba-reynolds/gophersocial/internal/realtime"
	"github.com/ba-reynolds/gophersocial/internal/service"
	"github.com/ba-reynolds/gophersocial/internal/util"
)

type RealtimeHandler struct {
	service *service.Service
	logger  *slog.Logger
}

func NewRealtimeHandler(service *service.Service, logger *slog.Logger) *RealtimeHandler {
	return &RealtimeHandler{service: service, logger: logger}
}

func (h *RealtimeHandler) Stream(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("refresh_token")
	if err != nil {
		util.RespondWithAppError(w, apperrors.UnauthorizedError("authentication required", err))
		return
	}
	userID, err := h.service.Auth.GetUserIDFromRefreshToken(r.Context(), cookie.Value)
	if err != nil {
		if appErr, ok := err.(*apperrors.AppError); ok {
			util.RespondWithAppError(w, appErr)
			return
		}
		util.RespondWithAppError(w, apperrors.UnauthorizedError("authentication required", err))
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		util.RespondWithAppError(w, apperrors.InternalServerError(fmt.Errorf("streaming is not supported")))
		return
	}
	controller := http.NewResponseController(w)
	_ = controller.SetWriteDeadline(time.Time{})
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	channel, cancel := h.service.Realtime.Subscribe(userID)
	defer cancel()

	count, err := h.service.Notifications.UnreadCount(r.Context(), userID)
	if err == nil {
		_ = writeEvent(w, flusher, realtime.Event{Type: "notification.new", Payload: map[string]int{"unread_count": count}})
	}

	heartbeat := time.NewTicker(15 * time.Second)
	defer heartbeat.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case event, ok := <-channel:
			if !ok {
				return
			}
			if err := writeEvent(w, flusher, event); err != nil {
				return
			}
		case <-heartbeat.C:
			if currentUserID, err := h.service.Auth.GetUserIDFromRefreshToken(r.Context(), cookie.Value); err != nil || currentUserID != userID {
				return
			}
			if _, err := fmt.Fprint(w, ": heartbeat\n\n"); err != nil {
				return
			}
			flusher.Flush()
		}
	}
}

func writeEvent(w http.ResponseWriter, flusher http.Flusher, event realtime.Event) error {
	payload, err := json.Marshal(event.Payload)
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event.Type, payload); err != nil {
		return err
	}
	flusher.Flush()
	return nil
}
