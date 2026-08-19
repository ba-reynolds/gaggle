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

// ListHandler handles HTTP requests for user-managed lists.
type ListHandler struct {
	service *service.Service
	logger  *slog.Logger
}

func NewListHandler(service *service.Service, logger *slog.Logger) *ListHandler {
	return &ListHandler{
		service: service,
		logger:  logger,
	}
}

func (h *ListHandler) respondError(w http.ResponseWriter, err error) {
	if appErr, ok := err.(*apperrors.AppError); ok {
		util.RespondWithAppError(w, appErr)
		return
	}
	util.RespondWithAppError(w, apperrors.InternalServerError(err))
}

// CreateList godoc
//
// @Summary      Create a list
// @Description  Creates a new list owned by the authenticated user.
// @Tags         lists
// @Accept       json
// @Produce      json
// @Param        payload body models.CreateListPayload true "List details"
// @Success      201 {object} models.Envelope{data=models.List}
// @Failure      400 {object} models.Envelope{data=nil,error=apperrors.AppError}
// @Failure      409 {object} models.Envelope{data=nil,error=apperrors.AppError}
// @Failure      500 {object} models.Envelope{data=nil,error=apperrors.AppError}
// @Security     ApiKeyAuth
// @Router       /lists [post]
func (h *ListHandler) CreateList(w http.ResponseWriter, r *http.Request) {
	user, err := middleware.GetAuthenticatedUserFromContext(r)
	if err != nil {
		h.respondError(w, apperrors.InternalServerError(err))
		return
	}
	var payload models.CreateListPayload
	if err := util.ReadJSON(r, &payload); err != nil {
		h.respondError(w, apperrors.PayloadValidationError(err))
		return
	}
	list, err := h.service.Lists.Create(r.Context(), user.ID, payload)
	if err != nil {
		h.respondError(w, err)
		return
	}
	util.RespondWithJson(w, http.StatusCreated, list)
}

// ListMyLists godoc
//
// @Summary      List the user's lists
// @Description  Returns the lists owned by the authenticated user.
// @Tags         lists
// @Produce      json
// @Success      200 {object} models.Envelope{data=[]models.List}
// @Failure      500 {object} models.Envelope{data=nil,error=apperrors.AppError}
// @Security     ApiKeyAuth
// @Router       /lists [get]
func (h *ListHandler) ListMyLists(w http.ResponseWriter, r *http.Request) {
	user, err := middleware.GetAuthenticatedUserFromContext(r)
	if err != nil {
		h.respondError(w, apperrors.InternalServerError(err))
		return
	}
	lists, err := h.service.Lists.ListForUser(r.Context(), user.ID)
	if err != nil {
		h.respondError(w, err)
		return
	}
	util.RespondWithJson(w, http.StatusOK, lists)
}

// UpdateList godoc
//
// @Summary      Update a list
// @Description  Edits a list's name and description (owner only).
// @Tags         lists
// @Accept       json
// @Produce      json
// @Param        listID  path int                    true "List ID"
// @Param        payload body models.CreateListPayload true "List details"
// @Success      200 {object} models.Envelope{data=models.List}
// @Failure      400 {object} models.Envelope{data=nil,error=apperrors.AppError}
// @Failure      403 {object} models.Envelope{data=nil,error=apperrors.AppError}
// @Failure      404 {object} models.Envelope{data=nil,error=apperrors.AppError}
// @Failure      409 {object} models.Envelope{data=nil,error=apperrors.AppError}
// @Failure      500 {object} models.Envelope{data=nil,error=apperrors.AppError}
// @Security     ApiKeyAuth
// @Router       /lists/{listID} [patch]
func (h *ListHandler) UpdateList(w http.ResponseWriter, r *http.Request) {
	user, err := middleware.GetAuthenticatedUserFromContext(r)
	if err != nil {
		h.respondError(w, apperrors.InternalServerError(err))
		return
	}
	listID, err := strconv.Atoi(r.PathValue("listID"))
	if err != nil {
		h.respondError(w, apperrors.BadRequestError("invalid list ID", err))
		return
	}
	var payload models.CreateListPayload
	if err := util.ReadJSON(r, &payload); err != nil {
		h.respondError(w, apperrors.PayloadValidationError(err))
		return
	}
	list, err := h.service.Lists.Update(r.Context(), listID, user.ID, payload)
	if err != nil {
		h.respondError(w, err)
		return
	}
	util.RespondWithJson(w, http.StatusOK, list)
}

// GetList godoc
//
// @Summary      Get a list
// @Description  Returns a single public list by ID.
// @Tags         lists
// @Produce      json
// @Param        listID path int true "List ID"
// @Success      200 {object} models.Envelope{data=models.List}
// @Failure      404 {object} models.Envelope{data=nil,error=apperrors.AppError}
// @Failure      500 {object} models.Envelope{data=nil,error=apperrors.AppError}
// @Security     ApiKeyAuth
// @Router       /lists/{listID} [get]
func (h *ListHandler) GetList(w http.ResponseWriter, r *http.Request) {
	listID, err := strconv.Atoi(r.PathValue("listID"))
	if err != nil {
		h.respondError(w, apperrors.BadRequestError("invalid list ID", err))
		return
	}
	list, err := h.service.Lists.Get(r.Context(), listID)
	if err != nil {
		h.respondError(w, err)
		return
	}
	util.RespondWithJson(w, http.StatusOK, list)
}

// DeleteList godoc
//
// @Summary      Delete a list
// @Description  Deletes a list (owner only).
// @Tags         lists
// @Produce      json
// @Param        listID path int true "List ID"
// @Success      200 {object} models.Envelope{data=map[string]bool}
// @Failure      403 {object} models.Envelope{data=nil,error=apperrors.AppError}
// @Failure      404 {object} models.Envelope{data=nil,error=apperrors.AppError}
// @Failure      500 {object} models.Envelope{data=nil,error=apperrors.AppError}
// @Security     ApiKeyAuth
// @Router       /lists/{listID} [delete]
func (h *ListHandler) DeleteList(w http.ResponseWriter, r *http.Request) {
	user, err := middleware.GetAuthenticatedUserFromContext(r)
	if err != nil {
		h.respondError(w, apperrors.InternalServerError(err))
		return
	}
	listID, err := strconv.Atoi(r.PathValue("listID"))
	if err != nil {
		h.respondError(w, apperrors.BadRequestError("invalid list ID", err))
		return
	}
	if err := h.service.Lists.Delete(r.Context(), listID, user.ID); err != nil {
		h.respondError(w, err)
		return
	}
	util.RespondWithJson(w, http.StatusOK, map[string]bool{"success": true})
}

// ListFeed godoc
//
// @Summary      Get a list's feed
// @Description  Returns top-level posts from the users in a public list.
// @Tags         lists
// @Produce      json
// @Param        listID path int true "List ID"
// @Param        limit  query int false "Maximum number of posts to retrieve"
// @Param        cursor query string false "Cursor for pagination"
// @Success      200 {object} models.Envelope{data=models.PostFeed,error=nil}
// @Failure      404 {object} models.Envelope{data=nil,error=apperrors.AppError}
// @Failure      500 {object} models.Envelope{data=nil,error=apperrors.AppError}
// @Security     ApiKeyAuth
// @Router       /lists/{listID}/feed [get]
func (h *ListHandler) ListFeed(w http.ResponseWriter, r *http.Request) {
	user, err := middleware.GetAuthenticatedUserFromContext(r)
	if err != nil {
		h.respondError(w, apperrors.InternalServerError(err))
		return
	}
	listID, err := strconv.Atoi(r.PathValue("listID"))
	if err != nil {
		h.respondError(w, apperrors.BadRequestError("invalid list ID", err))
		return
	}
	limit, cursor, _ := util.ParsePaginationParams(r.URL.Query().Get("limit"), r.URL.Query().Get("cursor"), 20, 100)
	feed, err := h.service.Lists.GetListFeed(r.Context(), user.ID, listID, limit, cursor)
	if err != nil {
		h.respondError(w, err)
		return
	}
	util.RespondWithJson(w, http.StatusOK, feed)
}

// ListMembers godoc
//
// @Summary      Get a list's members
// @Description  Returns the users in a public list.
// @Tags         lists
// @Produce      json
// @Param        listID path int true "List ID"
// @Param        limit  query int false "Maximum number of members to retrieve"
// @Param        cursor query string false "Cursor for pagination"
// @Success      200 {object} models.Envelope{data=models.ListMembersResponse,error=nil}
// @Failure      404 {object} models.Envelope{data=nil,error=apperrors.AppError}
// @Failure      500 {object} models.Envelope{data=nil,error=apperrors.AppError}
// @Security     ApiKeyAuth
// @Router       /lists/{listID}/members [get]
func (h *ListHandler) ListMembers(w http.ResponseWriter, r *http.Request) {
	listID, err := strconv.Atoi(r.PathValue("listID"))
	if err != nil {
		h.respondError(w, apperrors.BadRequestError("invalid list ID", err))
		return
	}
	limit, cursor, _ := util.ParsePaginationParams(r.URL.Query().Get("limit"), r.URL.Query().Get("cursor"), 20, 100)
	members, err := h.service.Lists.GetMembers(r.Context(), listID, limit, cursor)
	if err != nil {
		h.respondError(w, err)
		return
	}
	if err := h.service.Badges.HydrateProfiles(r.Context(), members.Items); err != nil {
		h.respondError(w, err)
		return
	}
	util.RespondWithJson(w, http.StatusOK, members)
}

// AddMember godoc
//
// @Summary      Add a user to a list
// @Description  Adds the given user to a list (owner only).
// @Tags         lists
// @Produce      json
// @Param        listID   path int    true "List ID"
// @Param        username path string true "Username to add"
// @Success      200 {object} models.Envelope{data=map[string]bool}
// @Failure      400 {object} models.Envelope{data=nil,error=apperrors.AppError}
// @Failure      403 {object} models.Envelope{data=nil,error=apperrors.AppError}
// @Failure      404 {object} models.Envelope{data=nil,error=apperrors.AppError}
// @Failure      409 {object} models.Envelope{data=nil,error=apperrors.AppError}
// @Failure      500 {object} models.Envelope{data=nil,error=apperrors.AppError}
// @Security     ApiKeyAuth
// @Router       /lists/{listID}/members/{username} [post]
func (h *ListHandler) AddMember(w http.ResponseWriter, r *http.Request) {
	user, err := middleware.GetAuthenticatedUserFromContext(r)
	if err != nil {
		h.respondError(w, apperrors.InternalServerError(err))
		return
	}
	listID, err := strconv.Atoi(r.PathValue("listID"))
	if err != nil {
		h.respondError(w, apperrors.BadRequestError("invalid list ID", err))
		return
	}
	username := r.PathValue("username")
	target, err := h.service.Users.GetByUsername(r.Context(), username)
	if err != nil {
		h.respondError(w, err)
		return
	}
	if target.SoftDeleted {
		h.respondError(w, apperrors.NotFoundError("user not found", nil))
		return
	}
	if err := h.service.Lists.AddMember(r.Context(), listID, user.ID, target.ID); err != nil {
		h.respondError(w, err)
		return
	}
	util.RespondWithJson(w, http.StatusOK, map[string]bool{"success": true})
}

// RemoveMember godoc
//
// @Summary      Remove a user from a list
// @Description  Removes the given user from a list (owner only).
// @Tags         lists
// @Produce      json
// @Param        listID   path int    true "List ID"
// @Param        username path string true "Username to remove"
// @Success      200 {object} models.Envelope{data=map[string]bool}
// @Failure      403 {object} models.Envelope{data=nil,error=apperrors.AppError}
// @Failure      404 {object} models.Envelope{data=nil,error=apperrors.AppError}
// @Failure      500 {object} models.Envelope{data=nil,error=apperrors.AppError}
// @Security     ApiKeyAuth
// @Router       /lists/{listID}/members/{username} [delete]
func (h *ListHandler) RemoveMember(w http.ResponseWriter, r *http.Request) {
	user, err := middleware.GetAuthenticatedUserFromContext(r)
	if err != nil {
		h.respondError(w, apperrors.InternalServerError(err))
		return
	}
	listID, err := strconv.Atoi(r.PathValue("listID"))
	if err != nil {
		h.respondError(w, apperrors.BadRequestError("invalid list ID", err))
		return
	}
	target, err := h.service.Users.GetByUsername(r.Context(), r.PathValue("username"))
	if err != nil {
		h.respondError(w, err)
		return
	}
	if err := h.service.Lists.RemoveMember(r.Context(), listID, user.ID, target.ID); err != nil {
		h.respondError(w, err)
		return
	}
	util.RespondWithJson(w, http.StatusOK, map[string]bool{"success": true})
}

// GetUserLists godoc
//
// @Summary      Get a user's lists
// @Description  Returns the public lists owned by the given user.
// @Tags         lists
// @Produce      json
// @Param        username path string true "Username"
// @Success      200 {object} models.Envelope{data=[]models.List}
// @Failure      404 {object} models.Envelope{data=nil,error=apperrors.AppError}
// @Failure      500 {object} models.Envelope{data=nil,error=apperrors.AppError}
// @Security     ApiKeyAuth
// @Router       /users/{username}/lists [get]
func (h *ListHandler) GetUserLists(w http.ResponseWriter, r *http.Request) {
	user, err := h.service.Users.GetByUsername(r.Context(), r.PathValue("username"))
	if err != nil {
		h.respondError(w, err)
		return
	}
	lists, err := h.service.Lists.ListForUser(r.Context(), user.ID)
	if err != nil {
		h.respondError(w, err)
		return
	}
	util.RespondWithJson(w, http.StatusOK, lists)
}
