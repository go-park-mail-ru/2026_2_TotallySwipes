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

// isUniqueViolation проверяет ошибку PostgreSQL на нарушение уникальности.
// Принимает: ошибку err, включая обёрнутую.
// Возвращает: true для кода 23505, иначе false; при вставке пользователя это трактуется как занятый email.
func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == pgUniqueViolation
}

// AuthUserRepo выполняет операции с пользователем для регистрации и входа.
type AuthUserRepo struct {
	db *sql.DB
}

// NewAuthUserRepository создаёт репозиторий регистрации и поиска пользователей.
// Принимает: подключение к PostgreSQL db.
// Возвращает: экземпляр AuthUserRepo.
func NewAuthUserRepository(db *sql.DB) *AuthUserRepo {
	return &AuthUserRepo{db: db}
}

// CreateUserWithProfile атомарно создаёт пользователя, профиль, первую версию, теги и фото.
// Принимает: контекст ctx, данные user и version, имена tags и метаданные photos.
// Возвращает: ID пользователя или ошибку; при занятом email — model.ErrEmailAlreadyExists.
func (r *AuthUserRepo) CreateUserWithProfile(ctx context.Context, user *model.UserInput,
	version *model.ProfileVersionInput, tags []string, photos []model.PhotoInput) (int64, error) {

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
		`INSERT INTO "user" (name, email, password_hash) VALUES ($1, $2, $3) RETURNING id`,
		user.Name, user.Email, user.PasswordHash,
	).Scan(&userID)

	if isUniqueViolation(err) {
		return 0, model.ErrEmailAlreadyExists
	}
	if err != nil {
		return 0, fmt.Errorf("create user: insert user: %w", err)
	}

	if err := createProfileTx(ctx, tx, userID, version, tags, photos); err != nil {
		return 0, fmt.Errorf("create user id=%d: create profile: %w", userID, err)
	}

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("CreateUserWithProfile: commit transaction: %w", err)
	}
	return userID, nil
}

// GetUserByEmail находит пользователя по email.
// Принимает: контекст ctx и адрес email.
// Возвращает: пользователя или ошибку; при отсутствии — model.ErrNotFound.
func (r *AuthUserRepo) GetUserByEmail(ctx context.Context, email string) (*model.User, error) {

	user := model.User{}

	err := r.db.QueryRowContext(ctx,
		`SELECT id, name, email, password_hash FROM "user" WHERE email = $1`,
		email).Scan(&user.ID, &user.Name, &user.Email, &user.PasswordHash)

	if errors.Is(err, sql.ErrNoRows) {
		return nil, model.ErrNotFound
	}

	if err != nil {
		return nil, fmt.Errorf("get user email=%s: %w", email, err)
	}

	return &user, nil
}
