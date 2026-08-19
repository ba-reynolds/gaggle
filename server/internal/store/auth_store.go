package store

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"

	"github.com/ba-reynolds/gaggle/internal/apperrors"
	"github.com/ba-reynolds/gaggle/internal/models"
	"github.com/google/uuid"
)

type authStore struct {
	db     *sql.DB
	logger *slog.Logger
}

func (store *authStore) CreateRefreshToken(ctx context.Context, tx *sql.Tx, refreshToken *models.RefreshToken) error {
	query := `
		INSERT INTO refresh_tokens (session_id, user_id, token_hash, issued_at, expires_at, revoked, revoked_at, user_agent, ip_address)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`

	exec := store.db.ExecContext
	if tx != nil {
		exec = tx.ExecContext
	}

	_, err := exec(ctx, query, refreshToken.SessionID, refreshToken.UserID, refreshToken.TokenHash, refreshToken.IssuedAt, refreshToken.ExpiresAt, refreshToken.Revoked, refreshToken.RevokedAt, refreshToken.UserAgent, refreshToken.IPAddress)
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
		SELECT refresh_token_id, session_id, user_id, token_hash, issued_at, expires_at, revoked, revoked_at, revoked_reason, user_agent, ip_address
		FROM refresh_tokens
		WHERE token_hash = $1
	`

	var refreshToken models.RefreshToken
	err := store.db.QueryRowContext(ctx, query, tokenHash).Scan(&refreshToken.RefreshTokenID, &refreshToken.SessionID, &refreshToken.UserID, &refreshToken.TokenHash, &refreshToken.IssuedAt, &refreshToken.ExpiresAt, &refreshToken.Revoked, &refreshToken.RevokedAt, &refreshToken.RevokedReason, &refreshToken.UserAgent, &refreshToken.IPAddress)
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

// RotateRefreshToken marks the presented token as revoked (reason=rotated) as
// part of issuing its successor. Runs on the caller's transaction so rotation
// is atomic: the new token row is only ever visible together with the old one
// being retired.
func (store *authStore) RotateRefreshToken(ctx context.Context, tx *sql.Tx, tokenHash string) error {
	query := `
		UPDATE refresh_tokens
		SET revoked = true, revoked_at = NOW(), revoked_reason = $2
		WHERE token_hash = $1 AND revoked = false
	`

	exec := store.db.ExecContext
	if tx != nil {
		exec = tx.ExecContext
	}

	result, err := exec(ctx, query, tokenHash, models.RevokedReasonRotated)
	if err != nil {
		store.logger.Error("database update failed",
			"operation", "rotate_refresh_token",
			"tokenHash", tokenHash,
			"query", query,
			"error", err,
		)
		return apperrors.InternalServerError(err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		store.logger.Error("failed to check rows affected",
			"operation", "rotate_refresh_token",
			"tokenHash", tokenHash,
			"error", err,
		)
		return apperrors.InternalServerError(err)
	}

	if rowsAffected == 0 {
		return apperrors.NotFoundError("refresh token not found", nil)
	}

	return nil
}

// RevokeSession revokes every token in a session family. Used for logout and
// for killing an entire chain when an already-rotated token is replayed.
func (store *authStore) RevokeSession(ctx context.Context, tx *sql.Tx, sessionID uuid.UUID, reason string) error {
	query := `
		UPDATE refresh_tokens
		SET revoked = true, revoked_at = COALESCE(revoked_at, NOW()), revoked_reason = $2
		WHERE session_id = $1 AND revoked = false
	`

	exec := store.db.ExecContext
	if tx != nil {
		exec = tx.ExecContext
	}

	_, err := exec(ctx, query, sessionID, reason)
	if err != nil {
		store.logger.Error("database update failed",
			"operation", "revoke_session",
			"sessionID", sessionID,
			"query", query,
			"error", err,
		)
		return apperrors.InternalServerError(err)
	}

	return nil
}

// SessionHasActiveToken reports whether any non-revoked token remains in the
// session family. Used to authenticate long-lived streams across rotations:
// an older-but-rotated token still proves the *session* is alive.
func (store *authStore) SessionHasActiveToken(ctx context.Context, sessionID uuid.UUID) (bool, error) {
	query := `
		SELECT EXISTS(
			SELECT 1 FROM refresh_tokens
			WHERE session_id = $1 AND revoked = false
		)
	`

	var active bool
	if err := store.db.QueryRowContext(ctx, query, sessionID).Scan(&active); err != nil {
		store.logger.Error("database query failed",
			"operation", "session_has_active_token",
			"sessionID", sessionID,
			"query", query,
			"error", err,
		)
		return false, apperrors.InternalServerError(err)
	}

	return active, nil
}