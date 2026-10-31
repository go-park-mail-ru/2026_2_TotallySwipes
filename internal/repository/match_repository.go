package repository

import (
	"context"
	"database/sql"
	"dating-app/internal/model"
	"errors"
	"fmt"
	"math"
)

type MatchRepository struct {
	db *sql.DB
}

func NewMatchRepository(db *sql.DB) *MatchRepository {
	return &MatchRepository{db: db}
}

// GetMatches - страница мэтчей пользователя от новых к старым и их общее число
func (r *MatchRepository) GetMatches(ctx context.Context, userID int64, limit int, cursor *int64) ([]model.Match, int, *int64, error) {
	beforeID := int64(math.MaxInt64)
	if cursor != nil {
		beforeID = *cursor
	}

	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if err != nil {
		return nil, 0, nil, fmt.Errorf("matches: begin read: %w", err)
	}
	defer tx.Rollback()

	var total int
	err = tx.QueryRowContext(ctx, countMatchesQuery, userID).Scan(&total)
	if err != nil {
		return nil, 0, nil, fmt.Errorf("matches: count: %w", err)
	}

	rows, err := tx.QueryContext(ctx, selectMatchesQuery, userID, beforeID, limit+1)
	if err != nil {
		return nil, 0, nil, fmt.Errorf("matches: select: %w", err)
	}
	defer rows.Close()

	matches := make([]model.Match, 0)
	for rows.Next() {
		var m model.Match
		ps := &m.Psycho
		if err := rows.Scan(&m.ID, &m.CreatedAt, &m.UserID, &m.Name, &m.BirthDate, &m.PhotoKey,
			&ps.Openness, &ps.Conscientiousness, &ps.Extraversion, &ps.Agreeableness, &ps.Neuroticism, &m.IsNew); err != nil {
			return nil, 0, nil, fmt.Errorf("matches: scan: %w", err)
		}
		matches = append(matches, m)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, nil, fmt.Errorf("matches: read: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, 0, nil, fmt.Errorf("matches: commit read: %w", err)
	}

	var nextCursor *int64
	if len(matches) > limit {
		matches = matches[:limit]
		id := matches[len(matches)-1].ID
		nextCursor = &id
	}
	return matches, total, nextCursor, nil
}

// MarkSeen отмечает мэтч просмотренным участником; повторная отметка не меняет время; model.ErrNotFound, если пользователь не в паре
func (r *MatchRepository) MarkSeen(ctx context.Context, userID, matchID int64) error {
	res, err := r.db.ExecContext(ctx, markMatchSeenQuery, userID, matchID)
	if err != nil {
		return fmt.Errorf("mark match id=%d seen: %w", matchID, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("mark match id=%d seen: rows affected: %w", matchID, err)
	}
	if n == 0 {
		return model.ErrNotFound
	}
	return nil
}

// GetSummary считает все мэтчи пользователя и ещё не открытые им
func (r *MatchRepository) GetSummary(ctx context.Context, userID int64) (*model.MatchSummary, error) {
	var s model.MatchSummary
	err := r.db.QueryRowContext(ctx, selectMatchSummaryQuery, userID).Scan(&s.Total, &s.New)
	if err != nil {
		return nil, fmt.Errorf("matches summary user id=%d: %w", userID, err)
	}
	return &s, nil
}

const getParticipantsQuery = `SELECT first_user_id, second_user_id
FROM match
WHERE id = $1 AND (first_user_id = $2 OR second_user_id = $2);`

func (r *MatchRepository) GetParticipants(ctx context.Context, userID, matchID int64) ([2]int64, error) {

	var partisipants [2]int64
	err := r.db.QueryRowContext(ctx, getParticipantsQuery, matchID, userID).Scan(&partisipants[0], &partisipants[1])

	if errors.Is(err, sql.ErrNoRows) {
		return [2]int64{}, model.ErrNotFound
	}

	if err != nil {
		return [2]int64{}, fmt.Errorf("get match participants: %w", err)
	}
	return partisipants, nil
}

const countMatchesQuery = `
		SELECT count(*) FROM match WHERE first_user_id = $1 OR second_user_id = $1`

const selectMatchesQuery = `
		SELECT m.id, m.created_at, partner.user_id, v.name, v.birth_date, ph.storage_key,
		       ps.openness, ps.conscientiousness, ps.extraversion, ps.agreeableness, ps.neuroticism,
		       CASE WHEN m.first_user_id = $1 THEN m.first_user_seen_at ELSE m.second_user_seen_at END IS NULL
		FROM match AS m
		CROSS JOIN LATERAL (
		    SELECT CASE WHEN m.first_user_id = $1 THEN m.second_user_id ELSE m.first_user_id END AS user_id
		) AS partner
		LEFT JOIN profile AS p ON p.user_id = partner.user_id
		LEFT JOIN LATERAL (
		    SELECT name, birth_date FROM profile_version WHERE profile_id = p.id ORDER BY revision DESC LIMIT 1
		) AS v ON true
		LEFT JOIN LATERAL (
		    SELECT * FROM profile_psycho WHERE profile_id = p.id ORDER BY revision DESC LIMIT 1
		) AS ps ON true
		LEFT JOIN LATERAL (
		    SELECT storage_key FROM photo WHERE profile_id = p.id ORDER BY position LIMIT 1
		) AS ph ON true
		WHERE (m.first_user_id = $1 OR m.second_user_id = $1) AND m.id < $2
		ORDER BY m.id DESC LIMIT $3`

const markMatchSeenQuery = `
		UPDATE match SET
		    first_user_seen_at = CASE WHEN first_user_id = $1 THEN COALESCE(first_user_seen_at, CURRENT_TIMESTAMP) ELSE first_user_seen_at END,
		    second_user_seen_at = CASE WHEN second_user_id = $1 THEN COALESCE(second_user_seen_at, CURRENT_TIMESTAMP) ELSE second_user_seen_at END,
		    updated_at = CURRENT_TIMESTAMP
		WHERE id = $2 AND (first_user_id = $1 OR second_user_id = $1)`

const selectMatchSummaryQuery = `
		SELECT count(*),
		       count(*) FILTER (WHERE CASE WHEN first_user_id = $1 THEN first_user_seen_at ELSE second_user_seen_at END IS NULL)
		FROM match WHERE first_user_id = $1 OR second_user_id = $1`
