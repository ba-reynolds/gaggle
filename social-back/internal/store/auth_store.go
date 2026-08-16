package store

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"

	"github.com/ba-reynolds/vitrilium/internal/apperrors"
	"github.com/ba-reynolds/vitrilium/internal/models"
)

type authStore struct {
	db     *sql.DB
	logger *slog.Logger
}

func (store *authStore) CreateRefreshToken(ctx context.Context, tx *sql.Tx, refreshToken *models.RefreshToken) error {
	query := `
		INSERT INTO refresh_tokens (user_id, token_hash, issued_at, expires_at, revoked, revoked_at, user_agent, ip_address)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`

	exec := store.db.ExecContext
	if tx != nil {
		exec = tx.ExecContext
	}

	_, err := exec(ctx, query, refreshToken.UserID, refreshToken.TokenHash, refreshToken.IssuedAt, refreshToken.ExpiresAt, refreshToken.Revoked, refreshToken.RevokedAt, refreshToken.UserAgent, refreshToken.IPAddress)
	if err != nil {
		// Log database insert errors with full context
		store.logger.Error("database insert failed",
			"operation", "create_refresh_token",
			"userID", refreshToken.UserID,
			"query", query,
			"error", err,
		)
		return apperrors.InternalServerError(err)
	}

	return nil
}

func (store *authStore) GetRefreshToken(ctx context.Context, tokenHash string) (*models.RefreshToken, error) {
	query := `
		SELECT refresh_token_id, user_id, token_hash, issued_at, expires_at, revoked, revoked_at, user_agent, ip_address
		FROM refresh_tokens
		WHERE token_hash = $1
	`

	var refreshToken models.RefreshToken
	err := store.db.QueryRowContext(ctx, query, tokenHash).Scan(&refreshToken.RefreshTokenID, &refreshToken.UserID, &refreshToken.TokenHash, &refreshToken.IssuedAt, &refreshToken.ExpiresAt, &refreshToken.Revoked, &refreshToken.RevokedAt, &refreshToken.UserAgent, &refreshToken.IPAddress)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			// Don't log - this is expected behavior, not an error
			return nil, apperrors.NotFoundError("refresh token not found", err)
		}
		// Log actual database errors with full context
		store.logger.Error("database query failed",
			"operation", "get_refresh_token",
			"tokenHash", tokenHash,
			"query", query,
			"error", err,
		)
		return nil, apperrors.InternalServerError(err)
	}

	return &refreshToken, nil
}

func (store *authStore) MarkRefreshTokenAsRevoked(ctx context.Context, tx *sql.Tx, tokenHash string) error {
	query := `
		UPDATE refresh_tokens SET revoked = true, revoked_at = NOW() WHERE token_hash = $1
	`

	exec := store.db.ExecContext
	if tx != nil {
		exec = tx.ExecContext
	}

	result, err := exec(ctx, query, tokenHash)
	if err != nil {
		// Log database update errors with full context
		store.logger.Error("database update failed",
			"operation", "mark_refresh_token_as_revoked",
			"tokenHash", tokenHash,
			"query", query,
			"error", err,
		)
		return apperrors.InternalServerError(err)
	}

	// Check if any rows were actually updated
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		// Log errors checking rows affected
		store.logger.Error("failed to check rows affected",
			"operation", "mark_refresh_token_as_revoked",
			"tokenHash", tokenHash,
			"error", err,
		)
		return apperrors.InternalServerError(err)
	}

	if rowsAffected == 0 {
		// Log when no rows were updated (could indicate token not found)
		store.logger.Warn("no rows updated",
			"operation", "mark_refresh_token_as_revoked",
			"tokenHash", tokenHash,
		)
		return apperrors.NotFoundError("refresh token not found", nil)
	}

	return nil
}
