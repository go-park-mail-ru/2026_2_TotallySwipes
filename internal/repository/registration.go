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

	var profileID int64
	err = tx.QueryRowContext(ctx,
		`INSERT INTO profile (user_id) VALUES ($1) RETURNING id`, userID,
	).Scan(&profileID)
	if err != nil {
		return 0, fmt.Errorf("create user id=%d: insert profile: %w", userID, err)
	}

	_, err = tx.ExecContext(ctx,
		`INSERT INTO profile_version (
		    profile_id, revision, birth_date, sex, search_sex, search_age_from, search_age_to, dating_goal, about_me
		) VALUES ($1, 1, $2, $3, $4, $5, $6, $7, $8)`,
		profileID, version.BirthDate, version.Sex, version.SearchSex,
		version.SearchAgeFrom, version.SearchAgeTo, version.DatingGoal, nullIfEmpty(version.AboutMe),
	)
	if err != nil {
		return 0, fmt.Errorf("create user id=%d: insert profile version: %w", userID, err)
	}

	for _, name := range tags {
		var tagID int64
		err = tx.QueryRowContext(ctx,
			`INSERT INTO tag (name) VALUES ($1)
			 ON CONFLICT (name) DO UPDATE SET name = EXCLUDED.name
			 RETURNING id`, name,
		).Scan(&tagID)
		if err != nil {
			return 0, fmt.Errorf("create user id=%d: upsert tag %q: %w", userID, name, err)
		}

		if _, err = tx.ExecContext(ctx,
			`INSERT INTO profile_tag (profile_id, tag_id) VALUES ($1, $2)`, profileID, tagID,
		); err != nil {
			return 0, fmt.Errorf("create user id=%d: insert profile tag %q: %w", userID, name, err)
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
