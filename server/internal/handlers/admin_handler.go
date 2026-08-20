package handlers

import (
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/ba-reynolds/gaggle/internal/apperrors"
	hostmetrics "github.com/ba-reynolds/gaggle/internal/metrics"
	"github.com/ba-reynolds/gaggle/internal/middleware"
	"github.com/ba-reynolds/gaggle/internal/models"
	"github.com/ba-reynolds/gaggle/internal/service"
	"github.com/ba-reynolds/gaggle/internal/util"
)

// AdminHandler handles admin-only operations for the badge system.
type AdminHandler struct {
	service *service.Service
	logger  *slog.Logger
}

func NewAdminHandler(service *service.Service, logger *slog.Logger) *AdminHandler {
	return &AdminHandler{
		service: service,
		logger:  logger,
	}
}

func (h *AdminHandler) respondError(w http.ResponseWriter, err error) {
	if appErr, ok := err.(*apperrors.AppError); ok {
		util.RespondWithAppError(w, appErr)
		return
	}
	util.RespondWithAppError(w, apperrors.InternalServerError(err))
}

// ListBadges godoc
//
// @Summary      List badge catalog
// @Description  Returns every badge definition (earned + assigned). Admin only.
// @Tags         admin
// @Produce      json
// @Success      200 {object} models.Envelope{data=[]models.Badge}
// @Failure      403 {object} models.Envelope{data=nil,error=apperrors.AppError}
// @Failure      500 {object} models.Envelope{data=nil,error=apperrors.AppError}
// @Security     ApiKeyAuth
// @Router       /admin/badges [get]
func (h *AdminHandler) ListBadges(w http.ResponseWriter, r *http.Request) {
	catalog, err := h.service.Badges.ListCatalog(r.Context())
	if err != nil {
		h.respondError(w, err)
		return
	}
	util.RespondWithJson(w, http.StatusOK, catalog)
}

// CreateBadge godoc
//
// @Summary      Create an assigned badge
// @Description  Registers a new admin-assigned badge definition. Admin only.
// @Tags         admin
// @Accept       json
// @Produce      json
// @Param        payload body models.CreateBadgePayload true "Badge definition"
// @Success      201 {object} models.Envelope{data=models.Badge}
// @Failure      400 {object} models.Envelope{data=nil,error=apperrors.AppError}
// @Failure      403 {object} models.Envelope{data=nil,error=apperrors.AppError}
// @Failure      409 {object} models.Envelope{data=nil,error=apperrors.AppError}
// @Failure      500 {object} models.Envelope{data=nil,error=apperrors.AppError}
// @Security     ApiKeyAuth
// @Router       /admin/badges [post]
func (h *AdminHandler) CreateBadge(w http.ResponseWriter, r *http.Request) {
	var payload models.CreateBadgePayload
	if err := util.ReadJSON(r, &payload); err != nil {
		h.respondError(w, apperrors.PayloadValidationError(err))
		return
	}
	badge, err := h.service.Badges.CreateBadge(r.Context(), payload)
	if err != nil {
		h.respondError(w, err)
		return
	}
	util.RespondWithJson(w, http.StatusCreated, badge)
}

// UpdateBadge godoc
//
// @Summary      Update an assigned badge
// @Description  Edits an admin-assigned badge definition. Earned badges are immutable. Admin only.
// @Tags         admin
// @Accept       json
// @Produce      json
// @Param        badgeID path int true "Badge ID"
// @Param        payload body models.CreateBadgePayload true "Badge definition"
// @Success      200 {object} models.Envelope{data=models.Badge}
// @Failure      400 {object} models.Envelope{data=nil,error=apperrors.AppError}
// @Failure      403 {object} models.Envelope{data=nil,error=apperrors.AppError}
// @Failure      404 {object} models.Envelope{data=nil,error=apperrors.AppError}
// @Failure      500 {object} models.Envelope{data=nil,error=apperrors.AppError}
// @Security     ApiKeyAuth
// @Router       /admin/badges/{badgeID} [patch]
func (h *AdminHandler) UpdateBadge(w http.ResponseWriter, r *http.Request) {
	badgeID, err := strconv.Atoi(r.PathValue("badgeID"))
	if err != nil {
		h.respondError(w, apperrors.BadRequestError("invalid badge ID", err))
		return
	}
	var payload models.CreateBadgePayload
	if err := util.ReadJSON(r, &payload); err != nil {
		h.respondError(w, apperrors.PayloadValidationError(err))
		return
	}
	badge, err := h.service.Badges.UpdateBadge(r.Context(), badgeID, payload)
	if err != nil {
		h.respondError(w, err)
		return
	}
	util.RespondWithJson(w, http.StatusOK, badge)
}

// DeleteBadge godoc
//
// @Summary      Delete an assigned badge
// @Description  Removes an admin-assigned badge definition not currently held by any user. Admin only.
// @Tags         admin
// @Produce      json
// @Param        badgeID path int true "Badge ID"
// @Success      200 {object} models.Envelope{data=map[string]bool}
// @Failure      400 {object} models.Envelope{data=nil,error=apperrors.AppError}
// @Failure      403 {object} models.Envelope{data=nil,error=apperrors.AppError}
// @Failure      404 {object} models.Envelope{data=nil,error=apperrors.AppError}
// @Failure      409 {object} models.Envelope{data=nil,error=apperrors.AppError}
// @Failure      500 {object} models.Envelope{data=nil,error=apperrors.AppError}
// @Security     ApiKeyAuth
// @Router       /admin/badges/{badgeID} [delete]
func (h *AdminHandler) DeleteBadge(w http.ResponseWriter, r *http.Request) {
	badgeID, err := strconv.Atoi(r.PathValue("badgeID"))
	if err != nil {
		h.respondError(w, apperrors.BadRequestError("invalid badge ID", err))
		return
	}
	if err := h.service.Badges.DeleteBadge(r.Context(), badgeID); err != nil {
		h.respondError(w, err)
		return
	}
	util.RespondWithJson(w, http.StatusOK, map[string]bool{"success": true})
}

// GrantBadge godoc
//
// @Summary      Grant a badge to a user
// @Description  Awards an admin-assigned badge to a user. Admin only.
// @Tags         admin
// @Produce      json
// @Param        username path string true "Username"
// @Param        badgeID  path int    true "Badge ID"
// @Success      200 {object} models.Envelope{data=map[string]bool}
// @Failure      400 {object} models.Envelope{data=nil,error=apperrors.AppError}
// @Failure      403 {object} models.Envelope{data=nil,error=apperrors.AppError}
// @Failure      404 {object} models.Envelope{data=nil,error=apperrors.AppError}
// @Failure      409 {object} models.Envelope{data=nil,error=apperrors.AppError}
// @Failure      500 {object} models.Envelope{data=nil,error=apperrors.AppError}
// @Security     ApiKeyAuth
// @Router       /admin/users/{username}/badges/{badgeID} [post]
func (h *AdminHandler) GrantBadge(w http.ResponseWriter, r *http.Request) {
	user, err := middleware.GetAuthenticatedUserFromContext(r)
	if err != nil {
		h.respondError(w, apperrors.InternalServerError(err))
		return
	}
	badgeID, err := strconv.Atoi(r.PathValue("badgeID"))
	if err != nil {
		h.respondError(w, apperrors.BadRequestError("invalid badge ID", err))
		return
	}
	if err := h.service.Badges.GrantBadge(r.Context(), r.PathValue("username"), badgeID, user.ID); err != nil {
		h.respondError(w, err)
		return
	}
	util.RespondWithJson(w, http.StatusOK, map[string]bool{"success": true})
}

// RevokeBadge godoc
//
// @Summary      Revoke a badge from a user
// @Description  Removes an admin-assigned badge from a user. Admin only.
// @Tags         admin
// @Produce      json
// @Param        username path string true "Username"
// @Param        badgeID  path int    true "Badge ID"
// @Success      200 {object} models.Envelope{data=map[string]bool}
// @Failure      403 {object} models.Envelope{data=nil,error=apperrors.AppError}
// @Failure      404 {object} models.Envelope{data=nil,error=apperrors.AppError}
// @Failure      500 {object} models.Envelope{data=nil,error=apperrors.AppError}
// @Security     ApiKeyAuth
// @Router       /admin/users/{username}/badges/{badgeID} [delete]
func (h *AdminHandler) RevokeBadge(w http.ResponseWriter, r *http.Request) {
	badgeID, err := strconv.Atoi(r.PathValue("badgeID"))
	if err != nil {
		h.respondError(w, apperrors.BadRequestError("invalid badge ID", err))
		return
	}
	if err := h.service.Badges.RevokeBadge(r.Context(), r.PathValue("username"), badgeID); err != nil {
		h.respondError(w, err)
		return
	}
	util.RespondWithJson(w, http.StatusOK, map[string]bool{"success": true})
}

// Metrics godoc
//
// @Summary      Admin metrics dashboard
// @Description  Live host stats (CPU, memory, load, uptime, disk), platform
// @Description  counters, active users, and page-view traffic. Admin only.
// @Tags         admin
// @Produce      json
// @Success      200 {object} models.Envelope{data=models.AdminMetrics}
// @Failure      403 {object} models.Envelope{data=nil,error=apperrors.AppError}
// @Failure      500 {object} models.Envelope{data=nil,error=apperrors.AppError}
// @Security     ApiKeyAuth
// @Router       /admin/metrics [get]
func (h *AdminHandler) Metrics(w http.ResponseWriter, r *http.Request) {
	host, err := hostmetrics.ReadHostStats()
	if err != nil {
		h.respondError(w, apperrors.InternalServerError(err))
		return
	}

	app, err := h.service.Metrics.AppStats(r.Context())
	if err != nil {
		h.respondError(w, err)
		return
	}

	now := time.Now()
	dau, err := h.service.Metrics.DistinctUsersActiveSince(r.Context(), now.Add(-24*time.Hour))
	if err != nil {
		h.respondError(w, err)
		return
	}
	wau, err := h.service.Metrics.DistinctUsersActiveSince(r.Context(), now.Add(-7*24*time.Hour))
	if err != nil {
		h.respondError(w, err)
		return
	}

	lastMinute, err := h.service.Metrics.RequestsLastMinute(r.Context())
	if err != nil {
		h.respondError(w, err)
		return
	}

	byDay, err := h.service.Metrics.ViewsByDay(r.Context(), 14)
	if err != nil {
		h.respondError(w, err)
		return
	}

	snapshot := &models.AdminMetrics{
		Host:   *host,
		App:    *app,
		Active: models.ActiveUsers{DAU: dau, WAU: wau},
		Views: models.ViewStats{
			RequestsPerMinute: float64(lastMinute),
			ByDay:             byDay,
		},
	}
	util.RespondWithJson(w, http.StatusOK, snapshot)
}
