package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"dating-app/internal/model"
)

// FilterRepo хранит фильтры ленты пользователей (таблица search_filter).
type FilterRepo struct {
	db *sql.DB
}

func NewFilterRepository(db *sql.DB) *FilterRepo {
	return &FilterRepo{db: db}
}

// Get возвращает фильтр пользователя или model.ErrNotFound, если он ещё не задан
func (r *FilterRepo) Get(ctx context.Context, userID int64) (*model.SearchFilter, error) {
	var f model.SearchFilter
	err := r.db.QueryRowContext(ctx,
		`SELECT sex, age_from, age_to FROM search_filter WHERE user_id = $1`, userID,
	).Scan(&f.Sex, &f.AgeFrom, &f.AgeTo)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, model.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get search filter user_id=%d: %w", userID, err)
	}
	return &f, nil
}

// Upsert создаёт или заменяет фильтр пользователя
func (r *FilterRepo) Upsert(ctx context.Context, userID int64, f *model.SearchFilter) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO search_filter (user_id, sex, age_from, age_to) VALUES ($1, $2, $3, $4)
		 ON CONFLICT (user_id) DO UPDATE
		 SET sex = EXCLUDED.sex, age_from = EXCLUDED.age_from, age_to = EXCLUDED.age_to,
		     updated_at = CURRENT_TIMESTAMP`,
		userID, f.Sex, f.AgeFrom, f.AgeTo)
	if err != nil {
		return fmt.Errorf("upsert search filter user_id=%d: %w", userID, err)
	}
	return nil
}
