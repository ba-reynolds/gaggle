package handlers

import (
	"context"
	"encoding/json"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"strings"

	"github.com/ba-reynolds/gophersocial/internal/apperrors"
	"github.com/ba-reynolds/gophersocial/internal/cache"
	"github.com/ba-reynolds/gophersocial/internal/middleware"
	"github.com/ba-reynolds/gophersocial/internal/models"
	"github.com/ba-reynolds/gophersocial/internal/service"
	"github.com/ba-reynolds/gophersocial/internal/util"
)

// PostHandler handles HTTP requests for post operations
type PostHandler struct {
	service *service.Service
	logger  *slog.Logger
	rdb     *cache.Client
}

func NewPostHandler(service *service.Service, logger *slog.Logger, rdb *cache.Client) *PostHandler {
	return &PostHandler{
		service: service,
		logger:  logger,
		rdb:     rdb,
	}
}

// invalidateFeedForUserAndFollowers clears the cached home feed for a user and
// everyone following them (their home feeds show the user's posts).
func (h *PostHandler) invalidateFeedForUserAndFollowers(ctx context.Context, userID int) {
	if h.rdb == nil {
		return
	}
	h.rdb.InvalidateHomeFeed(ctx, userID)
	if ids, err := h.service.UserRelationships.GetFollowerIDs(ctx, userID); err == nil {
		for _, id := range ids {
			h.rdb.InvalidateHomeFeed(ctx, id)
		}
	}
}

// invalidateFeedForUser clears the cached home feed for a single user (their
// own engagement changes alter their home feed).
func (h *PostHandler) invalidateFeedForUser(ctx context.Context, userID int) {
	if h.rdb == nil {
		return
	}
	h.rdb.InvalidateHomeFeed(ctx, userID)
}

// stripPort removes the port from a host:port string for storage in an INET
// column. Falls back to the raw value if it has no port.
func stripPort(addr string) string {
	if host, _, err := net.SplitHostPort(addr); err == nil {
		return host
	}
	return addr
}

// CreatePost godoc
//
//	@Summary		Create a new post
//	@Description	Creates a new post with content and optional media
//	@Tags			posts
//	@Accept			json
//	@Produce		json
//	@Param			payload	body		models.CreatePostPayload	true	"Post creation information"
//	@Success		201		{object}	models.Envelope{data=models.FullPost,error=nil}
//	@Failure		400		{object}	models.Envelope{data=nil,error=apperrors.AppError}
//	@Failure		500		{object}	models.Envelope{data=nil,error=apperrors.AppError}
//	@Security		ApiKeyAuth
//	@Router			/posts [post]
func (h *PostHandler) CreatePost(w http.ResponseWriter, r *http.Request) {
	user, err := middleware.GetAuthenticatedUserFromContext(r)
	if err != nil {
		h.logger.Error("authentication middleware error",
			"error", err,
			"path", r.URL.Path,
			"method", r.Method,
		)
		util.RespondWithAppError(w, apperrors.InternalServerError(err))
		return
	}

	var payload models.CreatePostPayload
	if err := util.ReadJSON(r, &payload); err != nil {
		h.logger.Warn("invalid JSON payload in request",
			"error", err,
			"userID", user.ID,
			"content_type", r.Header.Get("Content-Type"),
		)
		util.RespondWithAppError(w, apperrors.PayloadValidationError(err))
		return
	}

	if err := util.Validate.Struct(payload); err != nil {
		h.logger.Warn("payload validation failed",
			"error", err,
			"userID", user.ID,
		)
		util.RespondWithAppError(w, apperrors.PayloadValidationError(err))
		return
	}

	h.logger.Debug("create post request",
		"userID", user.ID,
		"parentID", payload.ParentID,
		"mediaCount", len(payload.Media),
	)

	post := &models.Post{
		Content:     payload.Content,
		AuthorID:    user.ID,
		ParentID:    payload.ParentID,
		PollPayload: payload.Poll,
	}

	if err := h.service.Posts.Create(r.Context(), post, payload.Media); err != nil {
		if appErr, ok := err.(*apperrors.AppError); ok {
			util.RespondWithAppError(w, appErr)
			return
		}
		util.RespondWithAppError(w, apperrors.InternalServerError(err))
		return
	}

	fullPost, err := h.service.Posts.GetFullPostByID(r.Context(), post.ID, user.ID)
	if err != nil {
		if appErr, ok := err.(*apperrors.AppError); ok {
			util.RespondWithAppError(w, appErr)
			return
		}
		util.RespondWithAppError(w, apperrors.InternalServerError(err))
		return
	}

	// A new post changes the home feed of the author and all their followers.
	if h.rdb != nil {
		h.invalidateFeedForUserAndFollowers(r.Context(), user.ID)
	}
	if post.ParentID != nil {
		if err := h.service.Notifications.CreateForPost(r.Context(), user.ID, *post.ParentID, "reply"); err != nil {
			h.logger.Warn("failed to create reply notification", "postID", post.ID, "parentID", *post.ParentID, "error", err)
		}
	}
	if err := h.service.Notifications.CreateMentionNotifications(r.Context(), user.ID, post.ID, post.Content); err != nil {
		h.logger.Warn("failed to create mention notifications", "postID", post.ID, "error", err)
	}
	if err := h.service.Notifications.PublishFeedPost(r.Context(), user.ID, post.ID); err != nil {
		h.logger.Warn("failed to publish feed post event", "postID", post.ID, "userID", user.ID, "error", err)
	}

	if err := util.RespondWithJson(w, http.StatusCreated, fullPost); err != nil {
		h.logger.Error("failed to write HTTP response",
			"error", err,
			"userID", user.ID,
			"postID", post.ID,
			"status", http.StatusCreated,
		)
		util.RespondWithAppError(w, apperrors.InternalServerError(err))
		return
	}
}

// GetPostByID godoc
//
//	@Summary		Get post details
//	@Description	Retrieves a post's details including its author information and optional ancestor/descendant chains
//	@Tags			posts
//	@Accept			json
//	@Produce		json
//	@Param			postID		path		int		true	"Post ID"
//	@Param			ancestors	query		bool	false	"Include ancestor chain (default: false)"
//	@Param			descendants	query		bool	false	"Include descendant chain (default: false)"
//	@Param			limit		query		int		false	"Maximum number of ancestors/descendants to retrieve (uses default if invalid or missing)"
//	@Param			ancestor_cursor	query		string	false	"Cursor for ancestor pagination"
//	@Param			descendant_cursor	query	string	false	"Cursor for descendant pagination"
//	@Success		200			{object}	models.Envelope{data=models.PostWithAncestorsAndDescendants,error=nil}
//	@Failure		400			{object}	models.Envelope{data=nil,error=apperrors.AppError}
//	@Failure		404			{object}	models.Envelope{data=nil,error=apperrors.AppError}
//	@Failure		500			{object}	models.Envelope{data=nil,error=apperrors.AppError}
//	@Security		ApiKeyAuth
//	@Router			/posts/{postID} [get]
func (h *PostHandler) GetPostByID(w http.ResponseWriter, r *http.Request) {
	postIDString := r.PathValue("postID")
	postID, err := strconv.Atoi(postIDString)
	if err != nil {
		h.logger.Warn("invalid post ID parameter",
			"postID", postIDString,
			"error", err,
		)
		util.RespondWithAppError(w, apperrors.BadRequestError("invalid post ID", err))
		return
	}

	user, err := middleware.GetAuthenticatedUserFromContext(r)
	if err != nil {
		h.logger.Error("authentication middleware error",
			"error", err,
			"path", r.URL.Path,
			"method", r.Method,
		)
		util.RespondWithAppError(w, apperrors.InternalServerError(err))
		return
	}

	includeAncestors := r.URL.Query().Get("ancestors") == "true"
	includeDescendants := r.URL.Query().Get("descendants") == "true"

	limitStr := r.URL.Query().Get("limit")
	limit := 0
	if limitStr != "" {
		parsedLimit, err := strconv.Atoi(limitStr)
		if err != nil {
			limit = 0
		} else {
			limit = parsedLimit
		}
	}

	ancestorCursor := r.URL.Query().Get("ancestor_cursor")
	descendantCursor := r.URL.Query().Get("descendant_cursor")

	h.logger.Debug("get post request",
		"postID", postID,
		"includeAncestors", includeAncestors,
		"includeDescendants", includeDescendants,
		"limit", limit,
	)

	// Record a view (fire and forget; failure must not fail the request)
	viewerID := user.ID
	_ = h.service.PostEngagements.AddView(r.Context(), postID, &viewerID, stripPort(r.RemoteAddr), r.UserAgent())

	post, err := h.service.Posts.GetFullPostByID(r.Context(), postID, user.ID)
	if err != nil {
		if appErr, ok := err.(*apperrors.AppError); ok {
			util.RespondWithAppError(w, appErr)
			return
		}
		util.RespondWithAppError(w, apperrors.InternalServerError(err))
		return
	}

	var ancestors *models.PostChain
	if includeAncestors {
		ancestors, err = h.service.Posts.GetParentChain(r.Context(), postID, user.ID, limit, ancestorCursor)
		if err != nil {
			if appErr, ok := err.(*apperrors.AppError); ok {
				util.RespondWithAppError(w, appErr)
				return
			}
			util.RespondWithAppError(w, apperrors.InternalServerError(err))
			return
		}
	}

	var descendants *models.PostDescendants
	if includeDescendants {
		descendants, err = h.service.Posts.GetDescendants(r.Context(), postID, user.ID, limit, descendantCursor)
		if err != nil {
			if appErr, ok := err.(*apperrors.AppError); ok {
				util.RespondWithAppError(w, appErr)
				return
			}
			util.RespondWithAppError(w, apperrors.InternalServerError(err))
			return
		}
	}

	response := &models.PostWithAncestorsAndDescendants{
		Post:        post,
		Ancestors:   ancestors,
		Descendants: descendants,
	}

	if err := util.RespondWithJson(w, http.StatusOK, response); err != nil {
		h.logger.Error("failed to write HTTP response",
			"error", err,
			"postID", postID,
			"status", http.StatusOK,
		)
		util.RespondWithAppError(w, apperrors.InternalServerError(err))
		return
	}
}

// DeletePostByID godoc
//
//	@Summary		Delete a post
//	@Description	Deletes an existing post by its ID
//	@Tags			posts
//	@Accept			json
//	@Produce		json
//	@Param			postID	path		int	true	"Post ID"
//	@Success		200		{object}	models.Envelope{data=nil,error=nil}
//	@Failure		404		{object}	models.Envelope{data=nil,error=apperrors.AppError}
//	@Failure		500		{object}	models.Envelope{data=nil,error=apperrors.AppError}
//	@Security		ApiKeyAuth
//	@Router			/posts/{postID} [delete]
func (h *PostHandler) DeletePostByID(w http.ResponseWriter, r *http.Request) {
	post := middleware.GetPostFromContext(r)
	user, err := middleware.GetAuthenticatedUserFromContext(r)
	if err != nil {
		h.logger.Error("authentication middleware error",
			"error", err,
			"path", r.URL.Path,
			"method", r.Method,
		)
		util.RespondWithAppError(w, apperrors.InternalServerError(err))
		return
	}

	h.logger.Debug("delete post request",
		"postID", post.ID,
		"userID", user.ID,
	)

	if err := h.service.Posts.DeleteByID(r.Context(), post, user.ID); err != nil {
		if appErr, ok := err.(*apperrors.AppError); ok {
			util.RespondWithAppError(w, appErr)
			return
		}
		util.RespondWithAppError(w, apperrors.InternalServerError(err))
		return
	}

	if err := util.RespondWithJson(w, http.StatusOK, nil); err != nil {
		h.logger.Error("failed to write HTTP response",
			"error", err,
			"postID", post.ID,
			"userID", user.ID,
			"status", http.StatusOK,
		)
		util.RespondWithAppError(w, apperrors.InternalServerError(err))
		return
	}
}

// UpdatePost godoc
//
// @Summary      Update a post
// @Description  Updates the author's own post content, recording the previous content in the edit history.
// @Tags         posts
// @Accept       json
// @Produce      json
// @Param        postID  path             int                     true  "Post ID"
// @Param        payload body             models.UpdatePostPayload true "Post content"
// @Success      200     {object}         models.FullPost
// @Failure      400     {object}         models.Envelope{data=nil,error=apperrors.AppError}
// @Failure      403     {object}         models.Envelope{data=nil,error=apperrors.AppError}
// @Failure      404     {object}         models.Envelope{data=nil,error=apperrors.AppError}
// @Failure      500     {object}         models.Envelope{data=nil,error=apperrors.AppError}
// @Security     ApiKeyAuth
// @Router       /posts/{postID} [patch]
func (h *PostHandler) UpdatePost(w http.ResponseWriter, r *http.Request) {
	post := middleware.GetPostFromContext(r)
	user, err := middleware.GetAuthenticatedUserFromContext(r)
	if err != nil {
		util.RespondWithAppError(w, apperrors.InternalServerError(err))
		return
	}
	var payload models.UpdatePostPayload
	if err := util.ReadJSON(r, &payload); err != nil {
		util.RespondWithAppError(w, apperrors.PayloadValidationError(err))
		return
	}
	updated, err := h.service.Posts.Update(r.Context(), post, user.ID, payload.Content)
	if err != nil {
		h.respondError(w, err)
		return
	}
	full, err := h.service.Posts.GetFullPostByID(r.Context(), updated.ID, user.ID)
	if err != nil {
		h.respondError(w, err)
		return
	}
	util.RespondWithJson(w, http.StatusOK, full)
}

// ListPostEdits godoc
//
// @Summary      List post edit history
// @Description  Returns the recorded edit history (previous contents) for a post.
// @Tags         posts
// @Produce      json
// @Param        postID path int true "Post ID"
// @Success      200    {object} models.PostEditHistory
// @Failure      404    {object} models.Envelope{data=nil,error=apperrors.AppError}
// @Failure      500    {object} models.Envelope{data=nil,error=apperrors.AppError}
// @Security     ApiKeyAuth
// @Router       /posts/{postID}/edits [get]
func (h *PostHandler) ListPostEdits(w http.ResponseWriter, r *http.Request) {
	post := middleware.GetPostFromContext(r)
	history, err := h.service.Posts.ListEdits(r.Context(), post.ID)
	if err != nil {
		h.respondError(w, err)
		return
	}
	util.RespondWithJson(w, http.StatusOK, history)
}

// PinPost godoc
//
// @Summary      Pin a post
// @Description  Pins the author's own post to their profile. The author's previously pinned post (if any) is unpinned.
// @Tags         posts
// @Produce      json
// @Param        postID path int true "Post ID"
// @Success      200    {object} models.Envelope{data=map[string]bool}
// @Failure      403    {object} models.Envelope{data=nil,error=apperrors.AppError}
// @Failure      404    {object} models.Envelope{data=nil,error=apperrors.AppError}
// @Failure      500    {object} models.Envelope{data=nil,error=apperrors.AppError}
// @Security     ApiKeyAuth
// @Router       /posts/{postID}/pin [post]
func (h *PostHandler) PinPost(w http.ResponseWriter, r *http.Request) {
	post := middleware.GetPostFromContext(r)
	user, err := middleware.GetAuthenticatedUserFromContext(r)
	if err != nil {
		util.RespondWithAppError(w, apperrors.InternalServerError(err))
		return
	}
	if err := h.service.Posts.Pin(r.Context(), post, user.ID); err != nil {
		h.respondError(w, err)
		return
	}
	util.RespondWithJson(w, http.StatusOK, map[string]bool{"success": true})
}

// UnpinPost godoc
//
// @Summary      Unpin a post
// @Description  Unpins the author's own post from their profile.
// @Tags         posts
// @Produce      json
// @Param        postID path int true "Post ID"
// @Success      200    {object} models.Envelope{data=map[string]bool}
// @Failure      403    {object} models.Envelope{data=nil,error=apperrors.AppError}
// @Failure      404    {object} models.Envelope{data=nil,error=apperrors.AppError}
// @Failure      500    {object} models.Envelope{data=nil,error=apperrors.AppError}
// @Security     ApiKeyAuth
// @Router       /posts/{postID}/pin [delete]
func (h *PostHandler) UnpinPost(w http.ResponseWriter, r *http.Request) {
	post := middleware.GetPostFromContext(r)
	user, err := middleware.GetAuthenticatedUserFromContext(r)
	if err != nil {
		util.RespondWithAppError(w, apperrors.InternalServerError(err))
		return
	}
	if err := h.service.Posts.Unpin(r.Context(), post, user.ID); err != nil {
		h.respondError(w, err)
		return
	}
	util.RespondWithJson(w, http.StatusOK, map[string]bool{"success": true})
}

// GetPinned godoc
//
// @Summary      Get a user's pinned post
// @Description  Returns the full post currently pinned to the given user's profile, or 404 if none.
// @Tags         users
// @Produce      json
// @Param        username path string true "Username"
// @Success      200      {object} models.FullPost
// @Failure      404      {object} models.Envelope{data=nil,error=apperrors.AppError}
// @Failure      500      {object} models.Envelope{data=nil,error=apperrors.AppError}
// @Security     ApiKeyAuth
// @Router       /users/{username}/pinned [get]
func (h *PostHandler) GetPinned(w http.ResponseWriter, r *http.Request) {
	viewer, err := middleware.GetAuthenticatedUserFromContext(r)
	if err != nil {
		util.RespondWithAppError(w, apperrors.InternalServerError(err))
		return
	}
	profile, err := h.service.Users.GetUserProfileByUsername(r.Context(), r.PathValue("username"))
	if err != nil {
		h.respondError(w, err)
		return
	}
	post, err := h.service.Posts.GetPinned(r.Context(), profile.ID, viewer.ID)
	if err != nil {
		h.respondError(w, err)
		return
	}
	util.RespondWithJson(w, http.StatusOK, post)
}

func (h *PostHandler) respondError(w http.ResponseWriter, err error) {
	if appErr, ok := err.(*apperrors.AppError); ok {
		util.RespondWithAppError(w, appErr)
		return
	}
	util.RespondWithAppError(w, apperrors.InternalServerError(err))
}

// GetHomeFeed godoc
//
//	@Summary		Get home feed
//	@Description	Retrieves a paginated feed of posts from users that the authenticated user follows
//	@Tags			posts
//	@Accept			json
//	@Produce		json
//	@Param			limit	query		int		false	"Maximum number of posts to retrieve (uses default if invalid or missing)"
//	@Param			cursor	query		string	false	"Cursor for pagination"
//	@Success		200		{object}	models.Envelope{data=models.PostFeed,error=nil}
//	@Failure		400		{object}	models.Envelope{data=nil,error=apperrors.AppError}
//	@Failure		500		{object}	models.Envelope{data=nil,error=apperrors.AppError}
//	@Security		ApiKeyAuth
//	@Router			/posts/feed [get]
func (h *PostHandler) GetHomeFeed(w http.ResponseWriter, r *http.Request) {
	user, err := middleware.GetAuthenticatedUserFromContext(r)
	if err != nil {
		h.logger.Error("authentication middleware error",
			"error", err,
			"path", r.URL.Path,
			"method", r.Method,
		)
		util.RespondWithAppError(w, apperrors.InternalServerError(err))
		return
	}

	limitStr := r.URL.Query().Get("limit")
	cursor := r.URL.Query().Get("cursor")

	limit := 0
	if limitStr != "" {
		parsedLimit, err := strconv.Atoi(limitStr)
		if err != nil {
			limit = 0
		} else {
			limit = parsedLimit
		}
	}

	h.logger.Debug("get home feed request",
		"userID", user.ID,
		"limit", limit,
		"cursor", cursor,
	)

	// Serve from cache when possible.
	if h.rdb != nil {
		if cached, ok := h.rdb.GetHomeFeed(r.Context(), user.ID, cursor); ok {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write(cached)
			return
		}
	}

	feed, err := h.service.Posts.GetHomeFeed(r.Context(), user.ID, limit, cursor)
	if err != nil {
		if appErr, ok := err.(*apperrors.AppError); ok {
			util.RespondWithAppError(w, appErr)
			return
		}
		util.RespondWithAppError(w, apperrors.InternalServerError(err))
		return
	}

	envelope := models.Envelope{Data: feed, Error: nil}
	if h.rdb != nil {
		if data, err := json.Marshal(envelope); err == nil {
			h.rdb.SetHomeFeed(r.Context(), user.ID, cursor, data)
		}
	}

	if err := util.RespondWithJson(w, http.StatusOK, feed); err != nil {
		h.logger.Error("failed to write HTTP response",
			"error", err,
			"userID", user.ID,
			"status", http.StatusOK,
		)
		util.RespondWithAppError(w, apperrors.InternalServerError(err))
		return
	}
}

// GetUserFeed godoc
//
//	@Summary		Get user feed
//	@Description	Retrieves a paginated feed of posts made by a specific user
//	@Tags			posts
//	@Accept			json
//	@Produce		json
//	@Param			username		path		string	true	"Username of the user"
//	@Param			include_replies	query		bool	false	"Include replies in the feed (default: false)"
//	@Param			limit			query		int		false	"Maximum number of posts to retrieve (uses default if invalid or missing)"
//	@Param			cursor			query		string	false	"Cursor for pagination"
//	@Success		200				{object}	models.Envelope{data=models.PostFeed,error=nil}
//	@Failure		400				{object}	models.Envelope{data=nil,error=apperrors.AppError}
//	@Failure		404				{object}	models.Envelope{data=nil,error=apperrors.AppError}
//	@Failure		500				{object}	models.Envelope{data=nil,error=apperrors.AppError}
//	@Security		ApiKeyAuth
//	@Router			/users/{username}/posts [get]
func (h *PostHandler) GetUserFeed(w http.ResponseWriter, r *http.Request) {
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

	user, err := h.service.Users.GetByUsername(r.Context(), username)
	if err != nil {
		if appErr, ok := err.(*apperrors.AppError); ok {
			util.RespondWithAppError(w, appErr)
			return
		}
		util.RespondWithAppError(w, apperrors.InternalServerError(err))
		return
	}

	includeReplies := r.URL.Query().Get("include_replies") == "true"
	limitStr := r.URL.Query().Get("limit")
	cursor := r.URL.Query().Get("cursor")

	limit := 0
	if limitStr != "" {
		parsedLimit, err := strconv.Atoi(limitStr)
		if err != nil {
			limit = 0
		} else {
			limit = parsedLimit
		}
	}

	h.logger.Debug("get user feed request",
		"username", username,
		"userID", user.ID,
		"viewerID", viewer.ID,
		"includeReplies", includeReplies,
		"limit", limit,
		"cursor", cursor,
	)

	feed, err := h.service.Posts.GetUserFeed(r.Context(), user.ID, viewer.ID, includeReplies, limit, cursor)
	if err != nil {
		if appErr, ok := err.(*apperrors.AppError); ok {
			util.RespondWithAppError(w, appErr)
			return
		}
		util.RespondWithAppError(w, apperrors.InternalServerError(err))
		return
	}

	if err := util.RespondWithJson(w, http.StatusOK, feed); err != nil {
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

// BookmarkedPostsFeed godoc
//
// @Summary      Get bookmarked posts feed
// @Description  Returns a paginated feed of posts bookmarked by the authenticated user, optionally filtered by category IDs
// @Tags         posts
// @Accept       json
// @Produce      json
// @Param        category_ids query string false "Comma-separated list of bookmark category IDs"
// @Param        limit query int false "Maximum number of posts to retrieve"
// @Param        cursor query string false "Cursor for pagination"
// @Success      200 {object} models.PostFeed
// @Failure      400 {object} models.Envelope{data=nil,error=apperrors.AppError}
// @Failure      500 {object} models.Envelope{data=nil,error=apperrors.AppError}
// @Security     ApiKeyAuth
// @Router       /posts/bookmarks [get]
func (h *PostHandler) BookmarkedPostsFeed(w http.ResponseWriter, r *http.Request) {
	user, err := middleware.GetAuthenticatedUserFromContext(r)
	if err != nil {
		h.logger.Error("authentication middleware error", "error", err)
		util.RespondWithAppError(w, apperrors.InternalServerError(err))
		return
	}
	categoryIDs := []int{}
	if idsStr := r.URL.Query().Get("category_ids"); idsStr != "" {
		for _, idStr := range strings.Split(idsStr, ",") {
			idStr = strings.TrimSpace(idStr)
			if idStr == "" {
				continue
			}
			id, err := strconv.Atoi(idStr)
			if err == nil {
				categoryIDs = append(categoryIDs, id)
			}
		}
	}
	limitStr := r.URL.Query().Get("limit")
	cursor := r.URL.Query().Get("cursor")
	limit, _, _ := util.ParsePaginationParams(limitStr, cursor, 20, 100)
	feed, err := h.service.Posts.GetBookmarkedPostsFeed(r.Context(), user.ID, user.ID, categoryIDs, limit, cursor)
	if err != nil {
		if appErr, ok := err.(*apperrors.AppError); ok {
			util.RespondWithAppError(w, appErr)
			return
		}
		util.RespondWithAppError(w, apperrors.InternalServerError(err))
		return
	}
	util.RespondWithJson(w, http.StatusOK, feed)
}

// LikedPostsFeed godoc
//
// @Summary      Get posts liked by a user
// @Description  Returns a paginated feed of posts liked by the specified user
// @Tags         posts
// @Accept       json
// @Produce      json
// @Param        username path string true "Username"
// @Param        limit query int false "Maximum number of posts to retrieve"
// @Param        cursor query string false "Cursor for pagination"
// @Success      200 {object} models.PostFeed
// @Failure      400 {object} models.Envelope{data=nil,error=apperrors.AppError}
// @Failure      404 {object} models.Envelope{data=nil,error=apperrors.AppError}
// @Failure      500 {object} models.Envelope{data=nil,error=apperrors.AppError}
// @Security     ApiKeyAuth
// @Router       /users/{username}/likes [get]
func (h *PostHandler) LikedPostsFeed(w http.ResponseWriter, r *http.Request) {
	username := r.PathValue("username")
	if username == "" {
		util.RespondWithAppError(w, apperrors.BadRequestError("username is required", nil))
		return
	}

	viewer, err := middleware.GetAuthenticatedUserFromContext(r)
	if err != nil {
		h.logger.Error("authentication middleware error", "error", err)
		util.RespondWithAppError(w, apperrors.InternalServerError(err))
		return
	}

	user, err := h.service.Users.GetByUsername(r.Context(), username)
	if err != nil {
		if appErr, ok := err.(*apperrors.AppError); ok {
			util.RespondWithAppError(w, appErr)
			return
		}
		util.RespondWithAppError(w, apperrors.InternalServerError(err))
		return
	}

	limitStr := r.URL.Query().Get("limit")
	cursor := r.URL.Query().Get("cursor")
	limit, _, _ := util.ParsePaginationParams(limitStr, cursor, 20, 100)
	feed, err := h.service.Posts.GetLikedPostsFeed(r.Context(), user.ID, viewer.ID, limit, cursor)
	if err != nil {
		if appErr, ok := err.(*apperrors.AppError); ok {
			util.RespondWithAppError(w, appErr)
			return
		}
		util.RespondWithAppError(w, apperrors.InternalServerError(err))
		return
	}
	h.logger.Debug("liked posts feed request", "username", username, "userID", user.ID, "viewerID", viewer.ID, "limit", limit, "cursor", cursor)
	util.RespondWithJson(w, http.StatusOK, feed)
}

// GetPostQuotesFeed godoc
//
// @Summary      Get quotes of a post
// @Description  Returns a paginated feed of posts that quote the given post
// @Tags         posts
// @Accept       json
// @Produce      json
// @Param        postID path int true "Post ID"
// @Param        limit query int false "Maximum number of posts to retrieve"
// @Param        cursor query string false "Cursor for pagination"
// @Success      200 {object} models.Envelope{data=models.PostFeed,error=nil}
// @Failure      404 {object} models.Envelope{data=nil,error=apperrors.AppError}
// @Failure      500 {object} models.Envelope{data=nil,error=apperrors.AppError}
// @Security     ApiKeyAuth
// @Router       /posts/{postID}/quotes [get]
func (h *PostHandler) GetPostQuotesFeed(w http.ResponseWriter, r *http.Request) {
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
	limitStr := r.URL.Query().Get("limit")
	cursor := r.URL.Query().Get("cursor")
	limit, _, _ := util.ParsePaginationParams(limitStr, cursor, 20, 100)
	feed, err := h.service.Posts.GetQuotesFeed(r.Context(), postID, user.ID, limit, cursor)
	if err != nil {
		if appErr, ok := err.(*apperrors.AppError); ok {
			util.RespondWithAppError(w, appErr)
			return
		}
		util.RespondWithAppError(w, apperrors.InternalServerError(err))
		return
	}
	util.RespondWithJson(w, http.StatusOK, feed)
}

// GetPostLikers godoc
//
// @Summary      Get users who liked a post
// @Description  Returns a paginated list of users who liked the given post
// @Tags         posts
// @Accept       json
// @Produce      json
// @Param        postID path int true "Post ID"
// @Param        limit query int false "Maximum number of users to retrieve"
// @Param        cursor query string false "Cursor for pagination"
// @Success      200 {object} models.Envelope{data=models.UserList,error=nil}
// @Failure      404 {object} models.Envelope{data=nil,error=apperrors.AppError}
// @Failure      500 {object} models.Envelope{data=nil,error=apperrors.AppError}
// @Security     ApiKeyAuth
// @Router       /posts/{postID}/likers [get]
func (h *PostHandler) GetPostLikers(w http.ResponseWriter, r *http.Request) {
	_, err := middleware.GetAuthenticatedUserFromContext(r)
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
	limitStr := r.URL.Query().Get("limit")
	cursor := r.URL.Query().Get("cursor")
	limit, _, _ := util.ParsePaginationParams(limitStr, cursor, 20, 100)
	list, err := h.service.Posts.GetPostLikers(r.Context(), postID, limit, cursor)
	if err != nil {
		if appErr, ok := err.(*apperrors.AppError); ok {
			util.RespondWithAppError(w, appErr)
			return
		}
		util.RespondWithAppError(w, apperrors.InternalServerError(err))
		return
	}
	util.RespondWithJson(w, http.StatusOK, list)
}

// GetPostReposters godoc
//
// @Summary      Get users who reposted a post
// @Description  Returns a paginated list of users who reposted the given post
// @Tags         posts
// @Accept       json
// @Produce      json
// @Param        postID path int true "Post ID"
// @Param        limit query int false "Maximum number of users to retrieve"
// @Param        cursor query string false "Cursor for pagination"
// @Success      200 {object} models.Envelope{data=models.UserList,error=nil}
// @Failure      404 {object} models.Envelope{data=nil,error=apperrors.AppError}
// @Failure      500 {object} models.Envelope{data=nil,error=apperrors.AppError}
// @Security     ApiKeyAuth
// @Router       /posts/{postID}/reposters [get]
func (h *PostHandler) GetPostReposters(w http.ResponseWriter, r *http.Request) {
	_, err := middleware.GetAuthenticatedUserFromContext(r)
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
	limitStr := r.URL.Query().Get("limit")
	cursor := r.URL.Query().Get("cursor")
	limit, _, _ := util.ParsePaginationParams(limitStr, cursor, 20, 100)
	list, err := h.service.Posts.GetPostReposters(r.Context(), postID, limit, cursor)
	if err != nil {
		if appErr, ok := err.(*apperrors.AppError); ok {
			util.RespondWithAppError(w, appErr)
			return
		}
		util.RespondWithAppError(w, apperrors.InternalServerError(err))
		return
	}
	util.RespondWithJson(w, http.StatusOK, list)
}
