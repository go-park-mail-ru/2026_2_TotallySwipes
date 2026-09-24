package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

var (
	ErrNotFound = fmt.Errorf("not found")
)

type User struct {
	ID           int64
	Name         string
	Email        string
	PasswordHash string
	BirthDate    time.Time
}

type UserInput struct {
	Name         string
	Email        string
	PasswordHash string
	BirthDate    time.Time
}

type UpdateUserInput struct {
	Name         string
	Email        string
	PasswordHash string
	BirthDate    time.Time
}

type UserRepository interface {
	CreateUser(ctx context.Context, tx *sql.Tx, user *User) error
	GetUserByEmail(ctx context.Context, email string) (*User, error)
	GetUserByID(ctx context.Context, id int64) (*User, error)

	UpdateUser(ctx context.Context, tx *sql.Tx, id int64, input *UpdateUserInput) (*User, error)
	DeleteUserByID(ctx context.Context, tx *sql.Tx, id int64) error
}

type UserRepo struct {
	db *sql.DB
}

func NewUserRepository(db *sql.DB) *UserRepo {
	return &UserRepo{db: db}
}

func (r *UserRepo) GetUserByID(ctx context.Context, id int64) (*User, error) {

	user := User{}

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

func (r *UserRepo) GetUserByEmail(ctx context.Context, email string) (*User, error) {

	user := User{}

	err := r.db.QueryRowContext(ctx,
		`SELECT id, name, email, password_hash, birth_date FROM "user" WHERE email = $1`,
		email).Scan(&user.ID, &user.Name, &user.Email, &user.PasswordHash, &user.BirthDate)

	if err != nil {
		return nil, fmt.Errorf("get user email=%s: %w", email, err)
	}

	return &user, nil
}

func (r *UserRepo) CreateUser(ctx context.Context, tx *sql.Tx, input *UserInput) error {

	if tx == nil {
		return fmt.Errorf("create user: transaction is nil")
	}

	if input == nil {
		return fmt.Errorf("create user: input is nil")
	}

	_, err := tx.ExecContext(ctx,
		`INSERT INTO "user" (id, name, email, password_hash, birth_date) VALUES ($1, $2, $3, $4, $5)`,
		input.Name,
		input.Email,
		input.PasswordHash,
		input.BirthDate,
	)

	if err != nil {
		return fmt.Errorf("add user: %w", err)
	}

	return nil
}

func (r *UserRepo) UpdateUser(ctx context.Context, tx *sql.Tx, id int64, input *UpdateUserInput) (*User, error) {

	if tx == nil {
		return nil, fmt.Errorf("update user: transaction is nil")
	}

	if input == nil {
		return nil, fmt.Errorf("update user: input is nil")
	}

	var user User

	err := tx.QueryRowContext(ctx,
		`UPDATE "user"
		SET name = $1, email = $2, password_hash = $3, birth_date = $4, updated_at = CURRENT_TIMESTAMP WHERE id = $5 RETURNING id, name, email, password_hash, birth_date`,
		input.Name,
		input.Email,
		input.PasswordHash,
		input.BirthDate,
		id,
	).Scan(&user.ID, &user.Name, &user.Email, &user.PasswordHash, &user.BirthDate)

	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}

	if err != nil {
		return nil, fmt.Errorf("update user: %w", err)
	}

	return &user, nil
}

func (r *UserRepo) DeleteUserByID(ctx context.Context, tx *sql.Tx, id int64) error {

	if tx == nil {
		return fmt.Errorf("delete user id=%d: transaction is nil", id)
	}

	res, err := tx.ExecContext(ctx, `DELETE FROM "user" WHERE id = $1`, id)

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
