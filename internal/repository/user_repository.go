package repository

import (
	"context"

	"database/sql"
	"dating-app/internal/model"
	"errors"
	"fmt"
)

var (
	ErrNotFound = fmt.Errorf("not found")
)

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

	if err != nil {
		return nil, fmt.Errorf("get user email=%s: %w", email, err)
	}

	return &user, nil
}

func (r *UserRepo) CreateUser(ctx context.Context, input *model.UserInput) error {

	if input == nil {
		return fmt.Errorf("create user: input is nil")
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("CreateUser: begin transaction: %w", err)
	}
	defer tx.Rollback()

	_, err = tx.ExecContext(ctx,
		`INSERT INTO "user" (name, email, password_hash, birth_date) VALUES ($1, $2, $3, $4)`,
		input.Name,
		input.Email,
		input.PasswordHash,
		input.BirthDate,
	)

	if err != nil {
		return fmt.Errorf("add user: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("CreateUser: commit transaction: %w", err)
	}
	return nil
}

func (r *UserRepo) UpdateUser(ctx context.Context, id int64, input *model.UpdateUserInput) (*model.User, error) {

	if input == nil {
		return nil, fmt.Errorf("update user: input is nil")
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("UpdateUser: begin transaction: %w", err)
	}
	defer tx.Rollback()

	var user model.User

	err = tx.QueryRowContext(ctx,
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

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("UpdateUser: commit transaction: %w", err)
	}
	return &user, nil
}

func (r *UserRepo) DeleteUserByID(ctx context.Context, id int64) error {

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("DeleteUserByID: begin transaction: %w", err)
	}
	defer tx.Rollback()

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

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("DeleteUserByID: commit transaction: %w", err)
	}
	return nil
}
