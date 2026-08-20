package handlers

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/ba-reynolds/gaggle/internal/apperrors"
	"github.com/ba-reynolds/gaggle/internal/cache"
	"github.com/ba-reynolds/gaggle/internal/middleware"
	"github.com/ba-reynolds/gaggle/internal/models"
	"github.com/ba-reynolds/gaggle/internal/service"
	"github.com/ba-reynolds/gaggle/internal/util"
)

type PostEngagementHandler struct {
	service *service.Service
	logger  *slog.Logger
	rdb     *cache.Client
}

func NewPostEngagementHandler(service *service.Service, logger *slog.Logger, rdb *cache.Client) *PostEngagementHandler {
	return &PostEngagementHandler{service: service, logger: logger, rdb: rdb}
}

// invalidateActorFeed clears the cached home feed for the user who performed
// an engagement action (their feed ordering/content may have changed).
func (h *PostEngagementHandler) invalidateActorFeed(ctx context.Context, userID int) {
	if h.rdb != nil {
		h.rdb.InvalidateHomeFeed(ctx, userID)
	}
}

// requirePostVisible returns false (and writes a 404) when the viewer may not
// read the given post — either because the author's account is private, or the
// post's own visibility rule excludes them. Engagement writes must never be
// possible on a post the actor cannot see.
func (h *PostEngagementHandler) requirePostVisible(w http.ResponseWriter, r *http.Request, postID, viewerID int) bool {
	visible, err := h.service.Posts.CanViewPost(r.Context(), postID, viewerID)
	if err != nil {
		if appErr, ok := err.(*apperrors.AppError); ok && appErr.Code == apperrors.NotFound {
			util.RespondWithAppError(w, apperrors.NotFoundError("post not found", nil))
			return false
		}
		h.logger.Error("failed to check post visibility", "error", err, "postID", postID, "viewerID", viewerID)
		util.RespondWithAppError(w, apperrors.InternalServerError(err))
		return false
	}
	if !visible {
		util.RespondWithAppError(w, apperrors.NotFoundError("post not found", nil))
		return false
	}
	return true
}

// Like godoc
//
// @Summary      Like a post
// @Description  Likes a post by its ID
// @Tags         posts
// @Accept       json
// @Produce      json
// @Param        postID path int true "Post ID"
// @Success      200 {object} models.LikeResponse
// @Failure      400 {object} models.Envelope{data=nil,error=apperrors.AppError}
// @Failure      500 {object} models.Envelope{data=nil,error=apperrors.AppError}
// @Security     ApiKeyAuth
// @Router       /posts/{postID}/like [post]
func (h *PostEngagementHandler) Like(w http.ResponseWriter, r *http.Request) {
	user, err := middleware.GetAuthenticatedUserFromContext(r)
	if err != nil {
		h.logger.Error("authentication middleware error", "error", err)
		util.RespondWithAppError(w, apperrors.InternalServerError(err))
		return
	}
	postID, err := strconv.Atoi(r.PathValue("postID"))
	if err != nil {
		h.logger.Warn("invalid post ID parameter", "postID", r.PathValue("postID"), "error", err)
		util.RespondWithAppError(w, apperrors.BadRequestError("invalid post ID", err))
		return
	}
	if !h.requirePostVisible(w, r, postID, user.ID) {
		return
	}
	created, err := h.service.PostEngagements.Like(r.Context(), postID, user.ID)
	if err != nil {
		if appErr, ok := err.(*apperrors.AppError); ok {
			util.RespondWithAppError(w, appErr)
			return
		}
		util.RespondWithAppError(w, apperrors.InternalServerError(err))
		return
	}
	h.invalidateActorFeed(r.Context(), user.ID)
	if created {
		if err := h.service.Notifications.CreateForPost(r.Context(), user.ID, postID, "like"); err != nil {
			h.logger.Warn("failed to create like notification", "postID", postID, "userID", user.ID, "error", err)
		}
	}
	util.RespondWithJson(w, http.StatusOK, models.LikeResponse{Success: true})
}

// Unlike godoc
//
// @Summary      Unlike a post
// @Description  Removes a like from a post by its ID
// @Tags         posts
// @Accept       json
// @Produce      json
// @Param        postID path int true "Post ID"
// @Success      200 {object} models.LikeResponse
// @Failure      400 {object} models.Envelope{data=nil,error=apperrors.AppError}
// @Failure      500 {object} models.Envelope{data=nil,error=apperrors.AppError}
// @Security     ApiKeyAuth
// @Router       /posts/{postID}/like [delete]
func (h *PostEngagementHandler) Unlike(w http.ResponseWriter, r *http.Request) {
	user, err := middleware.GetAuthenticatedUserFromContext(r)
	if err != nil {
		h.logger.Error("authentication middleware error", "error", err)
		util.RespondWithAppError(w, apperrors.InternalServerError(err))
		return
	}
	postID, err := strconv.Atoi(r.PathValue("postID"))
	if err != nil {
		h.logger.Warn("invalid post ID parameter", "postID", r.PathValue("postID"), "error", err)
		util.RespondWithAppError(w, apperrors.BadRequestError("invalid post ID", err))
		return
	}
	if err := h.service.PostEngagements.Unlike(r.Context(), postID, user.ID); err != nil {
		if appErr, ok := err.(*apperrors.AppError); ok {
			util.RespondWithAppError(w, appErr)
			return
		}
		util.RespondWithAppError(w, apperrors.InternalServerError(err))
		return
	}
	h.invalidateActorFeed(r.Context(), user.ID)
	util.RespondWithJson(w, http.StatusOK, models.LikeResponse{Success: true})
}

// Repost godoc
//
// @Summary      Repost a post
// @Description  Reposts a post by its ID
// @Tags         posts
// @Accept       json
// @Produce      json
// @Param        postID path int true "Post ID"
// @Success      200 {object} models.RepostResponse
// @Failure      400 {object} models.Envelope{data=nil,error=apperrors.AppError}
// @Failure      500 {object} models.Envelope{data=nil,error=apperrors.AppError}
// @Security     ApiKeyAuth
// @Router       /posts/{postID}/repost [post]
func (h *PostEngagementHandler) Repost(w http.ResponseWriter, r *http.Request) {
	user, err := middleware.GetAuthenticatedUserFromContext(r)
	if err != nil {
		h.logger.Error("authentication middleware error", "error", err)
		util.RespondWithAppError(w, apperrors.InternalServerError(err))
		return
	}
	postID, err := strconv.Atoi(r.PathValue("postID"))
	if err != nil {
		h.logger.Warn("invalid post ID parameter", "postID", r.PathValue("postID"), "error", err)
		util.RespondWithAppError(w, apperrors.BadRequestError("invalid post ID", err))
		return
	}
	if !h.requirePostVisible(w, r, postID, user.ID) {
		return
	}
	created, err := h.service.PostEngagements.Repost(r.Context(), postID, user.ID)
	if err != nil {
		if appErr, ok := err.(*apperrors.AppError); ok {
			util.RespondWithAppError(w, appErr)
			return
		}
		util.RespondWithAppError(w, apperrors.InternalServerError(err))
		return
	}
	h.invalidateActorFeed(r.Context(), user.ID)
	if created {
		if err := h.service.Notifications.CreateForPost(r.Context(), user.ID, postID, "repost"); err != nil {
			h.logger.Warn("failed to create repost notification", "postID", postID, "userID", user.ID, "error", err)
		}
	}
	util.RespondWithJson(w, http.StatusOK, models.RepostResponse{Success: true})
}

// Unrepost godoc
//
// @Summary      Remove repost
// @Description  Removes a repost from a post by its ID
// @Tags         posts
// @Accept       json
// @Produce      json
// @Param        postID path int true "Post ID"
// @Success      200 {object} models.RepostResponse
// @Failure      400 {object} models.Envelope{data=nil,error=apperrors.AppError}
// @Failure      500 {object} models.Envelope{data=nil,error=apperrors.AppError}
// @Security     ApiKeyAuth
// @Router       /posts/{postID}/repost [delete]
func (h *PostEngagementHandler) Unrepost(w http.ResponseWriter, r *http.Request) {
	user, err := middleware.GetAuthenticatedUserFromContext(r)
	if err != nil {
		h.logger.Error("authentication middleware error", "error", err)
		util.RespondWithAppError(w, apperrors.InternalServerError(err))
		return
	}
	postID, err := strconv.Atoi(r.PathValue("postID"))
	if err != nil {
		h.logger.Warn("invalid post ID parameter", "postID", r.PathValue("postID"), "error", err)
		util.RespondWithAppError(w, apperrors.BadRequestError("invalid post ID", err))
		return
	}
	if err := h.service.PostEngagements.Unrepost(r.Context(), postID, user.ID); err != nil {
		if appErr, ok := err.(*apperrors.AppError); ok {
			util.RespondWithAppError(w, appErr)
			return
		}
		util.RespondWithAppError(w, apperrors.InternalServerError(err))
		return
	}
	h.invalidateActorFeed(r.Context(), user.ID)
	util.RespondWithJson(w, http.StatusOK, models.RepostResponse{Success: true})
}

// Bookmark godoc
//
// @Summary      Bookmark a post
// @Description  Bookmarks a post by its ID
// @Tags         posts
// @Accept       json
// @Produce      json
// @Param        postID path int true "Post ID"
// @Param        payload body models.BookmarkRequest true "Bookmark information (category_id only)"
// @Success      200 {object} models.BookmarkResponse
// @Failure      400 {object} models.Envelope{data=nil,error=apperrors.AppError}
// @Failure      500 {object} models.Envelope{data=nil,error=apperrors.AppError}
// @Security     ApiKeyAuth
// @Router       /posts/{postID}/bookmark [post]
func (h *PostEngagementHandler) Bookmark(w http.ResponseWriter, r *http.Request) {
	user, err := middleware.GetAuthenticatedUserFromContext(r)
	if err != nil {
		h.logger.Error("authentication middleware error", "error", err)
		util.RespondWithAppError(w, apperrors.InternalServerError(err))
		return
	}
	postID, err := strconv.Atoi(r.PathValue("postID"))
	if err != nil {
		h.logger.Warn("invalid post ID parameter", "postID", r.PathValue("postID"), "error", err)
		util.RespondWithAppError(w, apperrors.BadRequestError("invalid post ID", err))
		return
	}
	if !h.requirePostVisible(w, r, postID, user.ID) {
		return
	}
	var payload models.BookmarkRequest
	if err := util.ReadJSON(r, &payload); err != nil {
		h.logger.Warn("invalid JSON payload in request", "error", err, "userID", user.ID)
		util.RespondWithAppError(w, apperrors.PayloadValidationError(err))
		return
	}
	if err := util.Validate.Struct(payload); err != nil {
		h.logger.Warn("payload validation failed", "error", err, "userID", user.ID)
		util.RespondWithAppError(w, apperrors.PayloadValidationError(err))
		return
	}
	if err := h.service.PostEngagements.Bookmark(r.Context(), postID, user.ID, payload.CategoryID); err != nil {
		if appErr, ok := err.(*apperrors.AppError); ok {
			util.RespondWithAppError(w, appErr)
			return
		}
		util.RespondWithAppError(w, apperrors.InternalServerError(err))
		return
	}
	h.invalidateActorFeed(r.Context(), user.ID)
	util.RespondWithJson(w, http.StatusOK, models.BookmarkResponse{Success: true})
}

// Unbookmark godoc
//
// @Summary      Remove bookmark
// @Description  Removes a bookmark from a post by its ID
// @Tags         posts
// @Accept       json
// @Produce      json
// @Param        postID path int true "Post ID"
// @Success      200 {object} models.BookmarkResponse
// @Failure      400 {object} models.Envelope{data=nil,error=apperrors.AppError}
// @Failure      500 {object} models.Envelope{data=nil,error=apperrors.AppError}
// @Security     ApiKeyAuth
// @Router       /posts/{postID}/bookmark [delete]
func (h *PostEngagementHandler) Unbookmark(w http.ResponseWriter, r *http.Request) {
	user, err := middleware.GetAuthenticatedUserFromContext(r)
	if err != nil {
		h.logger.Error("authentication middleware error", "error", err)
		util.RespondWithAppError(w, apperrors.InternalServerError(err))
		return
	}
	postID, err := strconv.Atoi(r.PathValue("postID"))
	if err != nil {
		h.logger.Warn("invalid post ID parameter", "postID", r.PathValue("postID"), "error", err)
		util.RespondWithAppError(w, apperrors.BadRequestError("invalid post ID", err))
		return
	}
	if err := h.service.PostEngagements.Unbookmark(r.Context(), postID, user.ID); err != nil {
		if appErr, ok := err.(*apperrors.AppError); ok {
			util.RespondWithAppError(w, appErr)
			return
		}
		util.RespondWithAppError(w, apperrors.InternalServerError(err))
		return
	}
	h.invalidateActorFeed(r.Context(), user.ID)
	util.RespondWithJson(w, http.StatusOK, models.BookmarkResponse{Success: true})
}

// CreateBookmarkCategory godoc
//
// @Summary      Create a new bookmark category
// @Description  Creates a new bookmark category for the authenticated user
// @Tags         bookmarks
// @Accept       json
// @Produce      json
// @Param        payload body models.CreateBookmarkCategoryRequest true "Bookmark category information"
// @Success      201 {object} models.CreateBookmarkCategoryResponse
// @Failure      400 {object} models.Envelope{data=nil,error=apperrors.AppError}
// @Failure      409 {object} models.Envelope{data=nil,error=apperrors.AppError}
// @Failure      500 {object} models.Envelope{data=nil,error=apperrors.AppError}
// @Security     ApiKeyAuth
// @Router       /bookmarks/category [post]
func (h *PostEngagementHandler) CreateBookmarkCategory(w http.ResponseWriter, r *http.Request) {
	user, err := middleware.GetAuthenticatedUserFromContext(r)
	if err != nil {
		h.logger.Error("authentication middleware error", "error", err)
		util.RespondWithAppError(w, apperrors.InternalServerError(err))
		return
	}
	var payload models.CreateBookmarkCategoryRequest
	if err := util.ReadJSON(r, &payload); err != nil {
		h.logger.Warn("invalid JSON payload in request", "error", err, "userID", user.ID)
		util.RespondWithAppError(w, apperrors.PayloadValidationError(err))
		return
	}
	if err := util.Validate.Struct(payload); err != nil {
		h.logger.Warn("payload validation failed", "error", err, "userID", user.ID)
		util.RespondWithAppError(w, apperrors.PayloadValidationError(err))
		return
	}
	if payload.Color == "" {
		payload.Color = "#1DA1F2" // default color
	}
	cat, err := h.service.PostEngagements.CreateBookmarkCategory(r.Context(), user.ID, payload.CategoryName, payload.Color)
	if err != nil {
		if appErr, ok := err.(*apperrors.AppError); ok {
			util.RespondWithAppError(w, appErr)
			return
		}
		util.RespondWithAppError(w, apperrors.InternalServerError(err))
		return
	}
	util.RespondWithJson(w, http.StatusCreated, models.CreateBookmarkCategoryResponse{Success: true, Category: *cat})
}

// ListBookmarkCategories godoc
//
// @Summary      List bookmark categories
// @Description  Returns all bookmark categories for the authenticated user, plus uncategorized bookmark count
// @Tags         bookmarks
// @Produce      json
// @Success      200 {object} map[string]any
// @Failure      500 {object} models.Envelope{data=nil,error=apperrors.AppError}
// @Security     ApiKeyAuth
// @Router       /bookmarks/category [get]
func (h *PostEngagementHandler) ListBookmarkCategories(w http.ResponseWriter, r *http.Request) {
	user, err := middleware.GetAuthenticatedUserFromContext(r)
	if err != nil {
		h.logger.Error("authentication middleware error", "error", err)
		util.RespondWithAppError(w, apperrors.InternalServerError(err))
		return
	}
	categories, err := h.service.PostEngagements.ListBookmarkCategories(r.Context(), user.ID)
	if err != nil {
		if appErr, ok := err.(*apperrors.AppError); ok {
			util.RespondWithAppError(w, appErr)
			return
		}
		util.RespondWithAppError(w, apperrors.InternalServerError(err))
		return
	}
	uncategorizedCount, err := h.service.PostEngagements.GetUncategorizedBookmarkCount(r.Context(), user.ID)
	if err != nil {
		h.logger.Error("failed to get uncategorized bookmark count", "error", err, "userID", user.ID)
		util.RespondWithAppError(w, apperrors.InternalServerError(err))
		return
	}
	if categories == nil {
		categories = []models.BookmarkCategory{}
	}
	// Return envelope with extra top-level uncategorized_count for backward compat.
	// Frontend reads both envelope.uncategorized_count and data.uncategorized_count.
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(map[string]any{
		"data":                categories,
		"uncategorized_count": uncategorizedCount,
		"error":               nil,
	}); err != nil {
		h.logger.Error("failed to encode bookmark categories", "error", err)
	}
}

// DeleteBookmarkCategory godoc
//
// @Summary      Delete a bookmark category
// @Description  Deletes a bookmark category by ID for the authenticated user
// @Tags         bookmarks
// @Param        categoryID path int true "Category ID"
// @Success      204 {object} nil
// @Failure      404 {object} models.Envelope{data=nil,error=apperrors.AppError}
// @Failure      500 {object} models.Envelope{data=nil,error=apperrors.AppError}
// @Security     ApiKeyAuth
// @Router       /bookmarks/category/{categoryID} [delete]
func (h *PostEngagementHandler) DeleteBookmarkCategory(w http.ResponseWriter, r *http.Request) {
	user, err := middleware.GetAuthenticatedUserFromContext(r)
	if err != nil {
		h.logger.Error("authentication middleware error", "error", err)
		util.RespondWithAppError(w, apperrors.InternalServerError(err))
		return
	}
	categoryIDStr := r.PathValue("categoryID")
	categoryID, err := strconv.Atoi(categoryIDStr)
	if err != nil {
		h.logger.Warn("invalid category ID parameter", "categoryID", categoryIDStr, "error", err)
		util.RespondWithAppError(w, apperrors.BadRequestError("invalid category ID", err))
		return
	}
	err = h.service.PostEngagements.DeleteBookmarkCategory(r.Context(), user.ID, categoryID)
	if err != nil {
		if appErr, ok := err.(*apperrors.AppError); ok {
			util.RespondWithAppError(w, appErr)
			return
		}
		util.RespondWithAppError(w, apperrors.InternalServerError(err))
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// VotePoll godoc
//
// @Summary      Vote in a poll
// @Description  Casts one vote for the given option on a post's poll. Each user may vote once; voting a second time returns a conflict. Voting on an ended poll is rejected.
// @Tags         posts
// @Accept       json
// @Produce      json
// @Param        postID  path     int            true "Post ID"
// @Param        payload body     object         true "Vote" SchemaExample({"option_id":1})
// @Success      200     {object} models.Poll
// @Failure      400     {object} models.Envelope{data=nil,error=apperrors.AppError}
// @Failure      404     {object} models.Envelope{data=nil,error=apperrors.AppError}
// @Failure      409     {object} models.Envelope{data=nil,error=apperrors.AppError}
// @Failure      500     {object} models.Envelope{data=nil,error=apperrors.AppError}
// @Security     ApiKeyAuth
// @Router       /posts/{postID}/poll/vote [post]
func (h *PostEngagementHandler) VotePoll(w http.ResponseWriter, r *http.Request) {
	user, err := middleware.GetAuthenticatedUserFromContext(r)
	if err != nil {
		util.RespondWithAppError(w, apperrors.InternalServerError(err))
		return
	}
	postID, err := strconv.Atoi(r.PathValue("postID"))
	if err != nil {
		util.RespondWithAppError(w, apperrors.BadRequestError("invalid post ID", err))
		return
	}
	if !h.requirePostVisible(w, r, postID, user.ID) {
		return
	}
	var payload struct {
		OptionID int `json:"option_id"`
	}
	if err := util.ReadJSON(r, &payload); err != nil || payload.OptionID <= 0 {
		util.RespondWithAppError(w, apperrors.BadRequestError("option_id is required", err))
		return
	}
	poll, err := h.service.Posts.VotePoll(r.Context(), postID, payload.OptionID, user.ID)
	if err != nil {
		if appErr, ok := err.(*apperrors.AppError); ok {
			util.RespondWithAppError(w, appErr)
			return
		}
		util.RespondWithAppError(w, apperrors.InternalServerError(err))
		return
	}
	util.RespondWithJson(w, http.StatusOK, poll)
}

// Quote godoc
//
// @Summary      Quote a post
// @Description  Creates a new post quoting the given post ID (with optional content and media)
// @Tags         posts
// @Accept       json
// @Produce      json
// @Param        postID path int true "Quoted Post ID"
// @Param        payload body models.CreatePostPayload true "Quote post payload"
// @Success      201 {object} models.Envelope{data=models.Post}
// @Failure      400 {object} models.Envelope{data=nil,error=apperrors.AppError}
// @Failure      500 {object} models.Envelope{data=nil,error=apperrors.AppError}
// @Security     ApiKeyAuth
// @Router       /posts/{postID}/quote [post]
func (h *PostEngagementHandler) Quote(w http.ResponseWriter, r *http.Request) {
	user, err := middleware.GetAuthenticatedUserFromContext(r)
	if err != nil {
		h.logger.Error("authentication middleware error", "error", err)
		util.RespondWithAppError(w, apperrors.InternalServerError(err))
		return
	}
	quotedPostID, err := strconv.Atoi(r.PathValue("postID"))
	if err != nil {
		h.logger.Warn("invalid quoted post ID parameter", "postID", r.PathValue("postID"), "error", err)
		util.RespondWithAppError(w, apperrors.BadRequestError("invalid quoted post ID", err))
		return
	}
	if !h.requirePostVisible(w, r, quotedPostID, user.ID) {
		return
	}
	var payload models.CreatePostPayload
	if err := util.ReadJSON(r, &payload); err != nil {
		h.logger.Warn("invalid JSON payload in request", "error", err, "userID", user.ID)
		util.RespondWithAppError(w, apperrors.PayloadValidationError(err))
		return
	}
	if err := util.Validate.Struct(payload); err != nil {
		h.logger.Warn("payload validation failed", "error", err, "userID", user.ID)
		util.RespondWithAppError(w, apperrors.PayloadValidationError(err))
		return
	}
	post := &models.Post{
		Content:      payload.Content,
		AuthorID:     user.ID,
		ParentID:     payload.ParentID,
		QuotedPostID: &quotedPostID,
		NewsPayload:  payload.News,
		Visibility:   payload.Visibility,
	}
	if err := h.service.Posts.QuotePost(r.Context(), post, payload.Media); err != nil {
		if appErr, ok := err.(*apperrors.AppError); ok {
			util.RespondWithAppError(w, appErr)
			return
		}
		util.RespondWithAppError(w, apperrors.InternalServerError(err))
		return
	}
	h.invalidateActorFeed(r.Context(), user.ID)
	if err := h.service.Notifications.CreateForPost(r.Context(), user.ID, quotedPostID, "quote"); err != nil {
		h.logger.Warn("failed to create quote notification", "postID", quotedPostID, "userID", user.ID, "error", err)
	}
	if err := h.service.Notifications.CreateMentionNotifications(r.Context(), user.ID, post.ID, post.Content); err != nil {
		h.logger.Warn("failed to create mention notifications", "postID", post.ID, "error", err)
	}
	util.RespondWithJson(w, http.StatusCreated, post)
}
