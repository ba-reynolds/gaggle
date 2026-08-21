package handlers

import (
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"net/http"
	"net/url"
	"strings"

	"github.com/ba-reynolds/gaggle/internal/apperrors"
	"github.com/ba-reynolds/gaggle/internal/auth"
	"github.com/ba-reynolds/gaggle/internal/models"
	"github.com/ba-reynolds/gaggle/internal/service"
	"github.com/ba-reynolds/gaggle/internal/util"
	"github.com/ba-reynolds/gaggle/pkg/config"
)

// AuthHandler handles HTTP requests for authentication
type AuthHandler struct {
	service      *service.Service
	logger       *slog.Logger
	cookieSecure bool
	googleConfig config.GoogleOAuthConfig
}

func NewAuthHandler(service *service.Service, logger *slog.Logger, cookieSecure bool) *AuthHandler {
	return &AuthHandler{
		service:      service,
		logger:       logger,
		cookieSecure: cookieSecure,
	}
}

func NewAuthHandlerWithGoogle(service *service.Service, logger *slog.Logger, cookieSecure bool, googleConfig config.GoogleOAuthConfig) *AuthHandler {
	return &AuthHandler{
		service:      service,
		logger:       logger,
		cookieSecure: cookieSecure,
		googleConfig: googleConfig,
	}
}

// setRefreshTokenCookie writes the refresh-token cookie. The Secure attribute
// follows the scheme the client actually used (via nginx's X-Forwarded-Proto),
// NOT just the configured COOKIE_SECURE: browsers reject Secure cookies over
// plain http, so a box serving http://<ip> (as the pilot does) would never be
// able to persist a refresh cookie and every session would die on the next
// bootstrap/refresh. When the proxy header is absent (direct API access) the
// configured COOKIE_SECURE is the fallback.
func (h *AuthHandler) setRefreshTokenCookie(w http.ResponseWriter, r *http.Request, token string) {
	secure := h.cookieSecure
	if proto := r.Header.Get("X-Forwarded-Proto"); proto != "" {
		secure = proto == "https"
	}
	http.SetCookie(w, &http.Cookie{
		Name:     "refresh_token",
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	})
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

	accessToken, refreshToken, apperr := h.service.Auth.RefreshToken(r.Context(), refreshTokenCookie.Value, r.RemoteAddr, r.UserAgent())
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

	// Refresh tokens rotate on every use: hand the successor back via the same
	// httpOnly cookie so the client never replays a revoked token.
	h.setRefreshTokenCookie(w, r, refreshToken.TokenString)

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
	h.setRefreshTokenCookie(w, r, refreshToken.TokenString)

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

	user, accessToken, refreshToken, err := h.service.Auth.Register(r.Context(), payload.Username, payload.Email, payload.Password, payload.Language, r.RemoteAddr, r.UserAgent())
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
	h.setRefreshTokenCookie(w, r, refreshToken.TokenString)

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

// GoogleAuth godoc
//
//	@Summary		Google OAuth login
//	@Description	Verify a Google ID token (from GIS) and sign the user in or create an account
//	@Tags			auth
//	@Accept			json
//	@Param			payload	body	models.GoogleAuthRequest	true	"Google credential"
//	@Success		200		{object}	models.GoogleAuthResponse
//	@Failure		400		{object}	models.Envelope{data=nil,error=apperrors.AppError}
//	@Failure		401		{object}	models.Envelope{data=nil,error=apperrors.AppError}
//	@Failure		500		{object}	models.Envelope{data=nil,error=apperrors.AppError}
//	@Router			/auth/google [post]
func (h *AuthHandler) GoogleAuth(w http.ResponseWriter, r *http.Request) {
	if !h.googleConfig.Enabled {
		util.RespondWithAppError(w, apperrors.InternalServerError(nil))
		return
	}
	var payload models.GoogleAuthRequest
	if err := util.ReadJSON(r, &payload); err != nil {
		h.logger.Warn("invalid JSON payload in google auth", "error", err)
		util.RespondWithAppError(w, apperrors.PayloadValidationError(err))
		return
	}
	// Accept either id_token or credential (GIS uses credential)
	idToken := payload.IdToken
	if idToken == "" {
		idToken = payload.Credential
	}
	if idToken == "" {
		util.RespondWithAppError(w, apperrors.PayloadValidationError(nil))
		return
	}
	user, accessToken, refreshToken, isNew, err := h.service.Auth.GoogleAuth(r.Context(), idToken, h.googleConfig.ClientID, h.googleConfig.AllowUnverifiedEmail, payload.Language, r.RemoteAddr, r.UserAgent())
	if err != nil {
		if appErr, ok := err.(*apperrors.AppError); ok {
			util.RespondWithAppError(w, appErr)
			return
		}
		util.RespondWithAppError(w, apperrors.InternalServerError(err))
		return
	}
	h.setRefreshTokenCookie(w, r, refreshToken.TokenString)
	_ = user
	resp := models.GoogleAuthResponse{
		AccessToken: accessToken.TokenString,
		IsNewUser:   isNew,
	}
	if err := util.RespondWithJson(w, http.StatusOK, resp); err != nil {
		h.logger.Error("failed to write google auth response", "error", err)
		util.RespondWithAppError(w, apperrors.InternalServerError(err))
		return
	}
}

// GoogleOAuthLogin godoc
//
//	@Summary		Start Google OAuth code flow
//	@Description	Redirects the user to Google's consent screen
//	@Tags			auth
//	@Success		302
//	@Router			/auth/google/login [get]
func (h *AuthHandler) GoogleOAuthLogin(w http.ResponseWriter, r *http.Request) {
	if !h.googleConfig.Enabled {
		http.Error(w, "Google OAuth not configured", http.StatusNotImplemented)
		return
	}
	state := randomState()
	// Store state in short-lived cookie for CSRF protection
	http.SetCookie(w, &http.Cookie{
		Name:     "oauth_state",
		Value:    state,
		Path:     "/",
		HttpOnly: true,
		Secure:   h.cookieSecure && r.Header.Get("X-Forwarded-Proto") != "http",
		SameSite: http.SameSiteLaxMode,
		MaxAge:   600,
	})
	authURL := auth.BuildGoogleAuthURL(h.googleConfig.ClientID, h.googleConfig.RedirectURL, state)
	http.Redirect(w, r, authURL, http.StatusFound)
}

// GoogleOAuthCallback godoc
//
//	@Summary		Google OAuth callback
//	@Description	Handles the redirect from Google, creates/links the user, and redirects to the frontend with an access token
//	@Tags			auth
//	@Success		302
//	@Router			/auth/google/callback [get]
func (h *AuthHandler) GoogleOAuthCallback(w http.ResponseWriter, r *http.Request) {
	if !h.googleConfig.Enabled {
		http.Error(w, "Google OAuth not configured", http.StatusNotImplemented)
		return
	}
	// Validate state
	stateCookie, err := r.Cookie("oauth_state")
	if err != nil {
		h.logger.Warn("missing oauth_state cookie")
		http.Error(w, "invalid state", http.StatusBadRequest)
		return
	}
	queryState := r.URL.Query().Get("state")
	if queryState == "" || queryState != stateCookie.Value {
		h.logger.Warn("oauth state mismatch", "cookie", stateCookie.Value, "query", queryState)
		http.Error(w, "invalid state", http.StatusBadRequest)
		return
	}
	// Clear state cookie
	http.SetCookie(w, &http.Cookie{Name: "oauth_state", Value: "", Path: "/", MaxAge: -1})

	if errMsg := r.URL.Query().Get("error"); errMsg != "" {
		h.logger.Warn("google oauth error", "error", errMsg)
		http.Error(w, "google oauth: "+errMsg, http.StatusUnauthorized)
		return
	}
	code := r.URL.Query().Get("code")
	if code == "" {
		http.Error(w, "missing code", http.StatusBadRequest)
		return
	}
	// Exchange code for tokens
	tokenResp, err := auth.ExchangeGoogleCode(r.Context(), h.googleConfig.ClientID, h.googleConfig.ClientSecret, h.googleConfig.RedirectURL, code)
	if err != nil {
		h.logger.Error("google code exchange failed", "error", err)
		if appErr, ok := err.(*apperrors.AppError); ok {
			util.RespondWithAppError(w, appErr)
			return
		}
		http.Error(w, "code exchange failed", http.StatusUnauthorized)
		return
	}
	var googleInfo *models.GoogleUserInfo
	if tokenResp.IDToken != "" {
		googleInfo, err = auth.VerifyGoogleIDToken(r.Context(), tokenResp.IDToken, h.googleConfig.ClientID)
		if err != nil {
			h.logger.Error("failed to verify id_token from code exchange", "error", err)
			// fall back to userinfo via access token
			googleInfo = nil
		}
	}
	if googleInfo == nil && tokenResp.AccessToken != "" {
		googleInfo, err = auth.FetchGoogleUserInfo(r.Context(), tokenResp.AccessToken)
		if err != nil {
			h.logger.Error("failed to fetch google userinfo", "error", err)
			http.Error(w, "failed to fetch userinfo", http.StatusUnauthorized)
			return
		}
		if !googleInfo.EmailVerified && !h.googleConfig.AllowUnverifiedEmail {
			http.Error(w, "email not verified", http.StatusUnauthorized)
			return
		}
	}
	if googleInfo == nil {
		http.Error(w, "failed to obtain google identity", http.StatusUnauthorized)
		return
	}
	// Prefer the verified ID token path (re-uses the same service logic as the GIS credential flow).
	// In practice Google always returns an id_token when scope includes openid.
	if tokenResp.IDToken == "" {
		http.Error(w, "missing id_token from google", http.StatusUnauthorized)
		return
	}
	_, accessToken, refreshToken, isNew, err := h.service.Auth.GoogleAuth(r.Context(), tokenResp.IDToken, h.googleConfig.ClientID, h.googleConfig.AllowUnverifiedEmail, "", r.RemoteAddr, r.UserAgent())
	if err != nil {
		h.logger.Error("google callback auth failed", "error", err)
		if appErr, ok := err.(*apperrors.AppError); ok {
			util.RespondWithAppError(w, appErr)
			return
		}
		http.Error(w, "auth failed", http.StatusUnauthorized)
		return
	}
	h.setRefreshTokenCookie(w, r, refreshToken.TokenString)
	h.redirectToFrontend(w, r, accessToken.TokenString, isNew)
}

func (h *AuthHandler) redirectToFrontend(w http.ResponseWriter, r *http.Request, accessToken string, isNew bool) {
	base := h.googleConfig.FrontendRedirectURL
	if base == "" {
		base = "/"
	}
	// Ensure we don't double-slash
	base = strings.TrimRight(base, "/")
	frontendURL := base + "/auth/callback?access_token=" + url.QueryEscape(accessToken)
	if isNew {
		frontendURL += "&is_new_user=1"
	}
	http.Redirect(w, r, frontendURL, http.StatusFound)
}

func randomState() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return hex.EncodeToString([]byte("fallback-state-1234"))
	}
	return hex.EncodeToString(b)
}
