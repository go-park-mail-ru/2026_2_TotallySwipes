package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5/pgconn"

	"dating-app/internal/model"
)

const pgUniqueViolation = "23505"

// CreateUserWithProfile в одной транзакции создаёт пользователя, его профиль,
// первую версию профиля и привязывает теги (несуществующие создаются).
// Если email занят - model.ErrEmailAlreadyExists
func (r *UserRepo) CreateUserWithProfile(ctx context.Context, user *model.UserInput,
	version *model.ProfileVersionInput, tags []string) (int64, error) {

	if user == nil || version == nil {
		return 0, fmt.Errorf("create user with profile: input is nil")
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("CreateUserWithProfile: begin transaction: %w", err)
	}
	defer tx.Rollback()

	var userID int64
	err = tx.QueryRowContext(ctx,
		`INSERT INTO "user" (name, email, password_hash, birth_date) VALUES ($1, $2, $3, $4) RETURNING id`,
		user.Name, user.Email, user.PasswordHash, user.BirthDate,
	).Scan(&userID)

	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == pgUniqueViolation {
		return 0, model.ErrEmailAlreadyExists
	}
	if err != nil {
		return 0, fmt.Errorf("create user: insert user: %w", err)
	}

	profileID, err := insertProfile(ctx, tx, userID)
	if err != nil {
		return 0, fmt.Errorf("create user id=%d: %w", userID, err)
	}

	if err := insertProfileVersion(ctx, tx, profileID, version); err != nil {
		return 0, fmt.Errorf("create user id=%d: %w", userID, err)
	}

	for _, name := range tags {
		tagID, err := upsertTag(ctx, tx, name)
		if err != nil {
			return 0, fmt.Errorf("create user id=%d: %w", userID, err)
		}
		if err := insertProfileTag(ctx, tx, profileID, tagID); err != nil {
			return 0, fmt.Errorf("create user id=%d: %w", userID, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("CreateUserWithProfile: commit transaction: %w", err)
	}
	return userID, nil
}

// IsProfileCompleted - профиль считается заполненным, когда пройден психотест
func (r *ProfileRepo) IsProfileCompleted(ctx context.Context, userID int64) (bool, error) {
	var completed bool
	err := r.db.QueryRowContext(ctx,
		`SELECT EXISTS (
		    SELECT 1 FROM profile p
		    JOIN profile_psycho ps ON ps.profile_id = p.id
		    WHERE p.user_id = $1
		)`, userID,
	).Scan(&completed)
	if err != nil {
		return false, fmt.Errorf("is profile completed user_id=%d: %w", userID, err)
	}
	return completed, nil
}

func nullIfEmpty(s string) sql.NullString {
	return sql.NullString{String: s, Valid: s != ""}
}
