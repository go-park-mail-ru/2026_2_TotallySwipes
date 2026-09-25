package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

type DatingGoal string

const (
	DatingGoalRelationship DatingGoal = "relationship"
	DatingGoalFriendship   DatingGoal = "friendship"
	DatingGoalCasual       DatingGoal = "casual"
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

type Tag struct {
	ID   int64
	Name string
}

type PhotoInput struct {
	StorageKey string
	Position   int
}

type Photo struct {
	ID         int64
	StorageKey string
	Position   int
}

type Profile struct {
	ID             int64
	UserID         int64
	CreatedAt      time.Time
	UpdatedAt      time.Time
	CurrentVersion ProfileVersion
	CurrentPsycho  *ProfilePsycho

	Tags   []Tag
	Photos []Photo
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
	DatingGoal    DatingGoal
	RecordedAt    time.Time
	Sex           Sex
	SearchSex     SearchSex
	SearchAgeFrom int
	SearchAgeTo   int
}

type ProfileVersionInput struct {
	ProfileID     int64
	BirthDate     time.Time
	DatingGoal    DatingGoal
	RecordedAt    time.Time
	Sex           Sex
	SearchSex     SearchSex
	SearchAgeFrom int
	SearchAgeTo   int
}

type ProfilePsycho struct {
	ID                int64
	ProfileID         int64
	TestID            int64
	RecordedAt        time.Time
	Openness          *float64
	Conscientiousness *float64
	Extraversion      *float64
	Agreeableness     *float64
	Neuroticism       *float64
}

type ProfilePsychoInput struct {
	ProfileID         int64
	TestID            int64
	RecordedAt        time.Time
	Openness          *float64
	Conscientiousness *float64
	Extraversion      *float64
	Agreeableness     *float64
	Neuroticism       *float64
}

type ProfileRepository interface {
	GetByIDCurrentProfile(ctx context.Context, id int64) (*Profile, error)

	CreateProfile(ctx context.Context, tx *sql.Tx, input *ProfileInput) (int64, error)

	AddProfileVersion(ctx context.Context, tx *sql.Tx, profileID int64, version *ProfileVersionInput) error
	AddProfilePsycho(ctx context.Context, tx *sql.Tx, profileID int64, psycho *ProfilePsychoInput) error

	SetProfileTags(ctx context.Context, tx *sql.Tx, profileID int64, tagIDs []int64) error
	AddProfilePhotos(ctx context.Context, tx *sql.Tx, profileID int64, photos []PhotoInput) error
}

type ProfileRepo struct {
	db *sql.DB
}

var _ ProfileRepository = (*ProfileRepo)(nil)

func NewProfileRepository(db *sql.DB) *ProfileRepo {
	return &ProfileRepo{db: db}
}

func (r *ProfileRepo) GetByIDCurrentProfile(ctx context.Context, id int64) (*Profile, error) {

	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})

	if err != nil {
		return nil, fmt.Errorf("get current profile id=%d: begin read: %w", id, err)
	}
	defer tx.Rollback()

	profile, err := getProfileVersionAndPsycho(ctx, tx, id)

	if err != nil {
		return nil, err
	}

	profile.Tags, err = getProfileTags(ctx, tx, id)

	if err != nil {
		return nil, fmt.Errorf("get current profile id=%d: tags: %w", id, err)
	}
	profile.Photos, err = getProfilePhotos(ctx, tx, id)

	if err != nil {
		return nil, fmt.Errorf("get current profile id=%d: photos: %w", id, err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("get current profile id=%d: commit read: %w", id, err)
	}
	return profile, nil
}

func (r *ProfileRepo) CreateProfile(ctx context.Context, tx *sql.Tx, input *ProfileInput) (int64, error) {

	if tx == nil {
		return 0, fmt.Errorf("create profile: transaction is nil")
	}

	if input == nil {
		return 0, fmt.Errorf("create profile: input is nil")
	}

	if input.CurrentPsycho != nil && input.CurrentPsycho.TestID <= 0 {
		return 0, fmt.Errorf("create profile: test_id must be positive")
	}

	var profileID int64

	err := tx.QueryRowContext(ctx,
		`INSERT INTO profile (user_id) VALUES ($1) RETURNING id`, input.UserID,
	).Scan(&profileID)

	if err != nil {
		return 0, fmt.Errorf("create profile user_id=%d: insert profile: %w", input.UserID, err)
	}

	version := input.CurrentVersion
	_, err = tx.ExecContext(ctx,
		`INSERT INTO profile_version (
		    profile_id, revision, birth_date, sex, search_sex, search_age_from, search_age_to, dating_goal
		) VALUES ($1, 1, $2, $3, $4, $5, $6, $7)`,
		profileID, version.BirthDate, version.Sex, version.SearchSex,
		version.SearchAgeFrom, version.SearchAgeTo, version.DatingGoal,
	)

	if err != nil {
		return 0, fmt.Errorf("create profile id=%d: insert version: %w", profileID, err)
	}

	if psycho := input.CurrentPsycho; psycho != nil {
		_, err = tx.ExecContext(ctx,
			`INSERT INTO profile_psycho (
			    profile_id, revision, test_id,
			    openness, conscientiousness, extraversion, agreeableness, neuroticism
			) VALUES ($1, 1, $2, $3, $4, $5, $6, $7)`,
			profileID, psycho.TestID,
			psycho.Openness, psycho.Conscientiousness, psycho.Extraversion, psycho.Agreeableness, psycho.Neuroticism,
		)

		if err != nil {
			return 0, fmt.Errorf("create profile id=%d: insert psycho test: %w", profileID, err)
		}
	}

	return profileID, nil
}

func (r *ProfileRepo) AddProfileVersion(ctx context.Context, tx *sql.Tx, profileID int64, input *ProfileVersionInput) error {

	if input == nil {
		return fmt.Errorf("add profile version: input is nil")
	}

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
		sex, search_sex, search_age_from, search_age_to, dating_goal)
		SELECT $1, COALESCE(MAX(revision), 0) + 1, $2, $3, $4, $5, $6, $7 FROM profile_version
		WHERE profile_id = $1;`,
		profileID,
		input.BirthDate,
		input.Sex,
		input.SearchSex,
		input.SearchAgeFrom,
		input.SearchAgeTo,
		input.DatingGoal)

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
		return fmt.Errorf("add psycho test: input is nil")
	}

	if input.Openness == nil || input.Conscientiousness == nil || input.Extraversion == nil || input.Agreeableness == nil || input.Neuroticism == nil {
		return fmt.Errorf("add psycho test profile_id=%d: all five Big Five scores are required", profileID)
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
		profile_id, revision, test_id, openness,
		conscientiousness, extraversion, agreeableness, neuroticism)
		SELECT $1, COALESCE(MAX(revision), 0) + 1, $2, $3, $4, $5, $6, $7 FROM profile_psycho
		WHERE profile_id = $1;`,
		profileID,
		input.TestID,
		input.Openness,
		input.Conscientiousness,
		input.Extraversion,
		input.Agreeableness,
		input.Neuroticism)

	if err != nil {
		return fmt.Errorf("add psycho test profile_id=%d: insert: %w", profileID, err)
	}

	_, err = tx.ExecContext(ctx, `UPDATE profile SET updated_at = CURRENT_TIMESTAMP WHERE id = $1;`, profileID)

	if err != nil {
		return fmt.Errorf("add psycho test profile_id=%d: update profile timestamp: %w", profileID, err)
	}

	return nil
}

func (r *ProfileRepo) SetProfileTags(ctx context.Context, tx *sql.Tx, profileID int64, tagIDs []int64) error {

	if tx == nil {
		return fmt.Errorf("set profile tags: transaction is nil")
	}

	seen := make(map[int64]bool, len(tagIDs))
	for _, id := range tagIDs {
		if id <= 0 || seen[id] {
			return fmt.Errorf("set profile tags: tag IDs must be positive and unique")
		}
		seen[id] = true
	}

	if err := lockProfile(ctx, tx, profileID); err != nil {
		return fmt.Errorf("set profile tags: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `DELETE FROM profile_tag WHERE profile_id = $1`, profileID); err != nil {
		return fmt.Errorf("set profile tags profile_id=%d: delete: %w", profileID, err)
	}

	for _, id := range tagIDs {
		if _, err := tx.ExecContext(ctx, `INSERT INTO profile_tag (profile_id, tag_id) VALUES ($1, $2)`, profileID, id); err != nil {
			return fmt.Errorf("set profile tags profile_id=%d: insert tag_id=%d: %w", profileID, id, err)
		}
	}

	return touchProfile(ctx, tx, profileID)
}

func (r *ProfileRepo) AddProfilePhotos(ctx context.Context, tx *sql.Tx, profileID int64, photos []PhotoInput) error {

	if len(photos) == 0 {
		return nil
	}

	if tx == nil {
		return fmt.Errorf("add profile photos: transaction is nil")
	}

	for _, photo := range photos {

		if photo.StorageKey == "" || photo.Position <= 0 || photo.Position > 10 {
			return fmt.Errorf("add profile photos: expected non empty storage key and position between 1 and 10")
		}
	}

	if err := lockProfile(ctx, tx, profileID); err != nil {
		return fmt.Errorf("add profile photos: %w", err)
	}

	for _, photo := range photos {
		if _, err := tx.ExecContext(ctx, `INSERT INTO photo (profile_id, storage_key, position) VALUES ($1, $2, $3)`, profileID, photo.StorageKey, photo.Position); err != nil {
			return fmt.Errorf("add profile photos profile_id=%d: insert: %w", profileID, err)
		}
	}

	return touchProfile(ctx, tx, profileID)
}

func lockProfile(ctx context.Context, tx *sql.Tx, profileID int64) error {

	var id int64
	err := tx.QueryRowContext(ctx, `SELECT id FROM profile WHERE id = $1 FOR UPDATE`, profileID).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}

	if err != nil {
		return fmt.Errorf("lock profile id=%d: %w", profileID, err)
	}
	return nil
}

func touchProfile(ctx context.Context, tx *sql.Tx, profileID int64) error {

	_, err := tx.ExecContext(ctx, `UPDATE profile SET updated_at = CURRENT_TIMESTAMP WHERE id = $1`, profileID)
	if err != nil {
		return fmt.Errorf("update profile id=%d timestamp: %w", profileID, err)
	}
	return nil
}

func getProfileVersionAndPsycho(ctx context.Context, tx *sql.Tx, profileID int64) (*Profile, error) {
	const query = `
		SELECT p.id, p.user_id, p.created_at, p.updated_at,
		       v.id, v.profile_id, v.birth_date, v.recorded_at,
		       v.sex, v.search_sex, v.search_age_from, v.search_age_to, v.dating_goal,
		       ps.id, ps.profile_id, ps.test_id, ps.recorded_at,
		       ps.openness, ps.conscientiousness, ps.extraversion, ps.agreeableness, ps.neuroticism
		FROM profile AS p
		JOIN LATERAL (
		    SELECT id, profile_id, birth_date, recorded_at,
		           sex, search_sex, search_age_from, search_age_to, dating_goal
		    FROM profile_version
		    WHERE profile_id = p.id
		    ORDER BY revision DESC
		    LIMIT 1
		) AS v ON v.profile_id = p.id
		LEFT JOIN LATERAL (
		    SELECT id, profile_id, test_id, recorded_at,
		           openness, conscientiousness, extraversion, agreeableness, neuroticism
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

	err := tx.QueryRowContext(ctx, query, profileID).Scan(
		&profile.ID, &profile.UserID, &profile.CreatedAt, &profile.UpdatedAt,
		&version.ID, &version.ProfileID, &version.BirthDate, &version.RecordedAt,
		&version.Sex, &version.SearchSex, &version.SearchAgeFrom, &version.SearchAgeTo, &version.DatingGoal,
		&psychoID, &psychoProfileID, &testID, &recordedAt,
		&psycho.Openness, &psycho.Conscientiousness, &psycho.Extraversion, &psycho.Agreeableness, &psycho.Neuroticism,
	)

	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}

	if err != nil {
		return nil, fmt.Errorf("get current profile id=%d: %w", profileID, err)
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

func getProfileTags(ctx context.Context, tx *sql.Tx, profileID int64) ([]Tag, error) {

	rows, err := tx.QueryContext(ctx,
		`SELECT t.id, t.name FROM tag AS t
		JOIN profile_tag AS pt ON pt.tag_id = t.id WHERE pt.profile_id = $1 ORDER BY t.id`, profileID)

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	tags := make([]Tag, 0)
	for rows.Next() {
		var tag Tag
		if err := rows.Scan(&tag.ID, &tag.Name); err != nil {
			return nil, err
		}
		tags = append(tags, tag)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}
	return tags, nil
}

func getProfilePhotos(ctx context.Context, tx *sql.Tx, profileID int64) ([]Photo, error) {

	rows, err := tx.QueryContext(ctx,
		`SELECT id, storage_key, position FROM photo WHERE profile_id = $1 ORDER BY position, id`, profileID)

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	photos := make([]Photo, 0)
	for rows.Next() {
		var photo Photo
		if err := rows.Scan(&photo.ID, &photo.StorageKey, &photo.Position); err != nil {
			return nil, err
		}
		photos = append(photos, photo)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return photos, nil
}
