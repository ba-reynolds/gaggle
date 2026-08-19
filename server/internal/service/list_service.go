package service

import (
	"context"
	"log/slog"
	"strings"

	"github.com/ba-reynolds/gaggle/internal/apperrors"
	"github.com/ba-reynolds/gaggle/internal/models"
	"github.com/ba-reynolds/gaggle/internal/store"
)

type ListService struct {
	store  *store.Store
	logger *slog.Logger
}

func NewListService(store *store.Store, logger *slog.Logger) *ListService {
	return &ListService{store: store, logger: logger}
}

// Create validates the payload and creates a list for the owner.
func (s *ListService) Create(ctx context.Context, ownerID int, payload models.CreateListPayload) (*models.List, error) {
	name, description, err := validateListPayload(payload)
	if err != nil {
		return nil, err
	}
	list := &models.List{OwnerID: ownerID, Name: name, Description: description}
	if err := s.store.Lists.Create(ctx, list); err != nil {
		return nil, err
	}
	return list, nil
}

// Update edits a list's name/description (owner only).
func (s *ListService) Update(ctx context.Context, listID, actorID int, payload models.CreateListPayload) (*models.List, error) {
	list, err := s.store.Lists.GetByID(ctx, listID)
	if err != nil {
		return nil, err
	}
	if list.OwnerID != actorID {
		return nil, apperrors.ForbiddenError("only the list owner can edit this list", nil)
	}
	name, description, err := validateListPayload(payload)
	if err != nil {
		return nil, err
	}
	updated, err := s.store.Lists.Update(ctx, listID, name, description)
	if err != nil {
		return nil, err
	}
	return updated, nil
}

func validateListPayload(payload models.CreateListPayload) (string, string, error) {
	name := strings.TrimSpace(payload.Name)
	description := strings.TrimSpace(payload.Description)
	if name == "" || len(name) > 100 {
		return "", "", apperrors.BadRequestError("list name is required (max 100 characters)", nil)
	}
	if len(description) > 300 {
		return "", "", apperrors.BadRequestError("list description must be at most 300 characters", nil)
	}
	return name, description, nil
}

// Get returns a public list by id.
func (s *ListService) Get(ctx context.Context, listID int) (*models.List, error) {
	return s.store.Lists.GetByID(ctx, listID)
}

// ListForUser returns a user's public lists.
func (s *ListService) ListForUser(ctx context.Context, userID int) ([]models.List, error) {
	return s.store.Lists.ListByOwner(ctx, userID)
}

// Delete removes a list if the actor owns it.
func (s *ListService) Delete(ctx context.Context, listID, actorID int) error {
	list, err := s.store.Lists.GetByID(ctx, listID)
	if err != nil {
		return err
	}
	if list.OwnerID != actorID {
		return apperrors.ForbiddenError("only the list owner can delete this list", nil)
	}
	return s.store.Lists.Delete(ctx, listID, actorID)
}

// AddMember adds a user to a list (owner-only).
func (s *ListService) AddMember(ctx context.Context, listID, actorID, memberID int) error {
	if err := s.requireOwner(ctx, listID, actorID); err != nil {
		return err
	}
	return s.store.Lists.AddMember(ctx, listID, memberID)
}

// RemoveMember removes a user from a list (owner-only).
func (s *ListService) RemoveMember(ctx context.Context, listID, actorID, memberID int) error {
	if err := s.requireOwner(ctx, listID, actorID); err != nil {
		return err
	}
	return s.store.Lists.RemoveMember(ctx, listID, memberID)
}

// GetMembers returns the users in a public list.
func (s *ListService) GetMembers(ctx context.Context, listID, limit int, cursor string) (*models.ListMembersResponse, error) {
	return s.store.Lists.GetMembers(ctx, listID, limit, cursor)
}

// GetListFeed returns top-level posts from users in a public list, hydrated
// with media, engagement, and polls (mirrors the home-feed hydration chain).
func (s *ListService) GetListFeed(ctx context.Context, viewerID, listID, limit int, cursor string) (*models.PostFeed, error) {
	if _, err := s.store.Lists.GetByID(ctx, listID); err != nil {
		return nil, err
	}
	normalizedLimit := validateLimit(limit, 20, 100)

	feed, err := s.store.Posts.GetListFeed(ctx, listID, normalizedLimit, cursor)
	if err != nil {
		return nil, err
	}
	if err := s.store.Media.FetchPostMedia(ctx, feed.Items); err != nil {
		return nil, err
	}
	if err := hydrateEngagement(ctx, s.store, s.logger, feed.Items, viewerID); err != nil {
		return nil, err
	}
	if err := hydratePolls(ctx, s.store, feed.Items, viewerID); err != nil {
		return nil, err
	}
	if err := hydrateNews(ctx, s.store, feed.Items); err != nil {
		return nil, err
	}
	if err := hydrateParents(ctx, s.store, feed.Items); err != nil {
		return nil, err
	}
	feed.Items, err = filterVisiblePosts(ctx, s.store, feed.Items, viewerID)
	if err != nil {
		return nil, err
	}
	return feed, nil
}

func (s *ListService) requireOwner(ctx context.Context, listID, actorID int) error {
	list, err := s.store.Lists.GetByID(ctx, listID)
	if err != nil {
		return err
	}
	if list.OwnerID != actorID {
		return apperrors.ForbiddenError("only the list owner can edit this list", nil)
	}
	return nil
}
