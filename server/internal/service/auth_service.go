package service

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"strings"

	"github.com/ba-reynolds/gaggle/internal/apperrors"
	"github.com/ba-reynolds/gaggle/internal/auth"
	"github.com/ba-reynolds/gaggle/internal/models"
	"github.com/ba-reynolds/gaggle/internal/store"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

type AuthService struct {
	store         *store.Store
	authenticator *auth.JWTAuthenticator
	logger        *slog.Logger
}

// Some of these functions are simple wrappers around the authenticator,
// but I like to have them in the service layer for future extensibility

func (s *AuthService) Login(ctx context.Context, identifier string, password string, ipAddress string, userAgent string) (*models.Token, *models.Token, error) {
	var user *models.User
	var err error

	if strings.Contains(identifier, "@") {
		user, err = s.store.Users.GetByEmail(ctx, identifier)
	} else {
		user, err = s.store.Users.GetByUsername(ctx, identifier)
	}

	// Couldn't find user with given identifier
	if err != nil {
		return nil, nil, apperrors.NotFoundError("user not found", err)
	}

	// OAuth-only accounts have no password; nudge the user to use Google.
	if user.Password == "" {
		return nil, nil, apperrors.UnauthorizedError("this account uses Google sign-in; please continue with Google", nil)
	}

	// Password is incorrect
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password)); err != nil {
		s.logger.Debug("incorrect password during login attempt",
			"identifier", identifier,
			"userID", user.ID,
			"operation", "login",
		)
		return nil, nil, apperrors.UnauthorizedError("invalid credentials", err)
	}

	refreshToken, err := s.CreateRefreshToken(ctx, user.ID, ipAddress, userAgent)
	if err != nil {
		return nil, nil, err
	}

	// Create access token
	accessToken, err := s.authenticator.GenerateAccessToken(user.ID)
	if err != nil {
		s.logger.Error("failed to generate access token",
			"operation", "login",
			"userID", user.ID,
			"identifier", identifier,
			"error", err,
		)
		return nil, nil, apperrors.InternalServerError(err)
	}

	// Log successful business operations
	s.logger.Info("user logged in successfully",
		"userID", user.ID,
		"identifier", identifier,
	)

	return accessToken, refreshToken, nil
}

// RefreshToken validates a refresh token, rotates it (issues a successor and
// retires the presented one atomically), and mints a fresh access token.
// Reusing an already-rotated token is treated as theft: the whole session
// family is revoked. An expired-but-well-signed token maps to SESSION_EXPIRED
// so the client can distinguish "session over" from "bad credentials".
func (s *AuthService) RefreshToken(ctx context.Context, refreshTokenString string, ipAddress string, userAgent string) (*models.Token, *models.Token, error) {
	// Validate refresh token
	parsedRefreshToken, err := s.authenticator.ValidateToken(refreshTokenString, auth.RefreshToken)
	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			s.logger.Info("expired refresh token used",
				"operation", "refresh_token",
				"error", err,
			)
			return nil, nil, apperrors.SessionExpiredError("session expired", err)
		}
		s.logger.Error("failed to validate refresh token",
			"operation", "refresh_token",
			"error", err,
		)
		return nil, nil, apperrors.UnauthorizedError("invalid token", err)
	}

	// Check the DB row for the presented token.
	refreshTokenHash := auth.HashToken(refreshTokenString)
	storedRefreshToken, err := s.store.Auth.GetRefreshToken(ctx, refreshTokenHash)
	if err != nil {
		// Row missing entirely: nothing to replay against. Treat as invalid.
		if apperrors.Is(err, apperrors.NotFound) {
			return nil, nil, apperrors.UnauthorizedError("invalid token", nil)
		}
		return nil, nil, err
	}

	if storedRefreshToken.Revoked {
		// A rotated token being replayed is usually the signature of a stolen
		// credential. BUT two tabs sharing one cookie jar (or a lost refresh
		// response) replay it routinely: tab A rotates the token, tab B — still
		// holding the pre-rotation cookie — presents the now-revoked value.
		// Distinguish by user-agent: a replay from the SAME device is benign and
		// resolves by rotating the family's CURRENT token forward (everyone
		// stays logged in). A replay from a DIFFERENT device is theft.
		if storedRefreshToken.RevokedReason != nil && *storedRefreshToken.RevokedReason == models.RevokedReasonRotated {
			if storedRefreshToken.UserAgent != "" && storedRefreshToken.UserAgent == userAgent {
				s.logger.Info("rotated refresh token replayed from same device; rotating current token instead of revoking session",
					"sessionID", storedRefreshToken.SessionID.String(),
					"userID", storedRefreshToken.UserID,
					"operation", "refresh_token",
				)
				accessToken, newRefreshToken, err := s.rotateCurrentActiveToken(ctx, storedRefreshToken.SessionID, storedRefreshToken.UserID, ipAddress, userAgent)
				if err != nil {
					if apperrors.Is(err, apperrors.NotFound) {
						// No live token left in the family: the session really is over.
						return nil, nil, apperrors.SessionExpiredError("session expired", nil)
					}
					return nil, nil, err
				}
				return accessToken, newRefreshToken, nil
			}

			s.logger.Warn("rotated refresh token reused from different device; revoking session family",
				"sessionID", storedRefreshToken.SessionID.String(),
				"userID", storedRefreshToken.UserID,
				"operation", "refresh_token",
			)
			if revokeErr := s.store.Auth.RevokeSession(ctx, nil, storedRefreshToken.SessionID, models.RevokedReasonTheft); revokeErr != nil {
				return nil, nil, revokeErr
			}
		}
		return nil, nil, apperrors.SessionExpiredError("session expired", nil)
	}

	// Get user ID from token
	userID, err := s.authenticator.GetUserIDFromToken(parsedRefreshToken)
	if err != nil {
		s.logger.Error("failed to get user ID from refresh token",
			"operation", "refresh_token",
			"tokenHash", refreshTokenHash,
			"error", err,
		)
		return nil, nil, apperrors.UnauthorizedError("invalid token", err)
	}

	// Check if user exists
	if _, err := s.store.Users.GetByID(ctx, userID); err != nil {
		return nil, nil, err
	}

	// Rotate the refresh token atomically: insert the successor, retire the
	// presented token, all in one transaction.
	tx, err := s.store.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, nil, apperrors.InternalServerError(err)
	}
	defer tx.Rollback()

	newRefreshToken, err := s.issueRefreshToken(ctx, tx, userID, storedRefreshToken.SessionID, ipAddress, userAgent)
	if err != nil {
		return nil, nil, err
	}
	if err := s.store.Auth.RotateRefreshToken(ctx, tx, storedRefreshToken.TokenHash); err != nil {
		return nil, nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, nil, apperrors.InternalServerError(err)
	}

	// Generate new access token
	accessToken, err := s.authenticator.GenerateAccessToken(userID)
	if err != nil {
		s.logger.Error("failed to generate new access token during refresh",
			"operation", "refresh_token",
			"userID", userID,
			"error", err,
		)
		return nil, nil, apperrors.InternalServerError(err)
	}

	// Log successful business operations
	s.logger.Info("token refreshed successfully",
		"userID", userID,
	)

	return accessToken, newRefreshToken, nil
}

// rotateCurrentActiveToken issues a fresh refresh token for a session family,
// atomically retiring the family's currently-active token. Used to keep a
// session alive when a stale (already-rotated) token is replayed from the same
// device: instead of nuking the whole family, the live token simply moves one
// step forward so every open client can keep refreshing.
func (s *AuthService) rotateCurrentActiveToken(ctx context.Context, sessionID uuid.UUID, userID int, ipAddress string, userAgent string) (*models.Token, *models.Token, error) {
	for attempt := 0; attempt < 3; attempt++ {
		tx, err := s.store.DB.BeginTx(ctx, nil)
		if err != nil {
			return nil, nil, apperrors.InternalServerError(err)
		}

		active, err := s.store.Auth.GetCurrentActiveToken(ctx, tx, sessionID)
		if err != nil {
			tx.Rollback()
			return nil, nil, err
		}

		newRefreshToken, err := s.issueRefreshToken(ctx, tx, userID, sessionID, ipAddress, userAgent)
		if err != nil {
			tx.Rollback()
			return nil, nil, err
		}
		if err := s.store.Auth.RotateRefreshToken(ctx, tx, active.TokenHash); err != nil {
			tx.Rollback()
			if apperrors.Is(err, apperrors.NotFound) && attempt < 2 {
				continue
			}
			return nil, nil, err
		}
		if err := tx.Commit(); err != nil {
			return nil, nil, apperrors.InternalServerError(err)
		}

		// Generate new access token
		accessToken, err := s.authenticator.GenerateAccessToken(userID)
		if err != nil {
			s.logger.Error("failed to generate new access token during refresh",
				"operation", "refresh_token",
				"userID", userID,
				"error", err,
			)
			return nil, nil, apperrors.InternalServerError(err)
		}
		return accessToken, newRefreshToken, nil
	}
	return nil, nil, apperrors.SessionExpiredError("session expired", nil)
}

// CreateRefreshToken issues a refresh token for a brand-new session family.
// Used at login and registration.
func (s *AuthService) CreateRefreshToken(ctx context.Context, userID int, RemoteAddr string, userAgent string) (*models.Token, error) {
	return s.issueRefreshToken(ctx, nil, userID, uuid.New(), RemoteAddr, userAgent)
}

// issueRefreshToken generates a signed refresh token and persists its hash.
// sessionID ties it to a session family so rotation chain revocation works.
// When tx is non-nil the row is created inside that transaction (rotation).
func (s *AuthService) issueRefreshToken(ctx context.Context, tx *sql.Tx, userID int, sessionID uuid.UUID, remoteAddr string, userAgent string) (*models.Token, error) {
	refreshToken, err := s.authenticator.GenerateRefreshToken(userID)
	if err != nil {
		s.logger.Error("failed to generate refresh token",
			"operation", "issue_refresh_token",
			"userID", userID,
			"error", err,
		)
		return nil, apperrors.InternalServerError(err)
	}
	refreshTokenHash := auth.HashToken(refreshToken.TokenString)

	// Make it fit within the database column
	if len(userAgent) > 255 {
		userAgent = userAgent[:255]
	}

	// Strip port from the ip address
	ipAddress := strings.Split(remoteAddr, ":")[0]

	err = s.store.Auth.CreateRefreshToken(ctx, tx, &models.RefreshToken{
		SessionID:   sessionID,
		TokenHash:   refreshTokenHash,
		UserID:      userID,
		IssuedAt:    refreshToken.IssuedAt,
		Revoked:     false,
		RevokedAt:   nil,
		ExpiresAt:   refreshToken.ExpiresAt,
		IPAddress:   ipAddress,
		UserAgent:   userAgent,
	})
	if err != nil {
		return nil, err
	}

	return refreshToken, nil
}

func (s *AuthService) Logout(ctx context.Context, refreshTokenString string) error {
	refreshTokenHash := auth.HashToken(refreshTokenString)
	stored, err := s.store.Auth.GetRefreshToken(ctx, refreshTokenHash)
	if err != nil {
		// Session already gone (expired, rotated, or unknown token): logout
		// stays idempotent so clearing the cookie is always valid.
		if apperrors.Is(err, apperrors.NotFound) {
			s.logger.Debug("logout for unknown refresh token",
				"operation", "logout",
			)
			return nil
		}
		return err
	}

	if err := s.store.Auth.RevokeSession(ctx, nil, stored.SessionID, models.RevokedReasonLogout); err != nil {
		return err
	}

	// Log successful business operations
	s.logger.Info("user logged out successfully",
		"userID", stored.UserID,
		"sessionID", stored.SessionID.String(),
	)

	return nil
}

func (s *AuthService) ValidateToken(tokenString string, tokenType auth.TokenType) (*jwt.Token, error) {
	return s.authenticator.ValidateToken(tokenString, tokenType)
}

func (s *AuthService) GetUserIDFromToken(token *jwt.Token) (int, error) {
	return s.authenticator.GetUserIDFromToken(token)
}

// GetUserIDFromRefreshToken authenticates a long-lived connection (e.g. the
// realtime stream) from a refresh token. It checks the session *family* rather
// than the exact token so the stream survives refresh-token rotation; only a
// fully dead (logged-out or stolen) session is rejected.
func (s *AuthService) GetUserIDFromRefreshToken(ctx context.Context, tokenString string) (int, error) {
	parsed, err := s.authenticator.ValidateToken(tokenString, auth.RefreshToken)
	if err != nil {
		return 0, apperrors.UnauthorizedError("invalid token", err)
	}

	hash := auth.HashToken(tokenString)
	stored, err := s.store.Auth.GetRefreshToken(ctx, hash)
	if err != nil {
		if apperrors.Is(err, apperrors.NotFound) {
			return 0, apperrors.UnauthorizedError("invalid token", nil)
		}
		return 0, err
	}

	userID, err := s.authenticator.GetUserIDFromToken(parsed)
	if err != nil {
		return 0, apperrors.UnauthorizedError("invalid token", err)
	}

	active, err := s.store.Auth.SessionHasActiveToken(ctx, stored.SessionID)
	if err != nil {
		return 0, err
	}
	if !active {
		return 0, apperrors.UnauthorizedError("session is not active", nil)
	}

	if _, err := s.store.Users.GetByID(ctx, userID); err != nil {
		return 0, err
	}
	return userID, nil
}

func (s *AuthService) Register(ctx context.Context, username string, email string, password string, language string, ipAddress string, userAgent string) (*models.User, *models.Token, *models.Token, error) {
	// Hash the password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		s.logger.Error("failed to hash password during registration",
			"operation", "register",
			"username", username,
			"email", email,
			"error", err,
		)
		return nil, nil, nil, apperrors.InternalServerError(err)
	}

	// Create user object
	user := &models.User{
		Username: username,
		Email:    email,
		Password: string(hashedPassword),
	}

	// Create user in database using transaction
	tx, err := s.store.DB.BeginTx(ctx, nil)
	if err != nil {
		s.logger.Error("failed to begin transaction for user creation",
			"operation", "register",
			"username", username,
			"email", email,
			"error", err,
		)
		return nil, nil, nil, apperrors.InternalServerError(err)
	}
	defer tx.Rollback()

	err = s.store.Users.Create(ctx, tx, user)
	if err != nil {
		if appErr, ok := err.(*apperrors.AppError); ok && appErr.Code == apperrors.AlreadyExists {
			s.logger.Info("user registration failed due to duplicate",
				"username", username,
				"email", email,
			)
		}
		return nil, nil, nil, err
	}

	// Seed the settings row so language preference (from the browser) sticks
	// from day one. Defaults mirror the user_settings column default.
	settings := defaultSettings(language)
	if err := s.store.Users.CreateSettings(ctx, tx, user.ID, settings); err != nil {
		s.logger.Error("failed to seed settings during registration",
			"operation", "register",
			"userID", user.ID,
			"language", language,
			"error", err,
		)
		return nil, nil, nil, err
	}

	// Commit the transaction
	if err := tx.Commit(); err != nil {
		s.logger.Error("failed to commit transaction for user creation",
			"operation", "register",
			"username", username,
			"userID", user.ID,
			"error", err,
		)
		return nil, nil, nil, apperrors.InternalServerError(err)
	}

	// Create refresh token
	refreshToken, err := s.CreateRefreshToken(ctx, user.ID, ipAddress, userAgent)
	if err != nil {
		return nil, nil, nil, err
	}

	// Create access token
	accessToken, err := s.authenticator.GenerateAccessToken(user.ID)
	if err != nil {
		s.logger.Error("failed to generate access token during registration",
			"operation", "register",
			"userID", user.ID,
			"error", err,
		)
		return nil, nil, nil, apperrors.InternalServerError(err)
	}

	// Log successful business operations
	s.logger.Info("user registered successfully",
		"username", username,
		"userID", user.ID,
		"email", email,
	)

	return user, accessToken, refreshToken, nil
}

// GoogleAuth authenticates or creates a user from a verified Google ID token.
// clientID is used to validate aud; allowUnverified controls whether unverified
// Google emails are accepted. Returns user, accessToken, refreshToken, isNewUser.
func (s *AuthService) GoogleAuth(ctx context.Context, idToken string, clientID string, allowUnverified bool, language string, ipAddress string, userAgent string) (*models.User, *models.Token, *models.Token, bool, error) {
	info, err := auth.VerifyGoogleIDToken(ctx, idToken, clientID)
	if err != nil {
		return nil, nil, nil, false, err
	}
	if !info.EmailVerified && !allowUnverified {
		return nil, nil, nil, false, apperrors.UnauthorizedError("google email not verified", nil)
	}
	// 1) Existing google_id linkage
	if user, err := s.store.Users.GetByGoogleID(ctx, info.Sub); err == nil {
		accessToken, refreshToken, err := s.issueTokensForUser(ctx, user.ID, ipAddress, userAgent)
		if err != nil {
			return nil, nil, nil, false, err
		}
		s.logger.Info("google login (existing google_id)", "userID", user.ID, "google_sub", info.Sub)
		return user, accessToken, refreshToken, false, nil
	}
	// 2) Link by email if local account exists
	if existing, err := s.store.Users.GetByEmail(ctx, info.Email); err == nil {
		// Link the google_id if not already set
		if existing.GoogleID == nil || *existing.GoogleID == "" {
			if err := s.store.Users.LinkGoogleID(ctx, existing.ID, info.Sub); err != nil {
				// If linking fails due to uniqueness, maybe another user already has it; fall through
				s.logger.Warn("failed to link google_id to existing email account", "userID", existing.ID, "google_sub", info.Sub, "error", err)
			} else {
				existing.GoogleID = &info.Sub
			}
		} else if *existing.GoogleID != info.Sub {
			return nil, nil, nil, false, apperrors.UnauthorizedError("this email is already linked to a different Google account", nil)
		}
		accessToken, refreshToken, err := s.issueTokensForUser(ctx, existing.ID, ipAddress, userAgent)
		if err != nil {
			return nil, nil, nil, false, err
		}
		s.logger.Info("google login (linked existing email)", "userID", existing.ID, "google_sub", info.Sub)
		return existing, accessToken, refreshToken, false, nil
	}
	// 3) Create new user
	username := deriveUsername(info)
	// Ensure username is valid: 3-16, alphanumeric + underscore; fallback if needed
	username = sanitizeUsername(username)
	if len(username) < 3 {
		username = fmt.Sprintf("user_%s", info.Sub[:6])
		username = sanitizeUsername(username)
	}
	// Try base username, then suffixed variants
	var created *models.User
	var isNew bool
	for attempt := 0; attempt < 5; attempt++ {
		trial := username
		if attempt > 0 {
			suffix := fmt.Sprintf("%d", attempt)
			// keep within 16 chars
			maxBase := 16 - len(suffix)
			if len(trial) > maxBase {
				trial = trial[:maxBase]
			}
			trial = trial + suffix
		}
		user := &models.User{
			Username:     trial,
			Email:        info.Email,
			Password:     "", // oauth-only, no password
			GoogleID:     &info.Sub,
			AuthProvider: "google",
		}
		tx, err := s.store.DB.BeginTx(ctx, nil)
		if err != nil {
			return nil, nil, nil, false, apperrors.InternalServerError(err)
		}
		err = s.store.Users.Create(ctx, tx, user)
		if err != nil {
			tx.Rollback()
			if apperr, ok := err.(*apperrors.AppError); ok && apperr.Code == apperrors.AlreadyExists {
				continue
			}
			return nil, nil, nil, false, err
		}
		settings := defaultSettings(language)
		if err := s.store.Users.CreateSettings(ctx, tx, user.ID, settings); err != nil {
			tx.Rollback()
			return nil, nil, nil, false, err
		}
		// Set display name from Google name if available
		displayName := info.Name
		if displayName == "" {
			displayName = trial
		}
		if len(displayName) > 50 {
			displayName = displayName[:50]
		}
		// Update profile display_name (create already seeded it as username)
		if _, err := tx.ExecContext(ctx, `UPDATE user_profiles SET display_name = $1 WHERE user_id = $2`, displayName, user.ID); err != nil {
			// non-fatal; profile already exists with username
			s.logger.Warn("failed to set google display_name", "userID", user.ID, "displayName", displayName, "error", err)
		}
		if err := tx.Commit(); err != nil {
			return nil, nil, nil, false, apperrors.InternalServerError(err)
		}
		created = user
		isNew = true
		break
	}
	// Fallback: random suffix if deterministic attempts all collided
	for attempt := 0; created == nil && attempt < 5; attempt++ {
		randSuffix := randomUsernameSuffix()
		// Keep base truncated to 9 chars so "base_xxxxxx" <=16
		base := username
		if len(base) > 9 {
			base = base[:9]
		}
		base = strings.TrimSuffix(base, "_")
		trial := fmt.Sprintf("%s_%s", base, randSuffix)
		if len(trial) > 16 {
			trial = trial[:16]
		}
		// If random still collides, try pure user_xxxxxx form
		if attempt == 4 {
			trial = fmt.Sprintf("user_%s", randSuffix)
		}
		user := &models.User{
			Username:     trial,
			Email:        info.Email,
			Password:     "",
			GoogleID:     &info.Sub,
			AuthProvider: "google",
		}
		tx, err := s.store.DB.BeginTx(ctx, nil)
		if err != nil {
			return nil, nil, nil, false, apperrors.InternalServerError(err)
		}
		err = s.store.Users.Create(ctx, tx, user)
		if err != nil {
			tx.Rollback()
			if apperr, ok := err.(*apperrors.AppError); ok && apperr.Code == apperrors.AlreadyExists {
				s.logger.Info("random username collided, retrying", "trial", trial, "attempt", attempt)
				continue
			}
			return nil, nil, nil, false, err
		}
		settings := defaultSettings(language)
		if err := s.store.Users.CreateSettings(ctx, tx, user.ID, settings); err != nil {
			tx.Rollback()
			return nil, nil, nil, false, err
		}
		displayName := info.Name
		if displayName == "" {
			displayName = trial
		}
		if len(displayName) > 50 {
			displayName = displayName[:50]
		}
		if _, err := tx.ExecContext(ctx, `UPDATE user_profiles SET display_name = $1 WHERE user_id = $2`, displayName, user.ID); err != nil {
			s.logger.Warn("failed to set google display_name", "userID", user.ID, "displayName", displayName, "error", err)
		}
		if err := tx.Commit(); err != nil {
			return nil, nil, nil, false, apperrors.InternalServerError(err)
		}
		created = user
		isNew = true
		break
	}
	if created == nil {
		return nil, nil, nil, false, apperrors.InternalServerError(fmt.Errorf("failed to create google user after retries"))
	}
	accessToken, refreshToken, err := s.issueTokensForUser(ctx, created.ID, ipAddress, userAgent)
	if err != nil {
		return nil, nil, nil, false, err
	}
	s.logger.Info("google signup (new user)", "userID", created.ID, "username", created.Username, "google_sub", info.Sub)
	return created, accessToken, refreshToken, isNew, nil
}

func (s *AuthService) issueTokensForUser(ctx context.Context, userID int, ipAddress string, userAgent string) (*models.Token, *models.Token, error) {
	refreshToken, err := s.CreateRefreshToken(ctx, userID, ipAddress, userAgent)
	if err != nil {
		return nil, nil, err
	}
	accessToken, err := s.authenticator.GenerateAccessToken(userID)
	if err != nil {
		s.logger.Error("failed to generate access token for google user", "userID", userID, "error", err)
		return nil, nil, apperrors.InternalServerError(err)
	}
	return accessToken, refreshToken, nil
}

var usernameSanitizer = regexp.MustCompile(`[^a-zA-Z0-9_]`)

func sanitizeUsername(s string) string {
	s = usernameSanitizer.ReplaceAllString(s, "_")
	if len(s) > 16 {
		s = s[:16]
	}
	return s
}

func randomUsernameSuffix() string {
	b := make([]byte, 3)
	if _, err := rand.Read(b); err != nil {
		// fallback to pseudo-random
		return fmt.Sprintf("%06d", 100000+int(b[0])%900000)
	}
	return hex.EncodeToString(b)
}

func deriveUsername(info *models.GoogleUserInfo) string {
	// Prefer email local part
	if info.Email != "" {
		if idx := strings.Index(info.Email, "@"); idx > 0 {
			candidate := info.Email[:idx]
			candidate = strings.ToLower(candidate)
			if len(candidate) >= 3 {
				return candidate
			}
		}
	}
	if info.GivenName != "" {
		return strings.ToLower(info.GivenName)
	}
	if info.Name != "" {
		parts := strings.Fields(info.Name)
		if len(parts) > 0 {
			return strings.ToLower(parts[0])
		}
	}
	// fallback
	if len(info.Sub) >= 8 {
		return "user_" + info.Sub[:6]
	}
	return "user"
}

// defaultSettings returns the user_settings JSONB defaults (mirroring the
// database column default), with language overridden when non-empty.
func defaultSettings(language string) *models.UserSettings {
	settings := &models.UserSettings{
		Notifications: models.NotificationSettings{Email: true, Push: true, Mentions: true},
		Privacy:       models.PrivacySettings{ProfileVisibility: "public", ShowOnlineStatus: true, AllowTagging: true},
		Appearance:    models.AppearanceSettings{Theme: "system", FontSize: "medium"},
		Language:      "en",
	}
	if language != "" {
		settings.Language = language
	}
	return settings
}