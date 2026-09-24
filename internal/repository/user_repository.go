package repository

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v4/pgxpool"
)

type User struct {
	ID           int64
	Name         string
	Email        string
	PasswordHash string
	ProfileID    int64
}

type UpdateUserInput struct {
	Name         string
	Email        string
	PasswordHash string
}

type UserRepository interface {
	GetByID(ctx context.Context, id int64) (User, error)

	Add(ctx context.Context, user User) error
	Update(ctx context.Context, id int64, input UpdateUserInput) (User, error)
	DeleteById(ctx context.Context, id int64) error
}

type UserRepo struct {
	pool *pgxpool.Pool
}

func NewUserRepository(pool *pgxpool.Pool) *UserRepo {
	return &UserRepo{pool: pool}
}

func (r *UserRepo) GetByID(ctx context.Context, id int64) (User, error) {

	temp := User{}

	err := r.pool.QueryRow(ctx,
		`SELECT id, name, email, password_hash, profile_id FROM user WHERE id = $1`,
		id).Scan(&temp.ID, &temp.Name, &temp.Email, &temp.PasswordHash, &temp.ProfileID)

	if err != nil {
		return User{}, fmt.Errorf("get by id=%d user: %W", id, err)
	}

	return temp, nil
}

func (r *UserRepo) Add(ctx context.Context, user User) error {

	_, err := r.pool.Exec(ctx,
		`INSERT INTO "user" (id, name, email, password_hash, profile_id) VALUES ($1, $2, $3, $4, $5)`,
		user.ID,
		user.Name,
		user.Email,
		user.PasswordHash,
		user.ProfileID,
	)

	if err != nil {
		return fmt.Errorf("add user: %w", err)
	}

	return nil
}

func (r *UserRepo) Update(ctx context.Context, id int64, input UpdateUserInput) (User, error) {

	var temp User

	err := r.pool.QueryRow(ctx,
		`UPDATE "user" 
		SET name = $1, email = $2, password_hash = $3 WHERE id = $4 RETURNING id, name, email, password_hash, profile_id`,
		input.Name,
		input.Email,
		input.PasswordHash,
		id,
	).Scan(&temp.ID, &temp.Name, &temp.Email, &temp.PasswordHash, &temp.ProfileID)

	if err != nil {
		return User{}, fmt.Errorf("update user: %w", err)
	}

	return temp, nil
}

func (r *UserRepo) RemoveById(ctx context.Context, id int64) error {

	var temp int64
	if err := r.pool.QueryRow(ctx, `SELECT id FROM "user" WHERE id = $1`, id).Scan(&temp); err != nil {
		return fmt.Errorf("remove by id=%d  %w", id, err)
	}

	_, err := r.pool.Exec(ctx, `DELETE FROM "user" WHERE id = $1`, id)

	if err != nil {
		return fmt.Errorf("remove by id=%d user: %w", id, err)
	}

	return nil
}
