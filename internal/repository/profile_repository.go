package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"dating-app/internal/model"
)

type ProfileRepo struct {
	db *sql.DB
}

// NewProfileRepository создаёт репозиторий профилей.
// Принимает: подключение к PostgreSQL db.
// Возвращает: экземпляр ProfileRepo.
func NewProfileRepository(db *sql.DB) *ProfileRepo {
	return &ProfileRepo{db: db}
}

// GetByUserIDCurrentProfile загружает профиль с актуальной версией, психопрофилем, тегами и фото.
// Принимает: контекст ctx и ID пользователя.
// Возвращает: профиль или ошибку; при отсутствии — model.ErrNotFound.
func (r *ProfileRepo) GetByUserIDCurrentProfile(ctx context.Context, userID int64) (*model.Profile, error) {

	var profileID int64
	err := r.db.QueryRowContext(ctx, "SELECT id FROM profile WHERE user_id = $1", userID).Scan(&profileID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, model.ErrNotFound
	}

	if err != nil {
		return nil, fmt.Errorf("get current profile by user id=%d: %w", userID, err)
	}

	return r.GetByIDCurrentProfile(ctx, profileID)
}

// GetByIDCurrentProfile загружает актуальное состояние профиля в согласованном снимке БД.
// Принимает: контекст ctx и ID профиля id.
// Возвращает: профиль с версией, психопрофилем, тегами и фото либо ошибку, включая model.ErrNotFound.
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

	profile.Tags, err = loadTags(ctx, tx, profile.CurrentVersion.ID)

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

// createProfileTx создаёт профиль, первую версию с тегами и фотографии в переданной транзакции.
// Принимает: контекст ctx, транзакцию tx, userID, данные version, имена tags и метаданные photos.
// Возвращает: nil при успехе или ошибку; не изменяет version и не завершает транзакцию.
func createProfileTx(ctx context.Context, tx *sql.Tx, userID int64,
	version *model.ProfileVersionInput, tags []string, photos []model.PhotoInput) error {
	if version == nil {
		return fmt.Errorf("create profile: version is nil")
	}

	profileID, err := insertProfile(ctx, tx, userID)
	if err != nil {
		return err
	}

	initialVersion := *version
	initialVersion.Tags = make([]model.Tag, 0, len(tags))
	for _, name := range tags {
		tagID, err := upsertTag(ctx, tx, name)
		if err != nil {
			return err
		}
		initialVersion.Tags = append(initialVersion.Tags, model.Tag{ID: tagID, Name: name})
	}

	if _, err := insertProfileVersion(ctx, tx, profileID, &initialVersion); err != nil {
		return err
	}

	for _, photo := range photos {
		if err := insertPhoto(ctx, tx, profileID, photo); err != nil {
			return err
		}
	}

	return nil
}

// AddProfileVersion добавляет новую версию профиля вместе с полным набором её тегов.
// Принимает: контекст ctx, ID профиля profileID и данные input; пустой список тегов означает версию без тегов.
// Возвращает: nil при успехе или ошибку; версия и теги сохраняются в одной транзакции.
func (r *ProfileRepo) AddProfileVersion(ctx context.Context, profileID int64, input *model.ProfileVersionInput) error {

	if input == nil {
		return fmt.Errorf("add profile version: input is nil")
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("AddProfileVersion: begin transaction: %w", err)
	}
	defer tx.Rollback()

	if err := lockProfile(ctx, tx, profileID); err != nil {
		return fmt.Errorf("add profile version: %w", err)
	}

	if _, err := insertProfileVersion(ctx, tx, profileID, input); err != nil {
		return err
	}

	if err := touchProfile(ctx, tx, profileID); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("AddProfileVersion: commit transaction: %w", err)
	}

	return nil
}

// insertProfileVersionTags привязывает набор тегов к версии профиля в переданной транзакции.
// Принимает: контекст ctx, транзакцию tx, ID версии versionID и tags, из которых используются ID.
// Возвращает: nil при успехе или ошибку вставки; транзакцию не завершает.
func insertProfileVersionTags(ctx context.Context, tx *sql.Tx, versionID int64, tags []model.Tag) error {
	for _, tag := range tags {
		if err := insertProfileVersionTag(ctx, tx, versionID, tag.ID); err != nil {
			return err
		}
	}
	return nil
}

// AddProfilePsycho добавляет новую ревизию психопрофиля в отдельной транзакции.
// Принимает: контекст ctx, ID профиля profileID и данные результата input.
// Возвращает: nil при успехе или ошибку; ответы на вопросы этот метод не сохраняет.
func (r *ProfileRepo) AddProfilePsycho(ctx context.Context, profileID int64, input *model.ProfilePsychoInput) error {

	if input == nil {
		return fmt.Errorf("add psycho test: input is nil")
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("AddProfilePsycho: begin transaction: %w", err)
	}
	defer tx.Rollback()

	if err := lockProfile(ctx, tx, profileID); err != nil {
		return fmt.Errorf("add psycho test: %w", err)
	}

	if err := insertProfilePsycho(ctx, tx, profileID, input); err != nil {
		return err
	}

	if err := touchProfile(ctx, tx, profileID); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("AddProfilePsycho: commit transaction: %w", err)
	}

	return nil
}

// AddProfilePhotos добавляет фотографии профиля в одной транзакции.
// Принимает: контекст ctx, ID профиля profileID и метаданные photos.
// Возвращает: nil при успехе или ошибку записи.
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
		if err := insertPhoto(ctx, tx, profileID, photo); err != nil {
			return err
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

// IsProfileCompleted проверяет наличие хотя бы одной записи profile_psycho у пользователя.
// Принимает: контекст ctx и ID пользователя userID.
// Возвращает: признак наличия записи и ошибку запроса; заполненность пяти координат не проверяет.
func (r *ProfileRepo) IsProfileCompleted(ctx context.Context, userID int64) (bool, error) {
	var completed bool
	err := r.db.QueryRowContext(ctx,
		`SELECT EXISTS (
		    SELECT 1 FROM profile p
		    JOIN profile_psycho ps ON ps.profile_id = p.id
		    WHERE p.user_id = $1
		)`, userID,
	).Scan(&completed)
	if err != nil {
		return false, fmt.Errorf("is profile completed user_id=%d: %w", userID, err)
	}
	return completed, nil
}

// insertProfile создаёт запись профиля в переданной транзакции.
// Принимает: контекст ctx, транзакцию tx и ID пользователя userID.
// Возвращает: ID созданного профиля или ошибку; транзакцию не завершает.
func insertProfile(ctx context.Context, tx *sql.Tx, userID int64) (int64, error) {
	var profileID int64
	err := tx.QueryRowContext(ctx,
		`INSERT INTO profile (user_id) VALUES ($1) RETURNING id`, userID,
	).Scan(&profileID)
	if err != nil {
		return 0, fmt.Errorf("insert profile user_id=%d: %w", userID, err)
	}
	return profileID, nil
}

// insertProfileVersion создаёт следующую ревизию профиля и сохраняет её теги.
// Принимает: контекст ctx, транзакцию tx, ID профиля profileID и данные версии v.
// Возвращает: ID версии или ошибку; транзакцией и блокировкой профиля управляет вызывающий код.
func insertProfileVersion(ctx context.Context, tx *sql.Tx, profileID int64, v *model.ProfileVersionInput) (int64, error) {
	const query = `INSERT INTO profile_version (
     profile_id, revision, birth_date, sex, search_sex,
     search_age_from, search_age_to, dating_goal, about_me)
     SELECT $1, COALESCE(MAX(revision), 0) + 1, $2, $3, $4, $5, $6, $7, $8
     FROM profile_version WHERE profile_id = $1 RETURNING id`

	var versionID int64
	err := tx.QueryRowContext(ctx, query, profileID, v.BirthDate, v.Sex, v.SearchSex,
		v.SearchAgeFrom, v.SearchAgeTo, v.DatingGoal, v.AboutMe).Scan(&versionID)

	if err != nil {
		return 0, fmt.Errorf("insert profile version profile_id=%d: %w", profileID, err)
	}

	if err := insertProfileVersionTags(ctx, tx, versionID, v.Tags); err != nil {
		return 0, fmt.Errorf("insert tags for profile version %d: %w", versionID, err)
	}

	return versionID, nil
}

// insertProfilePsycho создаёт следующую ревизию психопрофиля в переданной транзакции.
// Принимает: контекст ctx, транзакцию tx, ID профиля profileID и данные p.
// Возвращает: nil при успехе или ошибку; транзакцией и блокировкой профиля управляет вызывающий код.
func insertProfilePsycho(ctx context.Context, tx *sql.Tx, profileID int64, p *model.ProfilePsychoInput) error {
	_, err := tx.ExecContext(ctx,
		`INSERT INTO profile_psycho (
		    profile_id, revision, test_id,
		    openness, conscientiousness, extraversion, agreeableness, neuroticism, personality_type)
		SELECT $1, COALESCE(MAX(revision), 0) + 1, $2, $3, $4, $5, $6, $7, $8 FROM profile_psycho
		WHERE profile_id = $1`,
		profileID, p.TestID,
		p.Openness, p.Conscientiousness, p.Extraversion, p.Agreeableness, p.Neuroticism,
		nullIfEmpty(string(p.PersonalityType)),
	)
	if err != nil {
		return fmt.Errorf("insert profile psycho profile_id=%d: %w", profileID, err)
	}
	return nil
}

// nullIfEmpty преобразует строку в значение для nullable-столбца SQL.
// Принимает: строку s.
// Возвращает: sql.NullString с Valid=false для пустой строки, иначе с исходным значением.
func nullIfEmpty(s string) sql.NullString {
	return sql.NullString{String: s, Valid: s != ""}
}

// insertPhoto сохраняет метаданные фотографии профиля в переданной транзакции.
// Принимает: контекст ctx, транзакцию tx, ID профиля profileID и метаданные photo.
// Возвращает: nil при успехе или ошибку; файл не загружает и транзакцию не завершает.
func insertPhoto(ctx context.Context, tx *sql.Tx, profileID int64, photo model.PhotoInput) error {
	_, err := tx.ExecContext(ctx,
		`INSERT INTO photo (profile_id, storage_key, position) VALUES ($1, $2, $3)`,
		profileID, photo.StorageKey, photo.Position)
	if err != nil {
		return fmt.Errorf("insert photo profile_id=%d position=%d: %w", profileID, photo.Position, err)
	}
	return nil
}

// insertProfileVersionTag создаёт связь версии профиля с тегом в переданной транзакции.
// Принимает: контекст ctx, транзакцию tx, ID версии versionID и ID тега tagID.
// Возвращает: nil при успехе или ошибку; транзакцию не завершает.
func insertProfileVersionTag(ctx context.Context, tx *sql.Tx, versionID, tagID int64) error {
	const query = `INSERT INTO profile_tag (profile_version_id, tag_id) VALUES ($1, $2)`
	if _, err := tx.ExecContext(ctx, query, versionID, tagID); err != nil {
		return fmt.Errorf("insert profile tag version_id=%d tag_id=%d: %w", versionID, tagID, err)

	}
	return nil
}

// upsertTag находит тег по имени или создаёт его в переданной транзакции.
// Принимает: контекст ctx, транзакцию tx и имя name.
// Возвращает: ID существующего или нового тега либо ошибку; транзакцию не завершает.
func upsertTag(ctx context.Context, tx *sql.Tx, name string) (int64, error) {
	var tagID int64
	err := tx.QueryRowContext(ctx,
		`INSERT INTO tag (name) VALUES ($1)
		 ON CONFLICT (name) DO UPDATE SET name = EXCLUDED.name
		 RETURNING id`, name,
	).Scan(&tagID)
	if err != nil {
		return 0, fmt.Errorf("upsert tag %q: %w", name, err)
	}
	return tagID, nil
}

// lockProfile блокирует строку профиля через SELECT FOR UPDATE до завершения транзакции.
// Принимает: контекст ctx, транзакцию tx и ID профиля profileID.
// Возвращает: nil при успехе или ошибку; при отсутствии профиля — model.ErrNotFound.
func lockProfile(ctx context.Context, tx *sql.Tx, profileID int64) error {

	var id int64
	err := tx.QueryRowContext(ctx, `SELECT id FROM profile WHERE id = $1 FOR UPDATE`, profileID).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return model.ErrNotFound
	}

	if err != nil {
		return fmt.Errorf("lock profile id=%d: %w", profileID, err)
	}
	return nil
}

// touchProfile обновляет время изменения профиля в переданной транзакции.
// Принимает: контекст ctx, транзакцию tx и ID профиля profileID.
// Возвращает: nil при успехе или ошибку запроса; транзакцию не завершает.
func touchProfile(ctx context.Context, tx *sql.Tx, profileID int64) error {

	_, err := tx.ExecContext(ctx, `UPDATE profile SET updated_at = CURRENT_TIMESTAMP WHERE id = $1`, profileID)
	if err != nil {
		return fmt.Errorf("update profile id=%d timestamp: %w", profileID, err)
	}
	return nil
}

// getProfileVersionAndPsycho читает профиль с последними ревизиями анкеты и психопрофиля.
// Принимает: контекст ctx, транзакцию tx и ID профиля profileID.
// Возвращает: профиль без отдельно загружаемых тегов и фото либо ошибку, включая model.ErrNotFound.
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
	var aboutMe sql.NullString

	version := &profile.CurrentVersion

	err := tx.QueryRowContext(ctx, query, profileID).Scan(
		&profile.ID, &profile.UserID, &profile.CreatedAt, &profile.UpdatedAt, &profile.Name,
		&version.ID, &version.ProfileID, &version.BirthDate, &version.RecordedAt,
		&version.Sex, &version.SearchSex, &version.SearchAgeFrom, &version.SearchAgeTo, &version.DatingGoal, &aboutMe,
		&psychoID, &psychoProfileID, &testID, &recordedAt,
		&psycho.Openness, &psycho.Conscientiousness, &psycho.Extraversion, &psycho.Agreeableness, &psycho.Neuroticism,
	)

	if errors.Is(err, sql.ErrNoRows) {
		return nil, model.ErrNotFound
	}

	if err != nil {
		return nil, fmt.Errorf("get current profile id=%d: %w", profileID, err)
	}
	if aboutMe.Valid {
		version.AboutMe = &aboutMe.String
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

// GetProfilesByCursorAndLimit читает страницу профилей по возрастанию ID пользователей, исключая самого пользователя.
// Принимает: контекст ctx, ID пользователя userID, положительный limit и cursor — последний ID пользователя или nil.
// Возвращает: профили с тегами и фото, курсор следующей страницы (nil в конце) и ошибку.
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
		v.search_age_from, v.search_age_to, v.dating_goal, v.about_me,
		ps.id, ps.profile_id, ps.test_id, ps.recorded_at,
		ps.openness, ps.conscientiousness, ps.extraversion, ps.agreeableness, ps.neuroticism
		FROM profile AS p
		JOIN "user" AS u ON u.id = p.user_id
		JOIN LATERAL (
		SELECT * FROM profile_version WHERE profile_id = p.id ORDER BY revision DESC LIMIT 1
		) AS v ON true
		LEFT JOIN LATERAL (
		SELECT * FROM profile_psycho WHERE profile_id = p.id ORDER BY revision DESC LIMIT 1
		) AS ps ON true
		WHERE u.id > $1 AND u.id <> $2
		ORDER BY u.id ASC LIMIT $3`, afterID, userID, limit+1)

	if err != nil {
		return nil, nil, fmt.Errorf("feed: select profiles: %w", err)
	}
	defer rows.Close()

	profiles := make([]model.Profile, 0)
	for rows.Next() {
		var p model.Profile
		var psycho model.ProfilePsycho
		var psychoID, psychoProfileID, testID *int64
		var recordedAt *time.Time
		v := &p.CurrentVersion
		if err := rows.Scan(&p.ID, &p.UserID, &p.CreatedAt, &p.UpdatedAt, &p.Name,
			&v.ID, &v.ProfileID, &v.BirthDate, &v.RecordedAt, &v.Sex, &v.SearchSex,
			&v.SearchAgeFrom, &v.SearchAgeTo, &v.DatingGoal, &v.AboutMe,
			&psychoID, &psychoProfileID, &testID, &recordedAt,
			&psycho.Openness, &psycho.Conscientiousness, &psycho.Extraversion,
			&psycho.Agreeableness, &psycho.Neuroticism); err != nil {
			return nil, nil, fmt.Errorf("feed: scan profile: %w", err)
		}

		if psychoID != nil {
			psycho.ID, psycho.ProfileID, psycho.TestID = *psychoID, *psychoProfileID, *testID
			psycho.RecordedAt = *recordedAt
			p.CurrentPsycho = &psycho
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

// loadFeedRelations дополняет профили ленты тегами актуальных версий и фотографиями.
// Принимает: контекст ctx, транзакцию tx и срез profiles.
// Возвращает: обновлённый срез или ошибку; изменяет элементы переданного среза.
func loadFeedRelations(ctx context.Context, tx *sql.Tx, profiles []model.Profile) ([]model.Profile, error) {

	for i := range profiles {

		profile := &profiles[i]

		tags, err := loadTags(ctx, tx, profile.CurrentVersion.ID)
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

// loadTags загружает теги конкретной версии профиля.
// Принимает: контекст ctx, транзакцию tx и ID версии versionID.
// Возвращает: срез тегов или ошибку чтения.
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

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return tags, nil
}

// loadPhoto загружает фотографии профиля в порядке отображения.
// Принимает: контекст ctx, транзакцию tx и ID профиля profileID.
// Возвращает: срез фотографий или ошибку чтения.
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

// GetShortByUserID читает краткие данные профиля и ключ его главной фотографии.
// Принимает: контекст ctx и ID пользователя userID.
// Возвращает: краткий профиль без вычисления возраста и URL либо ошибку, включая model.ErrNotFound.
func (r *ProfileRepo) GetShortByUserID(ctx context.Context, userID int64) (*model.ProfileShort, error) {
	var short model.ProfileShort
	var photoKey sql.NullString

	err := r.db.QueryRowContext(ctx,
		`SELECT u.id, u.name, v.birth_date, ph.storage_key
		FROM profile AS p
		JOIN "user" AS u ON u.id = p.user_id
		JOIN LATERAL (
		    SELECT birth_date FROM profile_version WHERE profile_id = p.id ORDER BY revision DESC LIMIT 1
		) AS v ON true
		LEFT JOIN LATERAL (
		    SELECT storage_key FROM photo WHERE profile_id = p.id ORDER BY position, id LIMIT 1
		) AS ph ON true
		WHERE p.user_id = $1`, userID,
	).Scan(&short.UserID, &short.Name, &short.BirthDate, &photoKey)

	if errors.Is(err, sql.ErrNoRows) {
		return nil, model.ErrNotFound
	}

	if err != nil {
		return nil, fmt.Errorf("get short profile user_id=%d: %w", userID, err)
	}

	if photoKey.Valid {
		short.MainPhotoKey = &photoKey.String
	}

	return &short, nil
}
