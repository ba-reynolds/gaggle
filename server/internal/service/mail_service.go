package service

import (
	"context"
	"log/slog"

	"github.com/ba-reynolds/gaggle/internal/models"
	"github.com/ba-reynolds/gaggle/internal/store"
)

type MailService struct {
	store  *store.Store
	logger *slog.Logger
}

func NewMailService(store *store.Store, logger *slog.Logger) *MailService {
	return &MailService{store: store, logger: logger}
}

func (s *MailService) Insert(ctx context.Context, m *models.MailMessage) (bool, error) {
	return s.store.Mail.Insert(ctx, m)
}

func (s *MailService) List(ctx context.Context, to string, limit int) ([]models.MailSummary, error) {
	return s.store.Mail.List(ctx, to, limit)
}

func (s *MailService) GetByID(ctx context.Context, id string) (*models.MailMessage, error) {
	return s.store.Mail.GetByID(ctx, id)
}
