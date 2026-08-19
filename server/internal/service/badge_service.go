package service

import (
	"context"
	"log/slog"

	"github.com/ba-reynolds/gaggle/internal/apperrors"
	"github.com/ba-reynolds/gaggle/internal/models"
	"github.com/ba-reynolds/gaggle/internal/store"
)

type BadgeService struct {
	store  *store.Store
	logger *slog.Logger
}

func NewBadgeService(store *store.Store, logger *slog.Logger) *BadgeService {
	return &BadgeService{store: store, logger: logger}
}

// GetBadgesForUsers returns badges keyed by user ID (admin-assigned + earned).
func (s *BadgeService) GetBadgesForUsers(ctx context.Context, ids []int) (map[int][]models.UserBadge, error) {
	return s.store.Badges.GetBadgesForUsers(ctx, ids)
}

// HydrateProfiles fills the Badges slice on a set of flat profile responses.
func (s *BadgeService) HydrateProfiles(ctx context.Context, profiles []models.UserProfileResponse) error {
	ids := make([]int, 0, len(profiles))
	for _, p := range profiles {
		if p.UserID != 0 {
			ids = append(ids, p.UserID)
		}
	}
	badges, err := s.GetBadgesForUsers(ctx, ids)
	if err != nil {
		return err
	}
	for i := range profiles {
		profiles[i].Badges = badges[profiles[i].UserID]
	}
	return nil
}

// HydrateUserWithProfile fills Badges on a single user-with-profile.
func (s *BadgeService) HydrateUserWithProfile(ctx context.Context, user *models.UserWithProfile) error {
	if user == nil {
		return nil
	}
	badges, err := s.GetBadgesForUsers(ctx, []int{user.ID})
	if err != nil {
		return err
	}
	user.Badges = badges[user.ID]
	return nil
}

// HydrateUserWithProfiles fills Badges on a set of user-with-profile records.
func (s *BadgeService) HydrateUserWithProfiles(ctx context.Context, users []models.UserWithProfile) error {
	ids := make([]int, 0, len(users))
	for _, u := range users {
		ids = append(ids, u.ID)
	}
	badges, err := s.GetBadgesForUsers(ctx, ids)
	if err != nil {
		return err
	}
	for i := range users {
		users[i].Badges = badges[users[i].ID]
	}
	return nil
}

// ListCatalog returns every badge definition.
func (s *BadgeService) ListCatalog(ctx context.Context) ([]models.Badge, error) {
	return s.store.Badges.ListCatalog(ctx)
}

// CreateBadge registers a new admin-assigned badge.
func (s *BadgeService) CreateBadge(ctx context.Context, payload models.CreateBadgePayload) (*models.Badge, error) {
	return s.store.Badges.CreateBadge(ctx, payload)
}

// UpdateBadge edits an admin-assigned badge.
func (s *BadgeService) UpdateBadge(ctx context.Context, badgeID int, payload models.CreateBadgePayload) (*models.Badge, error) {
	return s.store.Badges.UpdateBadge(ctx, badgeID, payload)
}

// DeleteBadge removes an admin-assigned badge not held by any user.
func (s *BadgeService) DeleteBadge(ctx context.Context, badgeID int) error {
	return s.store.Badges.DeleteBadge(ctx, badgeID)
}

// GrantBadge awards an admin badge to a user.
func (s *BadgeService) GrantBadge(ctx context.Context, username string, badgeID, grantedBy int) error {
	user, err := s.store.Users.GetByUsername(ctx, username)
	if err != nil {
		return err
	}
	if user.SoftDeleted {
		return apperrors.NotFoundError("user not found", nil)
	}
	return s.store.Badges.GrantBadge(ctx, user.ID, badgeID, grantedBy)
}

// RevokeBadge removes an admin badge from a user.
func (s *BadgeService) RevokeBadge(ctx context.Context, username string, badgeID int) error {
	user, err := s.store.Users.GetByUsername(ctx, username)
	if err != nil {
		return err
	}
	if user.SoftDeleted {
		return apperrors.NotFoundError("user not found", nil)
	}
	return s.store.Badges.RevokeBadge(ctx, user.ID, badgeID)
}
