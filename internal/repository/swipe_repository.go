package repository

import (
	"context"
	"database/sql"
	"dating-app/internal/model"
	"errors"
	"fmt"
)

const selectSwipeTargetProfileQuery = `
	SELECT id FROM profile WHERE user_id = $1`

const upsertSwipeQuery = `
		INSERT INTO swipe (target_profile_id, author_id, action)
		VALUES ($1, $2, $3)
		ON CONFLICT (author_id, target_profile_id)
		DO UPDATE SET action = EXCLUDED.action, updated_at = CURRENT_TIMESTAMP`

const insertMutualMatchQuery = `
		INSERT INTO match (first_user_id, second_user_id, access_to_message)
		SELECT LEAST($1::bigint, $2::bigint),
			GREATEST($1::bigint, $2::bigint),
			TRUE
		WHERE $3::text IN ('like', 'super_like')
		AND EXISTS (
			SELECT 1
			FROM swipe s
			JOIN profile p ON p.id = s.target_profile_id
			WHERE s.author_id = $2
				AND p.user_id = $1
				AND s.action IN ('like', 'super_like')
		)
		ON CONFLICT (first_user_id, second_user_id) DO NOTHING`

const lockSwipeQuery = `SELECT id FROM "user"
						WHERE id IN ($1, $2)
						ORDER BY id
						FOR UPDATE`

type SwipeRepository struct {
	db *sql.DB
}

func NewSwipeRepository(db *sql.DB) *SwipeRepository {
	return &SwipeRepository{db: db}
}

func (r *SwipeRepository) SaveSwipeAndMatch(ctx context.Context, swipe *model.Swipe) error {

	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})

	if err != nil {
		return fmt.Errorf("save swipe and match: begin transaction: %w", err)
	}
	defer tx.Rollback()

	_, err = tx.ExecContext(ctx, lockSwipeQuery, swipe.AuthorID, swipe.TargetID)

	if err != nil {
		return fmt.Errorf("save swipe and match: lock users: %w", err)
	}

	var targetProfileID int64
	err = tx.QueryRowContext(ctx, selectSwipeTargetProfileQuery, swipe.TargetID).Scan(&targetProfileID)

	if errors.Is(err, sql.ErrNoRows) {
		return model.ErrNotFound
	}

	if err != nil {
		return fmt.Errorf("save swipe and match: get target profile: %w", err)
	}

	_, err = tx.ExecContext(ctx, upsertSwipeQuery, targetProfileID, swipe.AuthorID, swipe.Action)

	if err != nil {
		return fmt.Errorf("save swipe and match: insert swipe: %w", err)
	}

	_, err = tx.ExecContext(ctx, insertMutualMatchQuery,
		swipe.AuthorID, swipe.TargetID, swipe.Action)

	if err != nil {
		return fmt.Errorf("save swipe and match: insert match: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("save swipe and match: commit transaction: %w", err)
	}

	return nil
}
