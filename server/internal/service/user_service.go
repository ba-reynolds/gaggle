package service

import (
	"context"
	"log/slog"

	"github.com/ba-reynolds/gophersocial/internal/apperrors"
	"github.com/ba-reynolds/gophersocial/internal/models"
	"github.com/ba-reynolds/gophersocial/internal/store"
)

type UserService struct {
	store  *store.Store
	logger *slog.Logger
}

// GetUserByID retrieves a user by their ID
func (s *UserService) GetByID(ctx context.Context, id int) (*models.User, error) {
	user, err := s.store.Users.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if user.SoftDeleted {
		s.logger.Debug("attempted to access soft deleted user",
			"userID", id,
			"operation", "get_by_id",
		)
		return nil, apperrors.NotFoundError("user not found", nil)
	}

	return user, nil
}

// GetUserByEmail retrieves a user by their email address
func (s *UserService) GetByEmail(ctx context.Context, email string) (*models.User, error) {
	user, err := s.store.Users.GetByEmail(ctx, email)
	if err != nil {
		return nil, err
	}

	if user.SoftDeleted {
		s.logger.Debug("attempted to access soft deleted user",
			"email", email,
			"operation", "get_by_email",
		)
		return nil, apperrors.NotFoundError("user not found", nil)
	}

	return user, nil
}

// GetUserByUsername retrieves a user by their username
func (s *UserService) GetByUsername(ctx context.Context, username string) (*models.User, error) {
	user, err := s.store.Users.GetByUsername(ctx, username)
	if err != nil {
		return nil, err
	}

	if user.SoftDeleted {
		s.logger.Debug("attempted to access soft deleted user",
			"username", username,
			"operation", "get_by_username",
		)
		return nil, apperrors.NotFoundError("user not found", nil)
	}

	return user, nil
}

// CreateUser creates a new user in the database
func (s *UserService) CreateUser(ctx context.Context, user *models.User) error {
	// Start a transaction for user creation
	tx, err := s.store.DB.BeginTx(ctx, nil)
	if err != nil {
		// Log transaction errors - these are service layer concerns
		s.logger.Error("failed to begin transaction",
			"operation", "create_user",
			"username", user.Username,
			"email", user.Email,
			"error", err,
		)
		return apperrors.InternalServerError(err)
	}
	defer tx.Rollback()

	// Create the user
	err = s.store.Users.Create(ctx, tx, user)
	if err != nil {
		// Storage layer already logged the actual database error
		// Just add service-level context for business logic
		if appErr, ok := err.(*apperrors.AppError); ok && appErr.Code == apperrors.AlreadyExists {
			s.logger.Info("user creation failed due to duplicate",
				"username", user.Username,
				"email", user.Email,
			)
		}
		return err
	}

	// Commit the transaction
	if err := tx.Commit(); err != nil {
		// Log transaction commit errors - these are service layer concerns
		s.logger.Error("failed to commit transaction",
			"operation", "create_user",
			"username", user.Username,
			"userID", user.ID,
			"error", err,
		)
		return apperrors.InternalServerError(err)
	}

	// Log successful business operations
	s.logger.Info("user created successfully",
		"username", user.Username,
		"userID", user.ID,
		"email", user.Email,
	)

	return nil
}

func (s *UserService) UpdateUserProfile(ctx context.Context, user *models.UserWithProfile) error {
	// Start a transaction for user profile update
	tx, err := s.store.DB.BeginTx(ctx, nil)
	if err != nil {
		// Log transaction errors - these are service layer concerns
		s.logger.Error("failed to begin transaction",
			"operation", "update_user_profile",
			"userID", user.ID,
			"username", user.Username,
			"error", err,
		)
		return apperrors.InternalServerError(err)
	}
	defer tx.Rollback()

	// Update the user profile
	if err := s.store.Users.UpdateUserProfile(ctx, tx, user); err != nil {
		// Storage layer already logged the actual database error
		// Just handle business logic
		return err
	}

	// Commit the transaction
	if err := tx.Commit(); err != nil {
		// Log transaction commit errors - these are service layer concerns
		s.logger.Error("failed to commit transaction",
			"operation", "update_user_profile",
			"userID", user.ID,
			"username", user.Username,
			"error", err,
		)
		return apperrors.InternalServerError(err)
	}

	// Log successful business operations
	s.logger.Info("user profile updated successfully",
		"userID", user.ID,
		"username", user.Username,
		"display_name", user.Profile.DisplayName,
	)

	return nil
}

func (s *UserService) GetUserProfileByUsername(ctx context.Context, username string) (*models.UserWithProfile, error) {
	user, err := s.store.Users.GetUserProfileByUsername(ctx, username)
	if err != nil {
		// Don't log here - storage layer already logged database errors
		// Just handle business logic
		return nil, err
	}

	if user.SoftDeleted {
		// Log business logic decisions for debugging
		s.logger.Debug("attempted to access soft deleted user profile",
			"username", username,
			"userID", user.ID,
			"operation", "get_user_profile_by_username",
		)
		return nil, apperrors.NotFoundError("user not found", nil)
	}

	return user, nil
}

func (s *UserService) GetSettings(ctx context.Context, userID int) (*models.UserSettings, error) {
	return s.store.Users.GetSettings(ctx, userID)
}

func (s *UserService) UpdateSettings(ctx context.Context, userID int, settings *models.UserSettings) error {
	return s.store.Users.UpdateSettings(ctx, userID, settings)
}

// Suggested returns accounts the viewer might want to follow.
func (s *UserService) Suggested(ctx context.Context, viewerID int, limit int) (*models.UserList, error) {
	return s.store.Users.Suggested(ctx, viewerID, validateLimit(limit, 20, 100))
}
