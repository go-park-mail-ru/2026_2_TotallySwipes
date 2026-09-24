package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v4/pgxpool"
)



var (
	Male string
	Female string
	All string
)

type Profile struct {
	ID           int64
	UserID        int64
	CurrentVersion ProfileVersion
	CurrentPsycho ProfilePsycho
}

type ProfileVersion struct {
	ID int64
	BirthDate time.Time
	CreatedAt time.Time
	Sex Male
	SearchSex strings
	SearchAgeFrom int
	SearchAgeTo int

}

type ProfilePsycho struct {
	ID int64
	ProfileID int64
	TestID int64
	CreatedAt time.Time
	Weight1 int
	Weight2 int
	Weight3 int
	Weight4 int
	Weight5 int
}

type UpdateProfileInput struct {
	Name         string
	Email        string
	PasswordHash string
}

type ProfileRepository interface {
	GetByID(ctx context.Context, id int64) (User, error)
	DeleteById(ctx context.Context, id int64) error

	AddVersion(ctx context.Context, version ProfileVersion) error
}

type ProfileRepo struct {
	pool *pgxpool.Pool
}

func NewProfileRepository(pool *pgxpool.Pool) *ProfileRepo {
	return &ProfileRepo{pool: pool}
}

func (r *ProfileRepo) GetByID(ctx context.Context, id int64) (User, error) {

	temp := User{}

	err := r.pool.QueryRow(ctx,
		`SELECT id, name, email, password_hash, profile_id FROM Profile WHERE id = $1`,
		id).Scan(&temp.ID, &temp.Name, &temp.Email, &temp.PasswordHash, &temp.ProfileID)

	if err != nil {
		return User{}, fmt.Errorf("get by id=%d user: %W", id, err)
	}

	return temp, nil
}

func (r *ProfileRepo) Add(ctx context.Context, ProfileUser) error {

	_, err := r.pool.Exec(ctx,
		`INSERT INTO profiles (id, name, email, password_hash, profile_id) VALUES ($1, $2, $3, $4, $5)`,
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

func (r *ProfileRepo) Update(ctx context.Context, id int64, input UpdateUserInput) (User, error) {

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

func (r *ProfileRepo) RemoveById(ctx context.Context, id int64) error {

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
