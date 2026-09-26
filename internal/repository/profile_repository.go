package repository

import (
	"context"

	"database/sql"
	"dating-app/internal/model"
	"errors"
	"fmt"
	"time"
)

type ProfileRepository interface {
	GetProfilesByCursorAndLimit(ctx context.Context, userID int64, limit int, cursor *int64) ([]model.Profile, *int64, error)
	GetByIDCurrentProfile(ctx context.Context, id int64) (*model.Profile, error)

	CreateProfile(ctx context.Context, input *model.ProfileInput) (int64, error)

	AddProfileVersion(ctx context.Context, profileID int64, version *model.ProfileVersionInput) error
	AddProfilePsycho(ctx context.Context, profileID int64, psycho *model.ProfilePsychoInput) error

	SetProfileTags(ctx context.Context, profileID int64, tagIDs []int64) error
	AddProfilePhotos(ctx context.Context, profileID int64, photos []model.PhotoInput) error
}

type ProfileRepo struct {
	db *sql.DB
}

func NewProfileRepository(db *sql.DB) *ProfileRepo {
	return &ProfileRepo{db: db}
}

func (r *ProfileRepo) GetByIDCurrentProfile(ctx context.Context, id int64) (*model.Profile, error) {

	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})

	if err != nil {
		return nil, fmt.Errorf("get current profile id=%d: begin read: %w", id, err)
	}
	defer tx.Rollback()

	profile, err := getProfileVersionAndPsycho(ctx, tx, id)

	if err != nil {
		return nil, err
	}

	profile.Tags, err = loadTags(ctx, tx, id)

	if err != nil {
		return nil, fmt.Errorf("get current profile id=%d: tags: %w", id, err)
	}
	profile.Photos, err = loadPhoto(ctx, tx, id)

	if err != nil {
		return nil, fmt.Errorf("get current profile id=%d: photos: %w", id, err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("get current profile id=%d: commit read: %w", id, err)
	}

	return profile, nil
}

func (r *ProfileRepo) CreateProfile(ctx context.Context, input *model.ProfileInput) (int64, error) {

	if input == nil {
		return 0, fmt.Errorf("create profile: input is nil")
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("CreateProfile: begin transaction: %w", err)
	}
	defer tx.Rollback()

	var profileID int64

	err = tx.QueryRowContext(ctx,
		`INSERT INTO profile (user_id) VALUES ($1) RETURNING id`, input.UserID,
	).Scan(&profileID)

	if err != nil {
		return 0, fmt.Errorf("create profile user_id=%d: insert profile: %w", input.UserID, err)
	}

	version := input.CurrentVersion
	_, err = tx.ExecContext(ctx,
		`INSERT INTO profile_version (
		    profile_id, revision, birth_date, sex, search_sex, search_age_from, search_age_to, dating_goal, about_me
		) VALUES ($1, 1, $2, $3, $4, $5, $6, $7, $8)`,
		profileID, version.BirthDate, version.Sex, version.SearchSex,
		version.SearchAgeFrom, version.SearchAgeTo, version.DatingGoal, version.AboutMe,
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

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("CreateProfile: commit transaction: %w", err)
	}

	return profileID, nil
}

func (r *ProfileRepo) AddProfileVersion(ctx context.Context, profileID int64, input *model.ProfileVersionInput) error {

	if input == nil {
		return fmt.Errorf("add profile version: input is nil")
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("AddProfileVersion: begin transaction: %w", err)
	}
	defer tx.Rollback()

	var tempID int64
	err = tx.QueryRowContext(ctx, `SELECT id FROM profile WHERE id = $1 FOR UPDATE;`, profileID).Scan(&tempID)

	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}

	if err != nil {
		return fmt.Errorf("add profile version profile_id=%d: lock profile: %w", profileID, err)
	}

	_, err = tx.ExecContext(ctx,
		`INSERT INTO profile_version (
		profile_id, revision, birth_date,
		sex, search_sex, search_age_from, search_age_to, dating_goal, about_me)
		SELECT $1, COALESCE(MAX(revision), 0) + 1, $2, $3, $4, $5, $6, $7, $8 FROM profile_version
		WHERE profile_id = $1;`,
		profileID,
		input.BirthDate,
		input.Sex,
		input.SearchSex,
		input.SearchAgeFrom,
		input.SearchAgeTo,
		input.DatingGoal,
		input.AboutMe)

	if err != nil {
		return fmt.Errorf("add profile version profile_id=%d: insert: %w", profileID, err)
	}

	_, err = tx.ExecContext(ctx, `UPDATE profile SET updated_at = CURRENT_TIMESTAMP WHERE id = $1;`, profileID)

	if err != nil {
		return fmt.Errorf("add profile version profile_id=%d: update profile timestamp: %w", profileID, err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("AddProfileVersion: commit transaction: %w", err)
	}

	return nil
}

func (r *ProfileRepo) AddProfilePsycho(ctx context.Context, profileID int64, input *model.ProfilePsychoInput) error {

	if input == nil {
		return fmt.Errorf("add psycho test: input is nil")
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("AddProfilePsycho: begin transaction: %w", err)
	}
	defer tx.Rollback()

	var tempID int64
	err = tx.QueryRowContext(ctx, `SELECT id FROM profile WHERE id = $1 FOR UPDATE`, profileID).Scan(&tempID)

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

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("AddProfilePsycho: commit transaction: %w", err)
	}

	return nil
}

func (r *ProfileRepo) SetProfileTags(ctx context.Context, profileID int64, tagIDs []int64) error {

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("SetProfileTags: begin transaction: %w", err)
	}
	defer tx.Rollback()

	if err := lockProfile(ctx, tx, profileID); err != nil {
		return fmt.Errorf("set profile tags: %w", err)
	}

	if _, err = tx.ExecContext(ctx, `DELETE FROM profile_tag WHERE profile_id = $1`, profileID); err != nil {
		return fmt.Errorf("set profile tags profile_id=%d: delete: %w", profileID, err)
	}

	for _, id := range tagIDs {
		if _, err = tx.ExecContext(ctx, `INSERT INTO profile_tag (profile_id, tag_id) VALUES ($1, $2)`, profileID, id); err != nil {
			return fmt.Errorf("set profile tags profile_id=%d: insert tag_id=%d: %w", profileID, id, err)
		}
	}

	if err := touchProfile(ctx, tx, profileID); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("SetProfileTags: commit transaction: %w", err)
	}
	return nil
}

func (r *ProfileRepo) AddProfilePhotos(ctx context.Context, profileID int64, photos []model.PhotoInput) error {

	if len(photos) == 0 {
		return nil
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("AddProfilePhotos: begin transaction: %w", err)
	}
	defer tx.Rollback()

	if err := lockProfile(ctx, tx, profileID); err != nil {
		return fmt.Errorf("add profile photos: %w", err)
	}

	for _, photo := range photos {
		if _, err = tx.ExecContext(ctx, `INSERT INTO photo (profile_id, storage_key, position) VALUES ($1, $2, $3)`, profileID, photo.StorageKey, photo.Position); err != nil {
			return fmt.Errorf("add profile photos profile_id=%d: insert: %w", profileID, err)
		}
	}

	if err := touchProfile(ctx, tx, profileID); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("AddProfilePhotos: commit transaction: %w", err)
	}
	return nil
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

func getProfileVersionAndPsycho(ctx context.Context, tx *sql.Tx, profileID int64) (*model.Profile, error) {
	const query = `
		SELECT p.id, p.user_id, p.created_at, p.updated_at, u.name,
		       v.id, v.profile_id, v.birth_date, v.recorded_at,
		       v.sex, v.search_sex, v.search_age_from, v.search_age_to, v.dating_goal, v.about_me,
		       ps.id, ps.profile_id, ps.test_id, ps.recorded_at,
		       ps.openness, ps.conscientiousness, ps.extraversion, ps.agreeableness, ps.neuroticism
		FROM profile AS p
 		JOIN "user" AS u ON u.id = p.user_id
		JOIN LATERAL (
		    SELECT id, profile_id, birth_date, recorded_at,
		           sex, search_sex, search_age_from, search_age_to, dating_goal, about_me
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

	var profile model.Profile
	var psychoID, psychoProfileID, testID *int64
	var recordedAt *time.Time
	var psycho model.ProfilePsycho

	version := &profile.CurrentVersion

	err := tx.QueryRowContext(ctx, query, profileID).Scan(
		&profile.ID, &profile.UserID, &profile.CreatedAt, &profile.UpdatedAt, &profile.Name,
		&version.ID, &version.ProfileID, &version.BirthDate, &version.RecordedAt,
		&version.Sex, &version.SearchSex, &version.SearchAgeFrom, &version.SearchAgeTo, &version.DatingGoal, &version.AboutMe,
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

	err = tx.QueryRowContext(ctx, `SELECT name FROM "user" WHERE id = $1`, profile.UserID).Scan(&profile.Name)

	if err != nil {
		return nil, fmt.Errorf("get current profile id=%d: %w", profileID, err)
	}

	return &profile, nil
}

func (r *ProfileRepo) GetProfilesByCursorAndLimit(ctx context.Context, userID int64, limit int, cursor *int64) ([]model.Profile, *int64, error) {

	afterID := int64(0)
	if cursor != nil {
		afterID = *cursor
	}

	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if err != nil {
		return nil, nil, fmt.Errorf("feed: begin read: %w", err)
	}
	defer tx.Rollback()

	rows, err := tx.QueryContext(ctx,
		`
		SELECT p.id, p.user_id, p.created_at, p.updated_at, u.name,
		v.id, v.profile_id, v.birth_date, v.recorded_at, v.sex, v.search_sex,
		v.search_age_from, v.search_age_to, v.dating_goal, v.about_me
		FROM profile AS p
		JOIN "user" AS u ON u.id = p.user_id
		JOIN LATERAL (
		SELECT * FROM profile_version WHERE profile_id = p.id ORDER BY revision DESC LIMIT 1
		) AS v ON true
		WHERE u.id > $1 AND u.id <> $2
		ORDER BY u.id ASC LIMIT $3`, afterID, userID, limit+1)

	if err != nil {
		return nil, nil, fmt.Errorf("feed: select profiles: %w", err)
	}
	defer rows.Close()

	profiles := make([]model.Profile, 0)
	for rows.Next() {
		var p model.Profile
		v := &p.CurrentVersion
		if err := rows.Scan(&p.ID, &p.UserID, &p.CreatedAt, &p.UpdatedAt, &p.Name,
			&v.ID, &v.ProfileID, &v.BirthDate, &v.RecordedAt, &v.Sex, &v.SearchSex,
			&v.SearchAgeFrom, &v.SearchAgeTo, &v.DatingGoal, &v.AboutMe); err != nil {
			return nil, nil, fmt.Errorf("feed: scan profile: %w", err)
		}

		p.Tags = make([]model.Tag, 0)
		p.Photos = make([]model.Photo, 0)
		profiles = append(profiles, p)
	}

	if err := rows.Err(); err != nil {
		return nil, nil, fmt.Errorf("feed: read profiles: %w", err)
	}

	var nextCursor *int64 = nil
	if len(profiles) > limit {
		profiles = profiles[:limit]
		id := profiles[len(profiles)-1].UserID
		nextCursor = &id
	}

	profiles, err = loadFeedRelations(ctx, tx, profiles)
	if err != nil {
		return nil, nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, nil, fmt.Errorf("feed: commit read: %w", err)
	}

	return profiles, nextCursor, nil
}

func loadFeedRelations(ctx context.Context, tx *sql.Tx, profiles []model.Profile) ([]model.Profile, error) {

	for i := range profiles {

		profile := &profiles[i]

		tags, err := loadTags(ctx, tx, profile.ID)
		if err != nil {
			return nil, fmt.Errorf("load profile id=%d tags: %w", profile.ID, err)
		}

		profile.Tags = tags

		photos, err := loadPhoto(ctx, tx, profile.ID)
		if err != nil {
			return nil, fmt.Errorf("load profile id=%d photos: %w", profile.ID, err)
		}

		profile.Photos = photos
	}

	return profiles, nil
}

func loadTags(ctx context.Context, tx *sql.Tx, profileID int64) ([]model.Tag, error) {

	rows, err := tx.QueryContext(ctx,
		`SELECT t.id, t.name FROM tag AS t
		JOIN profile_tag AS pt ON pt.tag_id = t.id WHERE pt.profile_id = $1 ORDER BY t.id`, profileID)

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	tags := make([]model.Tag, 0)
	for rows.Next() {
		var tag model.Tag
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

func loadPhoto(ctx context.Context, tx *sql.Tx, profileID int64) ([]model.Photo, error) {

	rows, err := tx.QueryContext(ctx,
		`SELECT id, storage_key, position FROM photo WHERE profile_id = $1 ORDER BY position, id`, profileID)

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	photos := make([]model.Photo, 0)
	for rows.Next() {
		var photo model.Photo
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
