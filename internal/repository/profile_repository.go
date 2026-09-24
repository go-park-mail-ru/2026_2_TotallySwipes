package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

type Sex string

const (
	SexMale   Sex = "male"
	SexFemale Sex = "female"
)

type SearchSex string

const (
	SearchSexMale   SearchSex = "male"
	SearchSexFemale SearchSex = "female"
	SearchSexAll    SearchSex = "all"
)

type Profile struct {
	ID             int64
	UserID         int64
	CreatedAt      time.Time
	UpdatedAt      time.Time
	CurrentVersion ProfileVersion
	CurrentPsycho  *ProfilePsycho
}

type ProfileInput struct {
	UserID         int64
	CurrentVersion ProfileVersionInput
	CurrentPsycho  *ProfilePsychoInput
}

type ProfileVersion struct {
	ID            int64
	ProfileID     int64
	BirthDate     time.Time
	RecordedAt    time.Time
	Sex           Sex
	SearchSex     SearchSex
	SearchAgeFrom int
	SearchAgeTo   int
}

type ProfileVersionInput struct {
	ProfileID     int64
	BirthDate     time.Time
	RecordedAt    time.Time
	Sex           Sex
	SearchSex     SearchSex
	SearchAgeFrom int
	SearchAgeTo   int
}

type ProfilePsycho struct {
	ID         int64
	ProfileID  int64
	TestID     int64
	RecordedAt time.Time
	Weight1    *float64
	Weight2    *float64
	Weight3    *float64
	Weight4    *float64
	Weight5    *float64
}

type ProfilePsychoInput struct {
	ProfileID  int64
	TestID     int64
	RecordedAt time.Time
	Weight1    *float64
	Weight2    *float64
	Weight3    *float64
	Weight4    *float64
	Weight5    *float64
}

type ProfileRepository interface {
	GetByIDCurrentProfile(ctx context.Context, id int64) (*Profile, error)

	CreateProfile(ctx context.Context, tx *sql.Tx, input *ProfileInput) error

	AddProfileVersion(ctx context.Context, tx *sql.Tx, profileID int64, version *ProfileVersionInput) error
	AddProfilePsycho(ctx context.Context, tx *sql.Tx, profileID int64, psycho *ProfilePsychoInput) error
}

type ProfileRepo struct {
	db *sql.DB
}

func NewProfileRepository(db *sql.DB) *ProfileRepo {
	return &ProfileRepo{db: db}
}

func (r *ProfileRepo) GetByIDCurrentProfile(ctx context.Context, id int64) (*Profile, error) {
	const query = `
		SELECT p.id, p.user_id, p.created_at, p.updated_at,
		       v.id, v.profile_id, v.birth_date, v.recorded_at,
		       v.sex, v.search_sex, v.search_age_from, v.search_age_to,
		       ps.id, ps.profile_id, ps.test_id, ps.recorded_at,
		       ps.weight_1, ps.weight_2, ps.weight_3, ps.weight_4, ps.weight_5
		FROM profile AS p
		JOIN LATERAL (
		    SELECT id, profile_id, birth_date, recorded_at,
		           sex, search_sex, search_age_from, search_age_to
		    FROM profile_version
		    WHERE profile_id = p.id
		    ORDER BY revision DESC
		    LIMIT 1
		) AS v ON v.profile_id = p.id
		LEFT JOIN LATERAL (
		    SELECT id, profile_id, test_id, recorded_at,
		           weight_1, weight_2, weight_3, weight_4, weight_5
		    FROM profile_psycho
		    WHERE profile_id = p.id
		    ORDER BY revision DESC
		    LIMIT 1
		) AS ps ON true
		WHERE p.id = $1`

	var profile Profile
	var psychoID, psychoProfileID, testID *int64
	var recordedAt *time.Time
	var psycho ProfilePsycho

	version := &profile.CurrentVersion

	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&profile.ID, &profile.UserID, &profile.CreatedAt, &profile.UpdatedAt,
		&version.ID, &version.ProfileID, &version.BirthDate, &version.RecordedAt,
		&version.Sex, &version.SearchSex, &version.SearchAgeFrom, &version.SearchAgeTo,
		&psychoID, &psychoProfileID, &testID, &recordedAt,
		&psycho.Weight1, &psycho.Weight2, &psycho.Weight3, &psycho.Weight4, &psycho.Weight5,
	)

	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}

	if err != nil {
		return nil, fmt.Errorf("get current profile id=%d: %w", id, err)
	}

	if psychoID != nil {
		psycho.ID = *psychoID
		psycho.ProfileID = *psychoProfileID
		psycho.TestID = *testID
		psycho.RecordedAt = *recordedAt
		profile.CurrentPsycho = &psycho
	}

	return &profile, nil
}

func (r *ProfileRepo) CreateProfile(ctx context.Context, tx *sql.Tx, input *ProfileInput) error {

	if tx == nil {
		return fmt.Errorf("create profile: transaction is nil")
	}

	if input == nil {
		return fmt.Errorf("create profile: input is nil")
	}

	if input.CurrentPsycho != nil && input.CurrentPsycho.TestID <= 0 {
		return fmt.Errorf("create profile: test_id must be positive")
	}

	var profileID int64

	err := tx.QueryRowContext(ctx,
		`INSERT INTO profile (user_id) VALUES ($1) RETURNING id`, input.UserID,
	).Scan(&profileID)

	if err != nil {
		return fmt.Errorf("create profile user_id=%d: insert profile: %w", input.UserID, err)
	}

	version := input.CurrentVersion
	_, err = tx.ExecContext(ctx,
		`INSERT INTO profile_version (
		    profile_id, revision, birth_date, sex, search_sex, search_age_from, search_age_to
		) VALUES ($1, 1, $2, $3, $4, $5, $6)`,
		profileID, version.BirthDate, version.Sex, version.SearchSex,
		version.SearchAgeFrom, version.SearchAgeTo,
	)

	if err != nil {
		return fmt.Errorf("create profile id=%d: insert version: %w", profileID, err)
	}

	if psycho := input.CurrentPsycho; psycho != nil {
		_, err = tx.ExecContext(ctx,
			`INSERT INTO profile_psycho (
			    profile_id, revision, test_id,
			    weight_1, weight_2, weight_3, weight_4, weight_5
			) VALUES ($1, 1, $2, $3, $4, $5, $6, $7)`,
			profileID, psycho.TestID,
			psycho.Weight1, psycho.Weight2, psycho.Weight3, psycho.Weight4, psycho.Weight5,
		)

		if err != nil {
			return fmt.Errorf("create profile id=%d: insert psycho test: %w", profileID, err)
		}
	}

	return nil
}

func (r *ProfileRepo) AddProfileVersion(ctx context.Context, tx *sql.Tx, profileID int64, input ProfileVersionInput) error {

	if tx == nil {
		return fmt.Errorf("write profile: transaction is nil")
	}

	var tempID int64
	err := tx.QueryRowContext(ctx, `SELECT id FROM profile WHERE id = $1 FOR UPDATE;`, profileID).Scan(&tempID)

	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}

	if err != nil {
		return fmt.Errorf("add profile version profile_id=%d: lock profile: %w", profileID, err)
	}

	_, err = tx.ExecContext(ctx,
		`INSERT INTO profile_version (
		profile_id, revision, birth_date,
		sex, search_sex, search_age_from, search_age_to)
		SELECT $1, COALESCE(MAX(revision), 0) + 1, $2, $3, $4, $5, $6 FROM profile_version
		WHERE profile_id = $1;`,
		profileID,
		input.BirthDate,
		input.Sex,
		input.SearchSex,
		input.SearchAgeFrom,
		input.SearchAgeTo)

	if err != nil {
		return fmt.Errorf("add profile version profile_id=%d: insert: %w", profileID, err)
	}

	_, err = tx.ExecContext(ctx, `UPDATE profile SET updated_at = CURRENT_TIMESTAMP WHERE id = $1;`, profileID)

	if err != nil {
		return fmt.Errorf("add profile version profile_id=%d: update profile timestamp: %w", profileID, err)
	}

	return nil
}

func (r *ProfileRepo) AddProfilePsycho(ctx context.Context, tx *sql.Tx, profileID int64, input *ProfilePsychoInput) error {

	if input == nil {
		return fmt.Errorf("add pscho test: input is nil")
	}

	if input.Weight1 == nil || input.Weight2 == nil || input.Weight3 == nil || input.Weight4 == nil || input.Weight5 == nil {
		return fmt.Errorf("add psycho test profile_id=%d: all five weights are required", profileID)
	}

	if tx == nil {
		return fmt.Errorf("write profile: transaction is nil")
	}

	var tempID int64
	err := tx.QueryRowContext(ctx, `SELECT id FROM profile WHERE id = $1 FOR UPDATE`, profileID).Scan(&tempID)

	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}

	if err != nil {
		return fmt.Errorf("add psycho test profile_id=%d: lock profile: %w", profileID, err)
	}

	_, err = tx.ExecContext(ctx,
		`INSERT INTO profile_psycho (
		profile_id, revision, test_id, weight_1,
		weight_2, weight_3, weight_4, weight_5)
		SELECT $1, COALESCE(MAX(revision), 0) + 1, $2, $3, $4, $5, $6, $7 FROM profile_psycho
		WHERE profile_id = $1;`,
		profileID,
		input.TestID,
		input.Weight1,
		input.Weight2,
		input.Weight3,
		input.Weight4,
		input.Weight5)

	if err != nil {
		return fmt.Errorf("add psycho test profile_id=%d: insert: %w", profileID, err)
	}

	_, err = tx.ExecContext(ctx, `UPDATE profile SET updated_at = CURRENT_TIMESTAMP WHERE id = $1;`, profileID)

	if err != nil {
		return fmt.Errorf("add psycho test profile_id=%d: update profile timestamp: %w", profileID, err)
	}

	return nil
}
