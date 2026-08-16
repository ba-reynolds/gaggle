package service

import (
	"context"
	"log/slog"

	"github.com/ba-reynolds/vitrilium/internal/apperrors"
	"github.com/ba-reynolds/vitrilium/internal/models"
	"github.com/ba-reynolds/vitrilium/internal/store"
)

type UserRelationshipService struct {
	store  *store.Store
	logger *slog.Logger
}

// CreateRelationship creates a new relationship between two users
func (s *UserRelationshipService) CreateRelationship(ctx context.Context, followerID, followingID int, relationshipType string) (*models.UserRelationship, error) {
	// Validate that users can't follow/block themselves
	if followerID == followingID {
		s.logger.Warn("attempted to create self-relationship",
			"follower_id", followerID,
			"following_id", followingID,
			"relationship_type", relationshipType,
		)
		return nil, apperrors.BadRequestError("cannot create relationship with yourself", nil)
	}

	// Check if the target user exists and is not soft deleted
	targetUser, err := s.store.Users.GetByID(ctx, followingID)
	if err != nil {
		// Don't log here - storage layer already logged database errors
		return nil, err
	}

	if targetUser.SoftDeleted {
		s.logger.Debug("attempted to create relationship with soft deleted user",
			"follower_id", followerID,
			"following_id", followingID,
			"relationship_type", relationshipType,
		)
		return nil, apperrors.NotFoundError("user not found", nil)
	}

	// Start a transaction for relationship creation
	tx, err := s.store.DB.BeginTx(ctx, nil)
	if err != nil {
		s.logger.Error("failed to begin transaction",
			"operation", "create_relationship",
			"follower_id", followerID,
			"following_id", followingID,
			"relationship_type", relationshipType,
			"error", err,
		)
		return nil, apperrors.InternalServerError(err)
	}
	defer tx.Rollback()

	var relationship *models.UserRelationship

	if relationshipType == "block" {
		// When blocking, remove any existing relationships in both directions
		// This ensures the blocked user can't follow you anymore
		if err := s.store.UserRelationships.Delete(ctx, tx, followerID, followingID); err != nil && !apperrors.Is(err, apperrors.NotFound) {
			return nil, err
		}
		if err := s.store.UserRelationships.Delete(ctx, tx, followingID, followerID); err != nil && !apperrors.Is(err, apperrors.NotFound) {
			return nil, err
		}

		// Create the block relationship
		relationship = &models.UserRelationship{
			FollowerID:       followerID,
			FollowingID:      followingID,
			RelationshipType: relationshipType,
		}
		if err := s.store.UserRelationships.Create(ctx, tx, relationship); err != nil {
			return nil, err
		}
	} else if relationshipType == "follow" {
		// When following, check if the target user has blocked you
		// If they have blocked you, you can't follow them
		blockedByTarget, err := s.store.UserRelationships.GetByIDs(ctx, followingID, followerID)
		if err != nil && !apperrors.Is(err, apperrors.NotFound) {
			return nil, err
		}
		if blockedByTarget != nil && blockedByTarget.RelationshipType == "block" {
			s.logger.Warn("attempted to follow user who has blocked you",
				"follower_id", followerID,
				"following_id", followingID,
			)
			return nil, apperrors.ForbiddenError("cannot follow this user", nil)
		}

		// Check if relationship already exists
		existingRelationship, err := s.store.UserRelationships.GetByIDs(ctx, followerID, followingID)
		if err != nil && !apperrors.Is(err, apperrors.NotFound) {
			return nil, err
		}

		if existingRelationship != nil {
			// Update existing relationship to follow
			existingRelationship.RelationshipType = relationshipType
			if err := s.store.UserRelationships.Update(ctx, tx, existingRelationship); err != nil {
				return nil, err
			}
			relationship = existingRelationship
		} else {
			// Create new follow relationship
			relationship = &models.UserRelationship{
				FollowerID:       followerID,
				FollowingID:      followingID,
				RelationshipType: relationshipType,
			}
			if err := s.store.UserRelationships.Create(ctx, tx, relationship); err != nil {
				return nil, err
			}
		}
	}

	// Commit the transaction
	if err := tx.Commit(); err != nil {
		s.logger.Error("failed to commit transaction",
			"operation", "create_relationship",
			"follower_id", followerID,
			"following_id", followingID,
			"relationship_type", relationshipType,
			"error", err,
		)
		return nil, apperrors.InternalServerError(err)
	}

	// Log successful business operations
	s.logger.Info("user relationship created/updated successfully",
		"follower_id", followerID,
		"following_id", followingID,
		"relationship_type", relationshipType,
		"relationship_id", relationship.RelationshipID,
	)

	return relationship, nil
}

// DeleteRelationship deletes a relationship between two users
func (s *UserRelationshipService) DeleteRelationship(ctx context.Context, followerID, followingID int) error {
	// Start a transaction for relationship deletion
	tx, err := s.store.DB.BeginTx(ctx, nil)
	if err != nil {
		s.logger.Error("failed to begin transaction",
			"operation", "delete_relationship",
			"follower_id", followerID,
			"following_id", followingID,
			"error", err,
		)
		return apperrors.InternalServerError(err)
	}
	defer tx.Rollback()

	// Delete the relationship
	if err := s.store.UserRelationships.Delete(ctx, tx, followerID, followingID); err != nil {
		// Storage layer already logged the actual database error
		return err
	}

	// Commit the transaction
	if err := tx.Commit(); err != nil {
		s.logger.Error("failed to commit transaction",
			"operation", "delete_relationship",
			"follower_id", followerID,
			"following_id", followingID,
			"error", err,
		)
		return apperrors.InternalServerError(err)
	}

	// Log successful business operations
	s.logger.Info("user relationship deleted successfully",
		"follower_id", followerID,
		"following_id", followingID,
	)

	return nil
}

// GetFollowers retrieves a paginated list of followers for a user
func (s *UserRelationshipService) GetFollowers(ctx context.Context, userID int, limit int, cursor string) (*models.UserFollowersResponse, error) {
	// Validate pagination parameters
	if limit <= 0 || limit > 100 {
		limit = 20 // Default limit
	}

	// Check if the user exists and is not soft deleted
	user, err := s.store.Users.GetByID(ctx, userID)
	if err != nil {
		// Don't log here - storage layer already logged database errors
		return nil, err
	}

	if user.SoftDeleted {
		s.logger.Debug("attempted to get followers for soft deleted user",
			"user_id", userID,
		)
		return nil, apperrors.NotFoundError("user not found", nil)
	}

	// Get followers
	followers, err := s.store.UserRelationships.GetFollowers(ctx, userID, limit, cursor)
	if err != nil {
		// Storage layer already logged the actual database error
		return nil, err
	}

	return followers, nil
}

// GetFollowing retrieves a paginated list of users that a user is following
func (s *UserRelationshipService) GetFollowing(ctx context.Context, userID int, limit int, cursor string) (*models.UserFollowingResponse, error) {
	// Validate pagination parameters
	if limit <= 0 || limit > 100 {
		limit = 20 // Default limit
	}

	// Check if the user exists and is not soft deleted
	user, err := s.store.Users.GetByID(ctx, userID)
	if err != nil {
		// Don't log here - storage layer already logged database errors
		return nil, err
	}

	if user.SoftDeleted {
		s.logger.Debug("attempted to get following for soft deleted user",
			"user_id", userID,
		)
		return nil, apperrors.NotFoundError("user not found", nil)
	}

	// Get following users
	following, err := s.store.UserRelationships.GetFollowing(ctx, userID, limit, cursor)
	if err != nil {
		// Storage layer already logged the actual database error
		return nil, err
	}

	return following, nil
}

// GetRelationshipStatus gets the relationship status between two users
func (s *UserRelationshipService) GetRelationshipStatus(ctx context.Context, followerID, followingID int) (*models.RelationshipStatus, error) {
	// Check if both users exist and are not soft deleted
	follower, err := s.store.Users.GetByID(ctx, followerID)
	if err != nil {
		// Don't log here - storage layer already logged database errors
		return nil, err
	}

	if follower.SoftDeleted {
		s.logger.Debug("attempted to get relationship status for soft deleted follower",
			"follower_id", followerID,
			"following_id", followingID,
		)
		return nil, apperrors.NotFoundError("user not found", nil)
	}

	following, err := s.store.Users.GetByID(ctx, followingID)
	if err != nil {
		// Don't log here - storage layer already logged database errors
		return nil, err
	}

	if following.SoftDeleted {
		s.logger.Debug("attempted to get relationship status for soft deleted following user",
			"follower_id", followerID,
			"following_id", followingID,
		)
		return nil, apperrors.NotFoundError("user not found", nil)
	}

	// Get relationship status
	status, err := s.store.UserRelationships.GetRelationshipStatus(ctx, followerID, followingID)
	if err != nil {
		// Storage layer already logged the actual database error
		return nil, err
	}

	return status, nil
}

func (s *UserRelationshipService) GetFollowerIDs(ctx context.Context, userID int) ([]int, error) {
	return s.store.UserRelationships.GetFollowerIDs(ctx, userID)
}
