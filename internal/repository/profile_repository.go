package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"slices"
	"time"

	"dating-app/internal/model"
)

type ProfileRepo struct {
	db *sql.DB
}

func NewProfileRepository(db *sql.DB) *ProfileRepo {
	return &ProfileRepo{db: db}
}

// profileSelect читает профиль с признаком фильтра, последней версией анкеты (её может не быть) и последним психопрофилем
const profileSelect = `
	SELECT p.id, p.user_id, p.created_at, p.updated_at,
	       EXISTS (SELECT 1 FROM search_filter WHERE user_id = p.user_id),
	       v.id, v.recorded_at, v.name, v.birth_date, v.sex, v.dating_goal, v.about_me,
	       v.education, v.work, v.smoking, v.alcohol, v.height,
	       ps.id, ps.recorded_at,
	       ps.openness, ps.conscientiousness, ps.extraversion, ps.agreeableness, ps.neuroticism
	FROM profile AS p
	LEFT JOIN LATERAL (
	    SELECT * FROM profile_version WHERE profile_id = p.id ORDER BY revision DESC LIMIT 1
	) AS v ON true
	LEFT JOIN LATERAL (
	    SELECT * FROM profile_psycho WHERE profile_id = p.id ORDER BY revision DESC LIMIT 1
	) AS ps ON true`

type rowScanner interface {
	Scan(dest ...any) error
}

// scanProfile разбирает строку profileSelect; теги и фото не загружает.
func scanProfile(row rowScanner) (*model.Profile, error) {
	var p model.Profile
	var versionID, psychoID *int64
	var versionRecordedAt, psychoRecordedAt *time.Time
	var psycho model.ProfilePsycho
	f := &p.CurrentVersion.ProfileFields

	err := row.Scan(&p.ID, &p.UserID, &p.CreatedAt, &p.UpdatedAt, &p.HasSearchFilter,
		&versionID, &versionRecordedAt, &f.Name, &f.BirthDate, &f.Sex, &f.DatingGoal, &f.AboutMe,
		&f.Education, &f.Work, &f.Smoking, &f.Alcohol, &f.Height,
		&psychoID, &psychoRecordedAt,
		&psycho.Openness, &psycho.Conscientiousness, &psycho.Extraversion, &psycho.Agreeableness, &psycho.Neuroticism,
	)
	if err != nil {
		return nil, err
	}

	if versionID != nil {
		p.CurrentVersion.ID = *versionID
		p.CurrentVersion.ProfileID = p.ID
		p.CurrentVersion.RecordedAt = *versionRecordedAt
	}
	if psychoID != nil {
		psycho.ID, psycho.ProfileID, psycho.RecordedAt = *psychoID, p.ID, *psychoRecordedAt
		p.CurrentPsycho = &psycho
	}
	p.Tags = make([]model.Tag, 0)
	p.Photos = make([]model.Photo, 0)
	return &p, nil
}

// GetByUserIDCurrentProfile загружает профиль с версией, психопрофилем, тегами и фото; нет профиля - model.ErrNotFound
func (r *ProfileRepo) GetByUserIDCurrentProfile(ctx context.Context, userID int64) (*model.Profile, error) {
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if err != nil {
		return nil, fmt.Errorf("get profile user_id=%d: begin read: %w", userID, err)
	}
	defer tx.Rollback()

	profile, err := loadProfileByUserID(ctx, tx, userID, false)
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("get profile user_id=%d: commit read: %w", userID, err)
	}
	return profile, nil
}

// loadProfileByUserID читает профиль в транзакции; lock - заблокировать строку профиля
func loadProfileByUserID(ctx context.Context, tx *sql.Tx, userID int64, lock bool) (*model.Profile, error) {
	if lock {
		var id int64
		err := tx.QueryRowContext(ctx, `SELECT id FROM profile WHERE user_id = $1 FOR UPDATE`, userID).Scan(&id)
		if errors.Is(err, sql.ErrNoRows) {
			return nil, model.ErrNotFound
		}
		if err != nil {
			return nil, fmt.Errorf("lock profile user_id=%d: %w", userID, err)
		}
	}

	profile, err := scanProfile(tx.QueryRowContext(ctx, profileSelect+` WHERE p.user_id = $1`, userID))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, model.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get profile user_id=%d: %w", userID, err)
	}

	if err := loadRelations(ctx, tx, profile); err != nil {
		return nil, fmt.Errorf("get profile user_id=%d: %w", userID, err)
	}
	return profile, nil
}

// PatchProfile сохраняет новую версию анкеты; теги наследуются, если патч их не заменяет
func (r *ProfileRepo) PatchProfile(ctx context.Context, userID int64, patch *model.ProfilePatch) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("patch profile: begin transaction: %w", err)
	}
	defer tx.Rollback()

	current, err := loadProfileByUserID(ctx, tx, userID, true)
	if err != nil {
		return err
	}

	version := model.ProfileVersionInput{ProfileFields: patch.Apply(current.CurrentVersion.ProfileFields), Tags: current.Tags}
	if patch.Tags != nil {
		if version.Tags, err = findTags(ctx, tx, *patch.Tags); err != nil {
			return err
		}
	}

	if _, err := insertProfileVersion(ctx, tx, current.ID, &version); err != nil {
		return err
	}
	if err := touchProfile(ctx, tx, current.ID); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("patch profile: commit transaction: %w", err)
	}
	return nil
}

// AddPhoto добавляет фото в конец списка; model.ErrPhotoLimit, если фото уже model.MaxPhotos
func (r *ProfileRepo) AddPhoto(ctx context.Context, userID int64, storageKey string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("add photo: begin transaction: %w", err)
	}
	defer tx.Rollback()

	profile, err := loadProfileByUserID(ctx, tx, userID, true)
	if err != nil {
		return err
	}
	if len(profile.Photos) >= model.MaxPhotos {
		return model.ErrPhotoLimit
	}

	_, err = tx.ExecContext(ctx,
		`INSERT INTO photo (profile_id, storage_key, position) VALUES ($1, $2, $3)`,
		profile.ID, storageKey, len(profile.Photos)+1)
	if err != nil {
		return fmt.Errorf("add photo profile_id=%d: %w", profile.ID, err)
	}

	if err := touchProfile(ctx, tx, profile.ID); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("add photo: commit transaction: %w", err)
	}
	return nil
}

// DeletePhoto удаляет фото, сдвигает следующие и возвращает ключ файла
func (r *ProfileRepo) DeletePhoto(ctx context.Context, userID, photoID int64) (string, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return "", fmt.Errorf("delete photo: begin transaction: %w", err)
	}
	defer tx.Rollback()

	profile, err := loadProfileByUserID(ctx, tx, userID, true)
	if err != nil {
		return "", err
	}

	idx := -1
	for i, photo := range profile.Photos {
		if photo.ID == photoID {
			idx = i
			break
		}
	}
	if idx < 0 {
		return "", model.ErrPhotoNotFound
	}
	if len(profile.Photos) == 1 && len(profile.Missing()) == 0 {
		return "", model.ErrLastPhoto
	}
	photo := profile.Photos[idx]

	if _, err := tx.ExecContext(ctx, `DELETE FROM photo WHERE id = $1`, photo.ID); err != nil {
		return "", fmt.Errorf("delete photo id=%d: %w", photo.ID, err)
	}
	_, err = tx.ExecContext(ctx,
		`UPDATE photo SET position = position - 1, updated_at = CURRENT_TIMESTAMP
		 WHERE profile_id = $1 AND position > $2`, profile.ID, photo.Position)
	if err != nil {
		return "", fmt.Errorf("shift photos profile_id=%d: %w", profile.ID, err)
	}

	if err := touchProfile(ctx, tx, profile.ID); err != nil {
		return "", err
	}
	if err := tx.Commit(); err != nil {
		return "", fmt.Errorf("delete photo: commit transaction: %w", err)
	}
	return photo.StorageKey, nil
}

// ReorderPhotos переставляет фото в порядке photoIDs; model.ErrPhotoOrderMismatch, если набор id не совпадает
func (r *ProfileRepo) ReorderPhotos(ctx context.Context, userID int64, photoIDs []int64) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("reorder photos: begin transaction: %w", err)
	}
	defer tx.Rollback()

	profile, err := loadProfileByUserID(ctx, tx, userID, true)
	if err != nil {
		return err
	}
	if len(photoIDs) != len(profile.Photos) {
		return model.ErrPhotoOrderMismatch
	}
	for _, photo := range profile.Photos {
		if !slices.Contains(photoIDs, photo.ID) {
			return model.ErrPhotoOrderMismatch
		}
	}

	_, err = tx.ExecContext(ctx,
		`UPDATE photo SET position = o.position, updated_at = CURRENT_TIMESTAMP
		 FROM unnest($2::bigint[]) WITH ORDINALITY AS o(id, position)
		 WHERE photo.id = o.id AND photo.profile_id = $1 AND photo.position <> o.position`,
		profile.ID, photoIDs)
	if err != nil {
		return fmt.Errorf("reorder photos profile_id=%d: %w", profile.ID, err)
	}

	if err := touchProfile(ctx, tx, profile.ID); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("reorder photos: commit transaction: %w", err)
	}
	return nil
}

// GetProfilesByCursorAndLimit - страница заполненных анкет под фильтр пользователя; условие заполненности должно совпадать с model.Profile.Missing
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

	rows, err := tx.QueryContext(ctx, profileSelect+`
		WHERE p.user_id > $1 AND p.user_id <> $2
		  AND v.name IS NOT NULL AND v.birth_date IS NOT NULL AND v.sex IS NOT NULL
		  AND v.dating_goal IS NOT NULL
		  AND EXISTS (SELECT 1 FROM photo WHERE profile_id = p.id)
		  AND EXISTS (SELECT 1 FROM search_filter WHERE user_id = p.user_id)
		  AND NOT EXISTS (
		      SELECT 1 FROM search_filter AS f WHERE f.user_id = $2
		        AND ((f.sex <> 'all' AND f.sex <> v.sex)
		          OR date_part('year', age($4::date, v.birth_date)) NOT BETWEEN f.age_from AND f.age_to))
		ORDER BY p.user_id ASC LIMIT $3`, afterID, userID, limit+1, time.Now().Format(time.DateOnly))
	if err != nil {
		return nil, nil, fmt.Errorf("feed: select profiles: %w", err)
	}
	defer rows.Close()

	profiles := make([]model.Profile, 0)
	for rows.Next() {
		p, err := scanProfile(rows)
		if err != nil {
			return nil, nil, fmt.Errorf("feed: scan profile: %w", err)
		}
		profiles = append(profiles, *p)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, fmt.Errorf("feed: read profiles: %w", err)
	}

	var nextCursor *int64
	if len(profiles) > limit {
		profiles = profiles[:limit]
		id := profiles[len(profiles)-1].UserID
		nextCursor = &id
	}

	for i := range profiles {
		if err := loadRelations(ctx, tx, &profiles[i]); err != nil {
			return nil, nil, fmt.Errorf("feed: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, nil, fmt.Errorf("feed: commit read: %w", err)
	}
	return profiles, nextCursor, nil
}

// loadRelations дополняет профиль тегами актуальной версии и фотографиями.
func loadRelations(ctx context.Context, tx *sql.Tx, profile *model.Profile) error {
	if profile.CurrentVersion.ID != 0 {
		tags, err := loadTags(ctx, tx, profile.CurrentVersion.ID)
		if err != nil {
			return fmt.Errorf("load profile id=%d tags: %w", profile.ID, err)
		}
		profile.Tags = tags
	}

	photos, err := loadPhotos(ctx, tx, profile.ID)
	if err != nil {
		return fmt.Errorf("load profile id=%d photos: %w", profile.ID, err)
	}
	profile.Photos = photos
	return nil
}

// insertProfileVersion создаёт следующую ревизию профиля и сохраняет её теги.
func insertProfileVersion(ctx context.Context, tx *sql.Tx, profileID int64, v *model.ProfileVersionInput) (int64, error) {
	const query = `INSERT INTO profile_version (
	    profile_id, revision, name, birth_date, sex, dating_goal, about_me,
	    education, work, smoking, alcohol, height)
	    SELECT $1, COALESCE(MAX(revision), 0) + 1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11
	    FROM profile_version WHERE profile_id = $1 RETURNING id`

	var versionID int64
	err := tx.QueryRowContext(ctx, query, profileID, v.Name, v.BirthDate, v.Sex, v.DatingGoal, v.AboutMe,
		v.Education, v.Work, v.Smoking, v.Alcohol, v.Height).Scan(&versionID)
	if err != nil {
		return 0, fmt.Errorf("insert profile version profile_id=%d: %w", profileID, err)
	}

	for _, tag := range v.Tags {
		_, err := tx.ExecContext(ctx,
			`INSERT INTO profile_tag (profile_version_id, tag_id) VALUES ($1, $2)`, versionID, tag.ID)
		if err != nil {
			return 0, fmt.Errorf("insert profile tag version_id=%d tag_id=%d: %w", versionID, tag.ID, err)
		}
	}
	return versionID, nil
}

func nullIfEmpty(s string) sql.NullString {
	return sql.NullString{String: s, Valid: s != ""}
}

// findTags возвращает теги в порядке names; model.ErrUnknownTag, если какого-то нет в справочнике
func findTags(ctx context.Context, tx *sql.Tx, names []string) ([]model.Tag, error) {
	tags := make([]model.Tag, 0, len(names))
	if len(names) == 0 {
		return tags, nil
	}

	rows, err := tx.QueryContext(ctx, `SELECT id, name FROM tag WHERE name = ANY($1)`, names)
	if err != nil {
		return nil, fmt.Errorf("find tags: %w", err)
	}
	defer rows.Close()

	ids := make(map[string]int64, len(names))
	for rows.Next() {
		var tag model.Tag
		if err := rows.Scan(&tag.ID, &tag.Name); err != nil {
			return nil, fmt.Errorf("find tags: %w", err)
		}
		ids[tag.Name] = tag.ID
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("find tags: %w", err)
	}

	for _, name := range names {
		id, ok := ids[name]
		if !ok {
			return nil, fmt.Errorf("find tag %q: %w", name, model.ErrUnknownTag)
		}
		tags = append(tags, model.Tag{ID: id, Name: name})
	}
	return tags, nil
}

func touchProfile(ctx context.Context, tx *sql.Tx, profileID int64) error {
	_, err := tx.ExecContext(ctx, `UPDATE profile SET updated_at = CURRENT_TIMESTAMP WHERE id = $1`, profileID)
	if err != nil {
		return fmt.Errorf("update profile id=%d timestamp: %w", profileID, err)
	}
	return nil
}

func loadTags(ctx context.Context, tx *sql.Tx, versionID int64) ([]model.Tag, error) {
	rows, err := tx.QueryContext(ctx,
		`SELECT t.id, t.name FROM tag AS t
		JOIN profile_tag AS pt ON pt.tag_id = t.id WHERE pt.profile_version_id = $1 ORDER BY t.id`, versionID)
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
	return tags, rows.Err()
}

func loadPhotos(ctx context.Context, tx *sql.Tx, profileID int64) ([]model.Photo, error) {
	rows, err := tx.QueryContext(ctx,
		`SELECT id, storage_key, position FROM photo WHERE profile_id = $1 ORDER BY position`, profileID)
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
	return photos, rows.Err()
}
