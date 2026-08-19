package handlers

import (
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/ba-reynolds/gophersocial/internal/apperrors"
	"github.com/ba-reynolds/gophersocial/internal/middleware"
	"github.com/ba-reynolds/gophersocial/internal/models"
	"github.com/ba-reynolds/gophersocial/internal/service"
	"github.com/ba-reynolds/gophersocial/internal/util"
)

// Keep the models import referenced for swag annotation resolution.
var _ models.PostFeed

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
		if err := h.service.Badges.HydrateProfiles(r.Context(), users.Items); err != nil {
			h.logger.Error("failed to hydrate badges", "error", err)
			h.respondError(w, err)
			return
		}
		util.RespondWithJson(w, http.StatusOK, users)
		return
	}
	filters, err := parseSearchFilters(r.URL.Query())
	if err != nil {
		h.respondError(w, err)
		return
	}
	posts, err := h.service.Search.Posts(r.Context(), user.ID, query, filters, limit, cursor)
	if err != nil {
		h.respondError(w, err)
		return
	}
	util.RespondWithJson(w, http.StatusOK, posts)
}

// parseSearchFilters reads the optional post search filter query params with
// loose Boolean handling: has_media and include_replies are treated as true
// only for the literal value "true". Dates accept RFC3339 or YYYY-MM-DD.
func parseSearchFilters(values url.Values) (models.PostSearchFilters, error) {
	filters := models.PostSearchFilters{
		From:    strings.TrimSpace(values.Get("from")),
		Hashtag: strings.TrimSpace(values.Get("hashtag")),
	}
	if values.Get("has_media") == "true" {
		filters.HasMedia = true
	}
	if n := strings.TrimSpace(values.Get("min_likes")); n != "" {
		v, err := strconv.Atoi(n)
		if err != nil || v < 0 {
			return filters, apperrors.BadRequestError("min_likes must be a non-negative integer", nil)
		}
		filters.MinLikes = v
	}
	if values.Get("include_replies") == "true" {
		filters.IncludeReplies = true
	}
	for _, key := range []string{"since", "until"} {
		if raw := values.Get(key); raw != "" {
			dateOnly, t, err := parseSearchTime(raw)
			if err != nil {
				return filters, apperrors.BadRequestError(key+" must be an RFC3339 timestamp or YYYY-MM-DD date", nil)
			}
			if key == "until" && dateOnly {
				t = t.Add(24 * time.Hour)
			}
			if key == "since" {
				filters.Since = &t
			} else {
				filters.Until = &t
			}
		}
	}
	return filters, nil
}

func parseSearchTime(raw string) (dateOnly bool, t time.Time, err error) {
	if t, err := time.Parse(time.RFC3339, raw); err == nil {
		return false, t, nil
	}
	t, err = time.Parse("2006-01-02", raw)
	return true, t, err
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

// Mentions godoc
//
// @Summary      List posts mentioning the viewer
// @Description  Returns posts that tagged the authenticated user with @username, newest first.
// @Tags         search
// @Produce      json
// @Param        limit query int false "Maximum number of posts"
// @Param        cursor query string false "Cursor for pagination"
// @Success      200 {object} models.Envelope{data=models.PostFeed}
// @Failure      500 {object} models.Envelope{data=nil,error=apperrors.AppError}
// @Security     ApiKeyAuth
// @Router       /mentions [get]
func (h *SearchHandler) Mentions(w http.ResponseWriter, r *http.Request) {
	user, err := middleware.GetAuthenticatedUserFromContext(r)
	if err != nil {
		util.RespondWithAppError(w, apperrors.InternalServerError(err))
		return
	}
	limit, cursor, _ := util.ParsePaginationParams(r.URL.Query().Get("limit"), r.URL.Query().Get("cursor"), 20, 100)
	posts, err := h.service.Search.Mentions(r.Context(), user.ID, limit, cursor)
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
