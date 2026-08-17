package store

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"strings"
	"time"

	"github.com/ba-reynolds/gophersocial/internal/apperrors"
	"github.com/ba-reynolds/gophersocial/internal/models"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/lib/pq"
)

type pollStore struct {
	db     *sql.DB
	logger *slog.Logger
}

func (s *pollStore) Create(ctx context.Context, tx *sql.Tx, postID int, payload *models.CreatePollPayload) error {
	row := s.db.QueryRowContext
	exec := s.db.ExecContext
	if tx != nil {
		row = tx.QueryRowContext
		exec = tx.ExecContext
	}
	var pollID int
	if err := row(ctx, `INSERT INTO polls (post_id, question, ends_at) VALUES ($1, $2, $3) RETURNING poll_id`, postID, payload.Question, payload.EndsAt).Scan(&pollID); err != nil {
		return apperrors.InternalServerError(err)
	}
	for index, option := range payload.Options {
		if _, err := exec(ctx, `INSERT INTO poll_options (poll_id, label, position) VALUES ($1, $2, $3)`, pollID, strings.TrimSpace(option), index+1); err != nil {
			return apperrors.InternalServerError(err)
		}
	}
	return nil
}

func (s *pollStore) GetForPost(ctx context.Context, postID, viewerID int) (*models.Poll, error) {
	var poll models.Poll
	var pollID int
	if err := s.db.QueryRowContext(ctx, `SELECT poll_id, question, ends_at FROM polls WHERE post_id = $1`, postID).Scan(&pollID, &poll.Question, &poll.EndsAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, apperrors.InternalServerError(err)
	}
	poll.ID = pollID
	rows, err := s.db.QueryContext(ctx, `
		SELECT o.option_id, o.label, o.position, COUNT(v.user_id)::int
		FROM poll_options o
		LEFT JOIN poll_votes v ON v.option_id = o.option_id
		WHERE o.poll_id = $1
		GROUP BY o.option_id, o.label, o.position
		ORDER BY o.position`, pollID)
	if err != nil {
		return nil, apperrors.InternalServerError(err)
	}
	defer rows.Close()
	for rows.Next() {
		var option models.PollOption
		if err := rows.Scan(&option.ID, &option.Label, &option.Position, &option.VoteCount); err != nil {
			return nil, apperrors.InternalServerError(err)
		}
		poll.TotalVotes += option.VoteCount
		poll.Options = append(poll.Options, option)
	}
	if err := rows.Err(); err != nil {
		return nil, apperrors.InternalServerError(err)
	}
	_ = s.db.QueryRowContext(ctx, `SELECT option_id FROM poll_votes WHERE poll_id = $1 AND user_id = $2`, pollID, viewerID).Scan(&poll.SelectedOptionID)
	poll.Closed = poll.EndsAt != nil && time.Now().After(*poll.EndsAt)
	return &poll, nil
}

func (s *pollStore) GetForPosts(ctx context.Context, postIDs []int, viewerID int) (map[int]*models.Poll, error) {
	result := make(map[int]*models.Poll)
	if len(postIDs) == 0 {
		return result, nil
	}

	rows, err := s.db.QueryContext(ctx, `SELECT poll_id, post_id, question, ends_at FROM polls WHERE post_id = ANY($1)`, pq.Array(postIDs))
	if err != nil {
		return nil, apperrors.InternalServerError(err)
	}
	byPollID := make(map[int]*models.Poll)
	var pollIDs []int
	for rows.Next() {
		var id, postID int
		var poll models.Poll
		if err := rows.Scan(&id, &postID, &poll.Question, &poll.EndsAt); err != nil {
			rows.Close()
			return nil, apperrors.InternalServerError(err)
		}
		poll.ID = id
		byPollID[id] = &poll
		result[postID] = &poll
		pollIDs = append(pollIDs, id)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, apperrors.InternalServerError(err)
	}
	if len(pollIDs) == 0 {
		return result, nil
	}

	optRows, err := s.db.QueryContext(ctx, `
		SELECT o.poll_id, o.option_id, o.label, o.position, COUNT(v.user_id)::int
		FROM poll_options o
		LEFT JOIN poll_votes v ON v.option_id = o.option_id
		WHERE o.poll_id = ANY($1)
		GROUP BY o.poll_id, o.option_id, o.label, o.position
		ORDER BY o.poll_id, o.position`, pq.Array(pollIDs))
	if err != nil {
		return nil, apperrors.InternalServerError(err)
	}
	defer optRows.Close()
	for optRows.Next() {
		var pollID, optionID, position, voteCount int
		var label string
		if err := optRows.Scan(&pollID, &optionID, &label, &position, &voteCount); err != nil {
			return nil, apperrors.InternalServerError(err)
		}
		poll := byPollID[pollID]
		poll.TotalVotes += voteCount
		poll.Options = append(poll.Options, models.PollOption{ID: optionID, Label: label, Position: position, VoteCount: voteCount})
	}
	if err := optRows.Err(); err != nil {
		return nil, apperrors.InternalServerError(err)
	}

	selRows, err := s.db.QueryContext(ctx, `SELECT poll_id, option_id FROM poll_votes WHERE poll_id = ANY($1) AND user_id = $2`, pq.Array(pollIDs), viewerID)
	if err != nil {
		return nil, apperrors.InternalServerError(err)
	}
	defer selRows.Close()
	for selRows.Next() {
		var pollID, optionID int
		if err := selRows.Scan(&pollID, &optionID); err != nil {
			return nil, apperrors.InternalServerError(err)
		}
		if poll := byPollID[pollID]; poll != nil {
			poll.SelectedOptionID = &optionID
		}
	}
	if err := selRows.Err(); err != nil {
		return nil, apperrors.InternalServerError(err)
	}

	for _, poll := range byPollID {
		poll.Closed = poll.EndsAt != nil && time.Now().After(*poll.EndsAt)
	}
	return result, nil
}

func (s *pollStore) Vote(ctx context.Context, tx *sql.Tx, postID, optionID, userID int) error {
	var pollID int
	var endsAt *time.Time
	if err := tx.QueryRowContext(ctx, `SELECT poll_id, ends_at FROM polls WHERE post_id = $1`, postID).Scan(&pollID, &endsAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return apperrors.NotFoundError("poll not found", err)
		}
		return apperrors.InternalServerError(err)
	}
	if endsAt != nil && time.Now().After(*endsAt) {
		return apperrors.BadRequestError("poll has ended", nil)
	}
	var optionPollID int
	if err := tx.QueryRowContext(ctx, `SELECT poll_id FROM poll_options WHERE option_id = $1`, optionID).Scan(&optionPollID); err != nil || optionPollID != pollID {
		return apperrors.BadRequestError("invalid poll option", err)
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO poll_votes (poll_id, option_id, user_id) VALUES ($1, $2, $3)`, pollID, optionID, userID); err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return apperrors.AlreadyExistsError("user has already voted in this poll", err)
		}
		return apperrors.InternalServerError(err)
	}
	return nil
}
