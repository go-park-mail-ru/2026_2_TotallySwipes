package repository

import (
	"context"
	"database/sql"
	"dating-app/internal/model"
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
	err = tx.QueryRowContext(ctx, `
		SELECT count(*) FROM match WHERE first_user_id = $1 OR second_user_id = $1`, userID).Scan(&total)
	if err != nil {
		return nil, 0, nil, fmt.Errorf("matches: count: %w", err)
	}

	rows, err := tx.QueryContext(ctx, `
		SELECT m.id, m.created_at, partner.user_id, v.name, v.birth_date, ph.storage_key,
		       ps.openness, ps.conscientiousness, ps.extraversion, ps.agreeableness, ps.neuroticism,
		       NOT EXISTS (SELECT 1 FROM message WHERE match_id = m.id)
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
		ORDER BY m.id DESC LIMIT $3`, userID, beforeID, limit+1)
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
