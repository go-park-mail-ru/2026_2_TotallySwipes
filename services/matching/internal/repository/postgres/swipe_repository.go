package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"dating-app/services/matching/internal/model"
)

type SwipeRepository struct{ db *sql.DB }

func NewSwipeRepository(db *sql.DB) *SwipeRepository {

	return &SwipeRepository{db: db}
}

// Блокируем пару в Matching, не читая таблицу аккаунтов Auth.
const lockSwipeQuery = `
    SELECT pg_advisory_xact_lock(hashtextextended(
        'matching-pair:' || LEAST($1::bigint, $2::bigint)::text || ':' || GREATEST($1::bigint, $2::bigint)::text, 0
    ))`

const upsertSwipeQuery = `
    INSERT INTO swipe (target_profile_id, author_id, action)
    VALUES ($1, $2, $3)
    ON CONFLICT (author_id, target_profile_id)
    DO UPDATE SET action = EXCLUDED.action, updated_at = CURRENT_TIMESTAMP`

// Сохраняем существующее правило: нужна встречная положительная реакция.
const insertMutualMatchQuery = `
    INSERT INTO match (first_user_id, second_user_id, access_to_message)
    SELECT LEAST($1::bigint, $2::bigint), GREATEST($1::bigint, $2::bigint), TRUE
    WHERE $3::text IN ('like', 'super_like')
      AND EXISTS (
          SELECT 1 FROM swipe
          WHERE author_id = $2 AND target_profile_id = $4 AND action IN ('like', 'super_like')
      )
    ON CONFLICT (first_user_id, second_user_id) DO NOTHING`

const selectMatchIDQuery = `
    SELECT id
    FROM match
    WHERE first_user_id = LEAST($1::bigint, $2::bigint)
      AND second_user_id = GREATEST($1::bigint, $2::bigint)
      AND $3::text IN ('like', 'super_like')`

func (r *SwipeRepository) SaveSwipeAndMatch(ctx context.Context, swipe *model.Swipe, authorProfileID, targetProfileID int64) (int64, error) {

	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})

	if err != nil {
		return 0, fmt.Errorf("save swipe and match: begin transaction: %w", err)
	}

	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, lockSwipeQuery, swipe.AuthorID, swipe.TargetID); err != nil {
		return 0, fmt.Errorf("save swipe and match: lock pair: %w", err)
	}

	if _, err := tx.ExecContext(ctx, upsertSwipeQuery, targetProfileID, swipe.AuthorID, string(swipe.Action)); err != nil {
		return 0, fmt.Errorf("save swipe and match: insert swipe: %w", err)
	}

	if _, err := tx.ExecContext(ctx, insertMutualMatchQuery, swipe.AuthorID, swipe.TargetID, string(swipe.Action), authorProfileID); err != nil {
		return 0, fmt.Errorf("save swipe and match: insert match: %w", err)
	}

	var matchID int64
	err = tx.QueryRowContext(ctx, selectMatchIDQuery, swipe.AuthorID, swipe.TargetID, string(swipe.Action)).Scan(&matchID)

	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return 0, fmt.Errorf("save swipe and match: select match: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("save swipe and match: commit: %w", err)
	}

	return matchID, nil
}
