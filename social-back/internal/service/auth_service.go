package service

import (
	"context"
	"log/slog"
	"strings"

	"github.com/ba-reynolds/vitrilium/internal/apperrors"
	"github.com/ba-reynolds/vitrilium/internal/auth"
	"github.com/ba-reynolds/vitrilium/internal/models"
	"github.com/ba-reynolds/vitrilium/internal/store"
	"github.com/golang-jwt/jwt/v5"
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

func (s *AuthService) RefreshToken(ctx context.Context, refreshTokenString string) (*models.Token, error) {
	// Validate refresh token
	parsedRefreshToken, err := s.authenticator.ValidateToken(refreshTokenString, auth.RefreshToken)
	if err != nil {
		s.logger.Error("failed to validate refresh token",
			"operation", "refresh_token",
			"error", err,
		)
		return nil, apperrors.UnauthorizedError("invalid token", err)
	}

	// check db that refresh token hasn't been revoked
	refreshTokenHash := auth.HashToken(refreshTokenString)
	storedRefreshToken, err := s.store.Auth.GetRefreshToken(ctx, refreshTokenHash)
	if err != nil {
		return nil, err
	}

	if storedRefreshToken.Revoked {
		s.logger.Debug("attempted to use revoked refresh token",
			"tokenHash", refreshTokenHash,
			"userID", storedRefreshToken.UserID,
			"operation", "refresh_token",
		)
		return nil, apperrors.UnauthorizedError("invalid token", nil)
	}

	// Get user ID from token
	userID, err := s.authenticator.GetUserIDFromToken(parsedRefreshToken)
	if err != nil {
		s.logger.Error("failed to get user ID from refresh token",
			"operation", "refresh_token",
			"tokenHash", refreshTokenHash,
			"error", err,
		)
		return nil, apperrors.UnauthorizedError("invalid token", err)
	}

	// Check if user exists
	_, err = s.store.Users.GetByID(ctx, userID)
	if err != nil {
		return nil, err
	}

	// Generate new access token
	accessToken, err := s.authenticator.GenerateAccessToken(userID)
	if err != nil {
		s.logger.Error("failed to generate new access token during refresh",
			"operation", "refresh_token",
			"userID", userID,
			"error", err,
		)
		return nil, apperrors.InternalServerError(err)
	}

	// Log successful business operations
	s.logger.Info("token refreshed successfully",
		"userID", userID,
	)

	return accessToken, nil
}

func (s *AuthService) CreateRefreshToken(ctx context.Context, userID int, RemoteAddr string, userAgent string) (*models.Token, error) {
	refreshToken, err := s.authenticator.GenerateRefreshToken(userID)
	if err != nil {
		s.logger.Error("failed to generate refresh token",
			"operation", "create_refresh_token",
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
	ipAddress := strings.Split(RemoteAddr, ":")[0]

	err = s.store.Auth.CreateRefreshToken(ctx, nil, &models.RefreshToken{
		TokenHash: refreshTokenHash,
		UserID:    userID,
		IssuedAt:  refreshToken.IssuedAt,
		Revoked:   false,
		RevokedAt: nil,
		ExpiresAt: refreshToken.ExpiresAt,
		IPAddress: ipAddress,
		UserAgent: userAgent,
	})
	if err != nil {
		return nil, err
	}

	return refreshToken, nil
}

func (s *AuthService) Logout(ctx context.Context, refreshTokenString string) error {
	refreshTokenHash := auth.HashToken(refreshTokenString)
	err := s.store.Auth.MarkRefreshTokenAsRevoked(ctx, nil, refreshTokenHash)
	if err != nil {
		return err
	}

	// Log successful business operations
	s.logger.Info("user logged out successfully",
		"tokenHash", refreshTokenHash,
	)

	return nil
}

func (s *AuthService) ValidateToken(tokenString string, tokenType auth.TokenType) (*jwt.Token, error) {
	return s.authenticator.ValidateToken(tokenString, tokenType)
}

func (s *AuthService) GetUserIDFromToken(token *jwt.Token) (int, error) {
	return s.authenticator.GetUserIDFromToken(token)
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
