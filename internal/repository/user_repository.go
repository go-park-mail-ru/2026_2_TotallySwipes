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

// isUniqueViolation - ошибка PostgreSQL 23505 (нарушение уникальности)
func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == pgUniqueViolation
}

// UserRepo хранит учётные записи пользователей (таблица user).
type UserRepo struct {
	db *sql.DB
}

func NewUserRepository(db *sql.DB) *UserRepo {
	return &UserRepo{db: db}
}

// CreateUser атомарно создаёт пользователя и пустой профиль; занятый email - model.ErrEmailAlreadyExists
func (r *UserRepo) CreateUser(ctx context.Context, user *model.UserInput) (int64, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("create user: begin transaction: %w", err)
	}
	defer tx.Rollback()

	var userID int64
	err = tx.QueryRowContext(ctx,
		`INSERT INTO "user" (email, password_hash) VALUES ($1, $2) RETURNING id`,
		user.Email, user.PasswordHash,
	).Scan(&userID)
	if isUniqueViolation(err) {
		return 0, model.ErrEmailAlreadyExists
	}
	if err != nil {
		return 0, fmt.Errorf("create user: insert user: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `INSERT INTO profile (user_id) VALUES ($1)`, userID); err != nil {
		return 0, fmt.Errorf("create user id=%d: insert profile: %w", userID, err)
	}

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("create user: commit transaction: %w", err)
	}
	return userID, nil
}

// GetUserByEmail находит пользователя по email или возвращает model.ErrNotFound
func (r *UserRepo) GetUserByEmail(ctx context.Context, email string) (*model.User, error) {

	user := model.User{}

	err := r.db.QueryRowContext(ctx,
		`SELECT id, email, password_hash FROM "user" WHERE email = $1`,
		email).Scan(&user.ID, &user.Email, &user.PasswordHash)

	if errors.Is(err, sql.ErrNoRows) {
		return nil, model.ErrNotFound
	}

	if err != nil {
		return nil, fmt.Errorf("get user email=%s: %w", email, err)
	}

	return &user, nil
}
