package handlers

import (
	"log/slog"
	"net/http"

	"github.com/ba-reynolds/gophersocial/internal/apperrors"
	"github.com/ba-reynolds/gophersocial/internal/middleware"
	"github.com/ba-reynolds/gophersocial/internal/service"
	"github.com/ba-reynolds/gophersocial/internal/util"
)

type SearchHandler struct {
	service *service.Service
	logger  *slog.Logger
}

func NewSearchHandler(service *service.Service, logger *slog.Logger) *SearchHandler {
	return &SearchHandler{service: service, logger: logger}
}

func (h *SearchHandler) Search(w http.ResponseWriter, r *http.Request) {
	user, err := middleware.GetAuthenticatedUserFromContext(r)
	if err != nil {
		util.RespondWithAppError(w, apperrors.InternalServerError(err))
		return
	}
	query := r.URL.Query().Get("q")
	searchType := r.URL.Query().Get("type")
	limit, cursor, _ := util.ParsePaginationParams(r.URL.Query().Get("limit"), r.URL.Query().Get("cursor"), 20, 100)
	if searchType == "users" {
		users, err := h.service.Search.Users(r.Context(), query, limit)
		if err != nil {
			h.respondError(w, err)
			return
		}
		util.RespondWithJson(w, http.StatusOK, users)
		return
	}
	posts, err := h.service.Search.Posts(r.Context(), user.ID, query, limit, cursor)
	if err != nil {
		h.respondError(w, err)
		return
	}
	util.RespondWithJson(w, http.StatusOK, posts)
}

func (h *SearchHandler) HashtagPosts(w http.ResponseWriter, r *http.Request) {
	user, err := middleware.GetAuthenticatedUserFromContext(r)
	if err != nil {
		util.RespondWithAppError(w, apperrors.InternalServerError(err))
		return
	}
	limit, cursor, _ := util.ParsePaginationParams(r.URL.Query().Get("limit"), r.URL.Query().Get("cursor"), 20, 100)
	posts, err := h.service.Search.HashtagPosts(r.Context(), user.ID, r.PathValue("tag"), limit, cursor)
	if err != nil {
		h.respondError(w, err)
		return
	}
	util.RespondWithJson(w, http.StatusOK, posts)
}

func (h *SearchHandler) Trends(w http.ResponseWriter, r *http.Request) {
	if _, err := middleware.GetAuthenticatedUserFromContext(r); err != nil {
		util.RespondWithAppError(w, apperrors.InternalServerError(err))
		return
	}
	limit, _, _ := util.ParsePaginationParams(r.URL.Query().Get("limit"), "", 10, 20)
	trends, err := h.service.Search.Trends(r.Context(), limit)
	if err != nil {
		h.respondError(w, err)
		return
	}
	util.RespondWithJson(w, http.StatusOK, trends)
}

func (h *SearchHandler) respondError(w http.ResponseWriter, err error) {
	h.logger.Error("search request failed", "error", err)
	if appErr, ok := err.(*apperrors.AppError); ok {
		util.RespondWithAppError(w, appErr)
		return
	}
	util.RespondWithAppError(w, apperrors.InternalServerError(err))
}
