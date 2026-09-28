package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5/pgconn"

	"dating-app/internal/model"
)

var (
	ErrNotFound = model.ErrNotFound
)

const pgUniqueViolation = "23505"

// isUniqueViolation - в таблице "user" единственное UNIQUE-ограничение
// (кроме PK) - email, поэтому такую ошибку трактуем как занятый email
func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == pgUniqueViolation
}

type UserRepo struct {
	db *sql.DB
}

func NewUserRepository(db *sql.DB) *UserRepo {
	return &UserRepo{db: db}
}

func (r *UserRepo) GetUserByID(ctx context.Context, id int64) (*model.User, error) {

	user := model.User{}

	err := r.db.QueryRowContext(ctx,
		`SELECT id, name, email, password_hash, birth_date FROM "user" WHERE id = $1`,
		id).Scan(&user.ID, &user.Name, &user.Email, &user.PasswordHash, &user.BirthDate)

	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}

	if err != nil {
		return nil, fmt.Errorf("get user id=%d: %w", id, err)
	}

	return &user, nil
}

func (r *UserRepo) GetUserByEmail(ctx context.Context, email string) (*model.User, error) {

	user := model.User{}

	err := r.db.QueryRowContext(ctx,
		`SELECT id, name, email, password_hash, birth_date FROM "user" WHERE email = $1`,
		email).Scan(&user.ID, &user.Name, &user.Email, &user.PasswordHash, &user.BirthDate)

	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}

	if err != nil {
		return nil, fmt.Errorf("get user email=%s: %w", email, err)
	}

	return &user, nil
}

func (r *UserRepo) CreateUser(ctx context.Context, input *model.UserInput) error {

	if input == nil {
		return fmt.Errorf("create user: input is nil")
	}

	_, err := r.db.ExecContext(ctx,
		`INSERT INTO "user" (name, email, password_hash, birth_date) VALUES ($1, $2, $3, $4)`,
		input.Name,
		input.Email,
		input.PasswordHash,
		input.BirthDate,
	)

	if isUniqueViolation(err) {
		return model.ErrEmailAlreadyExists
	}
	if err != nil {
		return fmt.Errorf("create user: %w", err)
	}
	return nil
}

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

	if isUniqueViolation(err) {
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

// UpdateUser меняет только переданные (не nil) поля и возвращает пользователя
// после изменения. ErrNotFound - нет такого id, model.ErrEmailAlreadyExists -
// новый email занят
func (r *UserRepo) UpdateUser(ctx context.Context, id int64, input *model.UpdateUserInput) (*model.User, error) {

	if input == nil {
		return nil, fmt.Errorf("update user: input is nil")
	}

	var user model.User

	// nil превращается в NULL, и COALESCE оставляет текущее значение
	err := r.db.QueryRowContext(ctx,
		`UPDATE "user" SET
		    name = COALESCE($1, name),
		    email = COALESCE($2, email),
		    password_hash = COALESCE($3, password_hash),
		    birth_date = COALESCE($4, birth_date),
		    updated_at = CURRENT_TIMESTAMP
		WHERE id = $5
		RETURNING id, name, email, password_hash, birth_date`,
		input.Name,
		input.Email,
		input.PasswordHash,
		input.BirthDate,
		id,
	).Scan(&user.ID, &user.Name, &user.Email, &user.PasswordHash, &user.BirthDate)

	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if isUniqueViolation(err) {
		return nil, model.ErrEmailAlreadyExists
	}
	if err != nil {
		return nil, fmt.Errorf("update user id=%d: %w", id, err)
	}
	return &user, nil
}

func (r *UserRepo) DeleteUser(ctx context.Context, id int64) error {

	res, err := r.db.ExecContext(ctx, `DELETE FROM "user" WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete user id=%d: %w", id, err)
	}

	count, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("delete user id=%d: rows affected: %w", id, err)
	}
	if count == 0 {
		return ErrNotFound
	}
	return nil
}
