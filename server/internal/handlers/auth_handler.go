package handlers

import (
	"log/slog"
	"net/http"

	"github.com/ba-reynolds/gophersocial/internal/apperrors"
	"github.com/ba-reynolds/gophersocial/internal/models"
	"github.com/ba-reynolds/gophersocial/internal/service"
	"github.com/ba-reynolds/gophersocial/internal/util"
)

// AuthHandler handles HTTP requests for authentication
type AuthHandler struct {
	service *service.Service
	logger  *slog.Logger
}

func NewAuthHandler(service *service.Service, logger *slog.Logger) *AuthHandler {
	return &AuthHandler{
		service: service,
		logger:  logger,
	}
}

// RefreshToken godoc
//
//	@Summary		Refresh token
//	@Description	Refresh token
//	@Tags			auth
//	@Accept			json
//	@Param			refresh_token	body	models.RefreshToken	true	"Refresh token"
//	@Success		200		{object}	models.RefreshTokenResponse
//	@Failure		400		{object}	models.Envelope{data=nil,error=apperrors.AppError}
//	@Failure		401		{object}	models.Envelope{data=nil,error=apperrors.AppError}
//	@Failure		500		{object}	models.Envelope{data=nil,error=apperrors.AppError}
//	@Router			/auth/refresh-token [post]
func (h *AuthHandler) RefreshToken(w http.ResponseWriter, r *http.Request) {
	// get refresh token from cookies
	refreshTokenCookie, err := r.Cookie("refresh_token")
	if err != nil {
		// Log HTTP layer errors - these are request parsing concerns
		h.logger.Error("failed to get refresh token from cookie",
			"error", err,
			"path", r.URL.Path,
			"method", r.Method,
		)
		util.RespondWithAppError(w, apperrors.InternalServerError(err))
		return
	}

	// Log the request for debugging
	h.logger.Debug("refresh token request", "path", r.URL.Path)

	accessToken, apperr := h.service.Auth.RefreshToken(r.Context(), refreshTokenCookie.Value)
	if apperr != nil {
		// Don't log service errors - they're already logged at appropriate layer
		// Just handle HTTP response mapping
		if appErr, ok := apperr.(*apperrors.AppError); ok {
			util.RespondWithAppError(w, appErr)
			return
		}
		util.RespondWithAppError(w, apperrors.InternalServerError(apperr))
		return
	}

	// Create access token response
	tokenResponse := models.RefreshTokenResponse{
		AccessToken: accessToken.TokenString,
	}

	if err := util.RespondWithJson(w, http.StatusOK, tokenResponse); err != nil {
		// Log HTTP response errors - these are HTTP layer concerns
		h.logger.Error("failed to write HTTP response",
			"error", err,
			"status", http.StatusOK,
		)
		util.RespondWithAppError(w, apperrors.InternalServerError(err))
		return
	}
}

// Login godoc
//
//	@Summary		Login
//	@Description	Login
//	@Tags			auth
//	@Accept			json
//	@Param			payload	body	models.LoginRequest	true	"Login payload"
//	@Success		200		{object}	models.LoginResponse
//	@Failure		400		{object}	models.Envelope{data=nil,error=apperrors.AppError}
//	@Failure		401		{object}	models.Envelope{data=nil,error=apperrors.AppError}
//	@Failure		500		{object}	models.Envelope{data=nil,error=apperrors.AppError}
//	@Router			/auth/login [post]
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var payload models.LoginRequest
	if err := util.ReadJSON(r, &payload); err != nil {
		// Log JSON parsing errors - these are HTTP layer concerns
		h.logger.Warn("invalid JSON payload in request",
			"error", err,
			"content_type", r.Header.Get("Content-Type"),
		)
		util.RespondWithAppError(w, apperrors.PayloadValidationError(err))
		return
	}

	// Validate the payload
	if err := util.Validate.Struct(payload); err != nil {
		// Log validation errors - these are HTTP layer concerns
		h.logger.Warn("payload validation failed",
			"error", err,
		)
		util.RespondWithAppError(w, apperrors.PayloadValidationError(err))
		return
	}

	// Log the login request for debugging
	h.logger.Debug("login request", "identifier", payload.Identifier)

	accessToken, refreshToken, err := h.service.Auth.Login(r.Context(), payload.Identifier, payload.Password, r.RemoteAddr, r.UserAgent())
	if err != nil {
		// Don't log service errors - they're already logged at appropriate layer
		// Just handle HTTP response mapping
		if appErr, ok := err.(*apperrors.AppError); ok {
			util.RespondWithAppError(w, appErr)
			return
		}
		util.RespondWithAppError(w, apperrors.InternalServerError(err))
		return
	}

	// set refresh token cookie
	http.SetCookie(w, &http.Cookie{
		Name:  "refresh_token",
		Value: refreshToken.TokenString,
		Path:  "/",
		// TODO: change to same site lax when in production
		// TODO: change to true when in production
		Secure:   true,
		SameSite: http.SameSiteNoneMode,
	})

	response := models.LoginResponse{
		AccessToken: accessToken.TokenString,
	}

	if err := util.RespondWithJson(w, http.StatusOK, response); err != nil {
		// Log HTTP response errors - these are HTTP layer concerns
		h.logger.Error("failed to write HTTP response",
			"error", err,
			"status", http.StatusOK,
		)
		util.RespondWithAppError(w, apperrors.InternalServerError(err))
		return
	}
}

// Register godoc
//
//	@Summary		Register
//	@Description	Register a new user account
//	@Tags			auth
//	@Accept			json
//	@Param			payload	body	models.RegisterRequest	true	"Register payload"
//	@Success		201		{object}	models.RegisterResponse
//	@Failure		400		{object}	models.Envelope{data=nil,error=apperrors.AppError}
//	@Failure		409		{object}	models.Envelope{data=nil,error=apperrors.AppError}
//	@Failure		500		{object}	models.Envelope{data=nil,error=apperrors.AppError}
//	@Router			/auth/register [post]
func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	var payload models.RegisterRequest
	if err := util.ReadJSON(r, &payload); err != nil {
		// Log JSON parsing errors - these are HTTP layer concerns
		h.logger.Warn("invalid JSON payload in request",
			"error", err,
			"content_type", r.Header.Get("Content-Type"),
		)
		util.RespondWithAppError(w, apperrors.PayloadValidationError(err))
		return
	}

	// Validate the payload
	if err := util.Validate.Struct(payload); err != nil {
		// Log validation errors - these are HTTP layer concerns
		h.logger.Warn("payload validation failed",
			"error", err,
		)
		util.RespondWithAppError(w, apperrors.PayloadValidationError(err))
		return
	}

	// Log the registration request for debugging
	h.logger.Debug("registration request", "username", payload.Username, "email", payload.Email)

	user, accessToken, refreshToken, err := h.service.Auth.Register(r.Context(), payload.Username, payload.Email, payload.Password, r.RemoteAddr, r.UserAgent())
	if err != nil {
		// Don't log service errors - they're already logged at appropriate layer
		// Just handle HTTP response mapping
		if appErr, ok := err.(*apperrors.AppError); ok {
			util.RespondWithAppError(w, appErr)
			return
		}
		util.RespondWithAppError(w, apperrors.InternalServerError(err))
		return
	}

	// set refresh token cookie
	http.SetCookie(w, &http.Cookie{
		Name:  "refresh_token",
		Value: refreshToken.TokenString,
		Path:  "/",
		// TODO: change to same site lax when in production
		// TODO: change to true when in production
		Secure:   true,
		SameSite: http.SameSiteNoneMode,
	})

	response := models.RegisterResponse{
		User:        user,
		AccessToken: accessToken.TokenString,
	}

	if err := util.RespondWithJson(w, http.StatusCreated, response); err != nil {
		// Log HTTP response errors - these are HTTP layer concerns
		h.logger.Error("failed to write HTTP response",
			"error", err,
			"status", http.StatusCreated,
		)
		util.RespondWithAppError(w, apperrors.InternalServerError(err))
		return
	}
}

// Logout godoc
//
//	@Summary		Logout
//	@Description	Logout
//	@Tags			auth
//	@Accept			json
//	@Success		200		{object}	models.Envelope{data=nil,error=nil}
//	@Failure		400		{object}	models.Envelope{data=nil,error=apperrors.AppError}
//	@Failure		401		{object}	models.Envelope{data=nil,error=apperrors.AppError}
//	@Failure		500		{object}	models.Envelope{data=nil,error=apperrors.AppError}
//	@Router			/auth/logout [post]
func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	refreshTokenCookie, err := r.Cookie("refresh_token")
	if err != nil {
		// Log HTTP layer errors - these are request parsing concerns
		h.logger.Error("failed to get refresh token from cookie",
			"error", err,
			"path", r.URL.Path,
			"method", r.Method,
		)
		util.RespondWithAppError(w, apperrors.InternalServerError(err))
		return
	}

	// Log the logout request for debugging
	h.logger.Debug("logout request", "path", r.URL.Path)

	err = h.service.Auth.Logout(r.Context(), refreshTokenCookie.Value)
	if err != nil {
		// Don't log service errors - they're already logged at appropriate layer
		// Just handle HTTP response mapping
		if appErr, ok := err.(*apperrors.AppError); ok {
			util.RespondWithAppError(w, appErr)
			return
		}
		util.RespondWithAppError(w, apperrors.InternalServerError(err))
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:  "refresh_token",
		Value: "",
		Path:  "/",
	})

	// Log successful operations
	h.logger.Info("logout completed successfully")

	if err := util.RespondWithJson(w, http.StatusOK, nil); err != nil {
		// Log HTTP response errors - these are HTTP layer concerns
		h.logger.Error("failed to write HTTP response",
			"error", err,
			"status", http.StatusOK,
		)
		util.RespondWithAppError(w, apperrors.InternalServerError(err))
		return
	}
}
