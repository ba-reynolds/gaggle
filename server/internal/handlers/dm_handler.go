package handlers

import (
	"log/slog"
	"net/http"
	"strconv"

	"github.com/ba-reynolds/gophersocial/internal/apperrors"
	"github.com/ba-reynolds/gophersocial/internal/middleware"
	"github.com/ba-reynolds/gophersocial/internal/models"
	"github.com/ba-reynolds/gophersocial/internal/service"
	"github.com/ba-reynolds/gophersocial/internal/util"
)

// DmHandler handles HTTP requests for direct messages.
type DmHandler struct {
	service *service.Service
	logger  *slog.Logger
}

func NewDmHandler(service *service.Service, logger *slog.Logger) *DmHandler {
	return &DmHandler{
		service: service,
		logger:  logger,
	}
}

func (h *DmHandler) respondError(w http.ResponseWriter, err error) {
	if appErr, ok := err.(*apperrors.AppError); ok {
		util.RespondWithAppError(w, appErr)
		return
	}
	util.RespondWithAppError(w, apperrors.InternalServerError(err))
}

// SendDm godoc
//
// @Summary      Send a direct message
// @Description  Sends a 1:1 direct message, creating the conversation on first contact.
// @Tags         dms
// @Accept       json
// @Produce      json
// @Param        username path string true "Recipient username"
// @Param        payload body models.SendMessagePayload true "Message body"
// @Success      201 {object} models.Envelope{data=models.Message}
// @Failure      400 {object} models.Envelope{data=nil,error=apperrors.AppError}
// @Failure      403 {object} models.Envelope{data=nil,error=apperrors.AppError}
// @Failure      404 {object} models.Envelope{data=nil,error=apperrors.AppError}
// @Failure      500 {object} models.Envelope{data=nil,error=apperrors.AppError}
// @Security     ApiKeyAuth
// @Router       /dms/{username} [post]
func (h *DmHandler) SendDm(w http.ResponseWriter, r *http.Request) {
	user, err := middleware.GetAuthenticatedUserFromContext(r)
	if err != nil {
		h.respondError(w, apperrors.InternalServerError(err))
		return
	}
	var payload models.SendMessagePayload
	if err := util.ReadJSON(r, &payload); err != nil {
		h.respondError(w, apperrors.PayloadValidationError(err))
		return
	}
	message, err := h.service.DMs.Send(r.Context(), user.ID, r.PathValue("username"), payload.Body)
	if err != nil {
		h.respondError(w, err)
		return
	}
	util.RespondWithJson(w, http.StatusCreated, message)
}

// ListConversations godoc
//
// @Summary      List conversations
// @Description  Returns the authenticated user's inbox, newest activity first.
// @Tags         dms
// @Produce      json
// @Success      200 {object} models.Envelope{data=models.ConversationFeed}
// @Failure      500 {object} models.Envelope{data=nil,error=apperrors.AppError}
// @Security     ApiKeyAuth
// @Router       /dms/conversations [get]
func (h *DmHandler) ListConversations(w http.ResponseWriter, r *http.Request) {
	user, err := middleware.GetAuthenticatedUserFromContext(r)
	if err != nil {
		h.respondError(w, apperrors.InternalServerError(err))
		return
	}
	feed, err := h.service.DMs.ListConversations(r.Context(), user.ID)
	if err != nil {
		h.respondError(w, err)
		return
	}
	util.RespondWithJson(w, http.StatusOK, feed)
}

// ListMessages godoc
//
// @Summary      List a conversation's messages
// @Description  Returns a conversation's message history (participants only), newest first.
// @Tags         dms
// @Produce      json
// @Param        conversationID path int true "Conversation ID"
// @Param        limit query int false "Maximum number of messages"
// @Param        cursor query string false "Cursor for pagination"
// @Success      200 {object} models.Envelope{data=models.MessageFeed,error=nil}
// @Failure      404 {object} models.Envelope{data=nil,error=apperrors.AppError}
// @Failure      500 {object} models.Envelope{data=nil,error=apperrors.AppError}
// @Security     ApiKeyAuth
// @Router       /dms/conversations/{conversationID}/messages [get]
func (h *DmHandler) ListMessages(w http.ResponseWriter, r *http.Request) {
	user, err := middleware.GetAuthenticatedUserFromContext(r)
	if err != nil {
		h.respondError(w, apperrors.InternalServerError(err))
		return
	}
	conversationID, err := strconv.Atoi(r.PathValue("conversationID"))
	if err != nil {
		h.respondError(w, apperrors.BadRequestError("invalid conversation ID", err))
		return
	}
	limit, cursor, _ := util.ParsePaginationParams(r.URL.Query().Get("limit"), r.URL.Query().Get("cursor"), 20, 100)
	feed, err := h.service.DMs.ListMessages(r.Context(), user.ID, conversationID, limit, cursor)
	if err != nil {
		h.respondError(w, err)
		return
	}
	util.RespondWithJson(w, http.StatusOK, feed)
}

// GetConversation godoc
//
// @Summary      Get a conversation
// @Description  Returns a single conversation (participants only).
// @Tags         dms
// @Produce      json
// @Param        conversationID path int true "Conversation ID"
// @Success      200 {object} models.Envelope{data=models.Conversation}
// @Failure      404 {object} models.Envelope{data=nil,error=apperrors.AppError}
// @Failure      500 {object} models.Envelope{data=nil,error=apperrors.AppError}
// @Security     ApiKeyAuth
// @Router       /dms/conversations/{conversationID} [get]
func (h *DmHandler) GetConversation(w http.ResponseWriter, r *http.Request) {
	user, err := middleware.GetAuthenticatedUserFromContext(r)
	if err != nil {
		h.respondError(w, apperrors.InternalServerError(err))
		return
	}
	conversationID, err := strconv.Atoi(r.PathValue("conversationID"))
	if err != nil {
		h.respondError(w, apperrors.BadRequestError("invalid conversation ID", err))
		return
	}
	conversation, err := h.service.DMs.GetConversation(r.Context(), user.ID, conversationID)
	if err != nil {
		h.respondError(w, err)
		return
	}
	util.RespondWithJson(w, http.StatusOK, conversation)
}

// UnreadCount godoc
//
// @Summary      Get unread DM count
// @Description  Returns the authenticated user's total unread direct message count.
// @Tags         dms
// @Produce      json
// @Success      200 {object} models.Envelope{data=map[string]int}
// @Failure      500 {object} models.Envelope{data=nil,error=apperrors.AppError}
// @Security     ApiKeyAuth
// @Router       /dms/unread-count [get]
func (h *DmHandler) UnreadCount(w http.ResponseWriter, r *http.Request) {
	user, err := middleware.GetAuthenticatedUserFromContext(r)
	if err != nil {
		h.respondError(w, apperrors.InternalServerError(err))
		return
	}
	count, err := h.service.DMs.UnreadCount(r.Context(), user.ID)
	if err != nil {
		h.respondError(w, err)
		return
	}
	util.RespondWithJson(w, http.StatusOK, map[string]int{"unread_count": count})
}

// MarkRead godoc
//
// @Summary      Mark a conversation read
// @Description  Marks incoming messages in a conversation as read (participants only).
// @Tags         dms
// @Produce      json
// @Param        conversationID path int true "Conversation ID"
// @Success      200 {object} models.Envelope{data=map[string]bool}
// @Failure      404 {object} models.Envelope{data=nil,error=apperrors.AppError}
// @Failure      500 {object} models.Envelope{data=nil,error=apperrors.AppError}
// @Security     ApiKeyAuth
// @Router       /dms/conversations/{conversationID}/read [post]
func (h *DmHandler) MarkRead(w http.ResponseWriter, r *http.Request) {
	user, err := middleware.GetAuthenticatedUserFromContext(r)
	if err != nil {
		h.respondError(w, apperrors.InternalServerError(err))
		return
	}
	conversationID, err := strconv.Atoi(r.PathValue("conversationID"))
	if err != nil {
		h.respondError(w, apperrors.BadRequestError("invalid conversation ID", err))
		return
	}
	if err := h.service.DMs.MarkRead(r.Context(), user.ID, conversationID); err != nil {
		h.respondError(w, err)
		return
	}
	util.RespondWithJson(w, http.StatusOK, map[string]bool{"success": true})
}
