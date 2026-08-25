package store

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"strconv"
	"strings"

	"github.com/ba-reynolds/gaggle/internal/apperrors"
	"github.com/ba-reynolds/gaggle/internal/models"
)

type mailStore struct {
	db     *sql.DB
	logger *slog.Logger
}

// Insert stores one inbound mail and reports whether the row was inserted.
// false means a mail with the same Message-ID was already stored — Cloudflare
// delivery is at-least-once, so duplicates are normal and dropped silently.
func (s *mailStore) Insert(ctx context.Context, m *models.MailMessage) (bool, error) {
	tag, err := s.db.ExecContext(ctx, `
		INSERT INTO mail_messages (id, ts, from_addr, to_addr, subject, body, message_id)
		VALUES ($1, $2, $3, $4, $5, $6, NULLIF($7, ''))
		ON CONFLICT (message_id) DO NOTHING`,
		m.ID, m.TS, m.FromAddr, m.ToAddr, m.Subject, m.Body, m.MessageID)
	if err != nil {
		return false, apperrors.InternalServerError(err)
	}
	rows, rerr := tag.RowsAffected()
	if rerr != nil {
		return false, apperrors.InternalServerError(rerr)
	}
	return rows == 1, nil
}

// List returns mail summaries (no body), newest first. `to` is a
// case-insensitive substring match against to_addr; empty means no filter.
func (s *mailStore) List(ctx context.Context, to string, limit int) ([]models.MailSummary, error) {
	query := strings.Builder{}
	query.WriteString(`SELECT id, ts, from_addr, to_addr, subject FROM mail_messages`)
	args := []any{}
	if strings.TrimSpace(to) != "" {
		query.WriteString(` WHERE to_addr ILIKE '%' || $1 || '%' ESCAPE '\'`)
		args = append(args, escapeLikePattern(to))
	}
	args = append(args, limit)
	query.WriteString(` ORDER BY ts DESC LIMIT $` + strconv.Itoa(len(args)))

	rows, err := s.db.QueryContext(ctx, query.String(), args...)
	if err != nil {
		return nil, apperrors.InternalServerError(err)
	}
	defer rows.Close()

	mails := []models.MailSummary{}
	for rows.Next() {
		var m models.MailSummary
		if err := rows.Scan(&m.ID, &m.TS, &m.FromAddr, &m.ToAddr, &m.Subject); err != nil {
			return nil, apperrors.InternalServerError(err)
		}
		mails = append(mails, m)
	}
	if err := rows.Err(); err != nil {
		return nil, apperrors.InternalServerError(err)
	}
	return mails, nil
}

func (s *mailStore) GetByID(ctx context.Context, id string) (*models.MailMessage, error) {
	var m models.MailMessage
	err := s.db.QueryRowContext(ctx, `
		SELECT id, ts, from_addr, to_addr, subject, body
		FROM mail_messages WHERE id = $1`, id).
		Scan(&m.ID, &m.TS, &m.FromAddr, &m.ToAddr, &m.Subject, &m.Body)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, apperrors.NotFoundError("mail not found", err)
		}
		return nil, apperrors.InternalServerError(err)
	}
	return &m, nil
}

// escapeLikePattern escapes LIKE/ILIKE metacharacters so the caller-supplied
// `to` substring is matched literally (same escaping as the post search path).
func escapeLikePattern(s string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(s)
}
