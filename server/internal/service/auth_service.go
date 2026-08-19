package service

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
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
		// A token that was already rotated away is being replayed: that is the
		// signature of a stolen credential. Revoke the entire session family,
		// including its current token, and park the session.
		if storedRefreshToken.RevokedReason != nil && *storedRefreshToken.RevokedReason == models.RevokedReasonRotated {
			s.logger.Warn("rotated refresh token reused; revoking session family",
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

func (s *AuthService) Register(ctx context.Context, username string, email string, password string, ipAddress string, userAgent string) (*models.User, *models.Token, *models.Token, error) {
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