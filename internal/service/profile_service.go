package service

import (
	"context"
	"dating-app/internal/model"
	"errors"
	"fmt"
	"time"
)

type ProfileService interface {
	// GetNextFeed формирует страницу анкет с возрастом, URL фото и совместимостью при наличии обоих векторов.
	// Принимает: контекст ctx, ID пользователя userID, limit от 1 до 10 и необязательный cursor.
	// Возвращает: страницу ленты или ошибку, включая model.ErrInvalidFeedRequest и model.ErrProfileRequired.
	GetNextFeed(ctx context.Context, userID int64, limit int, cursor *int64) (*model.FeedPage, error)

	// AddProfilePsycho добавляет новую ревизию психопрофиля в отдельной транзакции.
	// Принимает: контекст ctx, ID профиля profileID и данные результата input.
	// Возвращает: nil при успехе или ошибку; ответы на вопросы этот метод не сохраняет.
	AddProfilePsycho(ctx context.Context, profileID int64, psycho *model.ProfilePsychoInput) error
	// GetByUserIDCurrentProfile загружает профиль с актуальной версией, психопрофилем, тегами и фото.
	// Принимает: контекст ctx и ID пользователя.
	// Возвращает: профиль или ошибку; при отсутствии — model.ErrNotFound.
	GetByUserIDCurrentProfile(ctx context.Context, id int64) (*model.Profile, error)
	// GetShortProfile получает краткий профиль, вычисляет возраст и URL главного фото.
	// Принимает: контекст ctx и ID пользователя userID.
	// Возвращает: краткий профиль или ошибку; при отсутствии профиля — model.ErrProfileRequired.
	GetShortProfile(ctx context.Context, userID int64) (*model.ProfileShort, error)
}

type ProfileRepository interface {
	// GetByIDCurrentProfile загружает актуальное состояние профиля в согласованном снимке БД.
	// Принимает: контекст ctx и ID профиля id.
	// Возвращает: профиль с версией, психопрофилем, тегами и фото либо ошибку, включая model.ErrNotFound.
	GetByIDCurrentProfile(ctx context.Context, id int64) (*model.Profile, error)
	// GetByUserIDCurrentProfile загружает профиль с актуальной версией, психопрофилем, тегами и фото.
	// Принимает: контекст ctx и ID пользователя.
	// Возвращает: профиль или ошибку; при отсутствии — model.ErrNotFound.
	GetByUserIDCurrentProfile(ctx context.Context, id int64) (*model.Profile, error)
	// GetShortByUserID читает краткие данные профиля и ключ его главной фотографии.
	// Принимает: контекст ctx и ID пользователя userID.
	// Возвращает: краткий профиль без вычисления возраста и URL либо ошибку, включая model.ErrNotFound.
	GetShortByUserID(ctx context.Context, userID int64) (*model.ProfileShort, error)

	// AddProfileVersion добавляет новую версию профиля вместе с полным набором её тегов.
	// Принимает: контекст ctx, ID профиля profileID и данные input; пустой список тегов означает версию без тегов.
	// Возвращает: nil при успехе или ошибку; версия и теги сохраняются в одной транзакции.
	AddProfileVersion(ctx context.Context, profileID int64, version *model.ProfileVersionInput) error
	// AddProfilePsycho добавляет новую ревизию психопрофиля в отдельной транзакции.
	// Принимает: контекст ctx, ID профиля profileID и данные результата input.
	// Возвращает: nil при успехе или ошибку; ответы на вопросы этот метод не сохраняет.
	AddProfilePsycho(ctx context.Context, profileID int64, psycho *model.ProfilePsychoInput) error

	// AddProfilePhotos добавляет фотографии профиля в одной транзакции.
	// Принимает: контекст ctx, ID профиля profileID и метаданные photos.
	// Возвращает: nil при успехе или ошибку записи.
	AddProfilePhotos(ctx context.Context, profileID int64, photos []model.PhotoInput) error

	// GetProfilesByCursorAndLimit читает страницу профилей по возрастанию ID пользователей, исключая самого пользователя.
	// Принимает: контекст ctx, ID пользователя userID, положительный limit и cursor — последний ID пользователя или nil.
	// Возвращает: профили с тегами и фото, курсор следующей страницы (nil в конце) и ошибку.
	GetProfilesByCursorAndLimit(ctx context.Context, userID int64, limit int, cursor *int64) ([]model.Profile, *int64, error)
}

type ProfileServiceImpl struct {
	profileRepo      ProfileRepository
	media            URLProvider
	compatibilitySvc CompatibilityService
}

// NewProfileService создаёт сервис профилей и ленты.
// Принимает: репозиторий repo, сервис совместимости svc и поставщик URL media.
// Возвращает: экземпляр ProfileServiceImpl.
func NewProfileService(repo ProfileRepository, svc CompatibilityService, media URLProvider) *ProfileServiceImpl {
	return &ProfileServiceImpl{profileRepo: repo, compatibilitySvc: svc, media: media}
}

// GetByUserIDCurrentProfile загружает профиль с актуальной версией, психопрофилем, тегами и фото.
// Принимает: контекст ctx и ID пользователя.
// Возвращает: профиль или ошибку; при отсутствии — model.ErrNotFound.
func (s *ProfileServiceImpl) GetByUserIDCurrentProfile(ctx context.Context, id int64) (*model.Profile, error) {
	profile, err := s.profileRepo.GetByUserIDCurrentProfile(ctx, id)

	if err != nil {
		return nil, fmt.Errorf("get user profile id=%d: %w", id, err)
	}

	return profile, nil
}

// GetShortProfile получает краткий профиль, вычисляет возраст и URL главного фото.
// Принимает: контекст ctx и ID пользователя userID.
// Возвращает: краткий профиль или ошибку; при отсутствии профиля — model.ErrProfileRequired.
func (s *ProfileServiceImpl) GetShortProfile(ctx context.Context, userID int64) (*model.ProfileShort, error) {

	short, err := s.profileRepo.GetShortByUserID(ctx, userID)
	if errors.Is(err, model.ErrNotFound) {
		return nil, fmt.Errorf("get short profile user id=%d: %w", userID, model.ErrProfileRequired)
	}

	if err != nil {
		return nil, fmt.Errorf("get short profile user id=%d: %w", userID, err)
	}

	short.Age = CalculateAge(short.BirthDate, time.Now())
	if short.MainPhotoKey != nil {
		url, err := s.media.GetURL(ctx, *short.MainPhotoKey)

		if err != nil {
			return nil, fmt.Errorf("get short profile photo user id=%d: %w", userID, err)
		}
		short.MainPhotoURL = &url
	}

	return short, nil
}

// GetNextFeed формирует страницу анкет с возрастом, URL фото и совместимостью при наличии обоих векторов.
// Принимает: контекст ctx, ID пользователя userID, limit от 1 до 10 и необязательный cursor.
// Возвращает: страницу ленты или ошибку, включая model.ErrInvalidFeedRequest и model.ErrProfileRequired.
func (s *ProfileServiceImpl) GetNextFeed(ctx context.Context, userID int64, limit int, cursor *int64) (*model.FeedPage, error) {

	if userID <= 0 || limit < 1 || limit > 10 || (cursor != nil && *cursor <= 0) {
		return nil, fmt.Errorf("%w: invalid user ID, limit or cursor", model.ErrInvalidFeedRequest)
	}

	userProfile, err := s.profileRepo.GetByUserIDCurrentProfile(ctx, userID)
	if errors.Is(err, model.ErrNotFound) {
		return nil, fmt.Errorf("get feed user id=%d: %w", userID, model.ErrProfileRequired)
	}

	if err != nil {
		return nil, fmt.Errorf("get feed viewer profile: %w", err)
	}

	if userProfile == nil {
		return nil, fmt.Errorf("get feed: viewer profile is nil")
	}
	userVector, userHasTest := feedBigFive(userProfile.CurrentPsycho)
	profiles, nextCursor, err := s.profileRepo.GetProfilesByCursorAndLimit(ctx, userID, limit, cursor)

	if err != nil {
		return nil, fmt.Errorf("get feed profiles: %w", err)
	}

	page := &model.FeedPage{Items: make([]model.FeedItem, 0, len(profiles)), NextCursor: nextCursor}
	for _, profile := range profiles {

		profileVector, candidateHasTest := feedBigFive(profile.CurrentPsycho)
		var compatibilityRes *float64
		if userHasTest && candidateHasTest {
			compatibility, err := s.compatibilitySvc.CalculateDistance(*userVector, *profileVector)
			if err != nil {
				return nil, fmt.Errorf("get feed compatibility user id=%d: %w", profile.UserID, err)
			}
			compatibilityRes = &compatibility
		}

		item := model.FeedItem{
			UserID: profile.UserID, Name: profile.Name,
			Age:           CalculateAge(profile.CurrentVersion.BirthDate, time.Now()),
			AboutMe:       profile.CurrentVersion.AboutMe,
			DatingIntent:  profile.CurrentVersion.DatingGoal,
			Compatibility: compatibilityRes,
			Tags:          make([]string, 0, len(profile.Tags)),
			Photos:        make([]model.FeedPhoto, 0, len(profile.Photos)),
		}

		for _, tag := range profile.Tags {
			item.Tags = append(item.Tags, tag.Name)
		}

		for _, photo := range profile.Photos {

			url, err := s.media.GetURL(ctx, photo.StorageKey)

			if err != nil {
				return nil, fmt.Errorf("get feed photo id=%d: %w", photo.ID, err)
			}

			item.Photos = append(item.Photos, model.FeedPhoto{ID: photo.ID, URL: url})
		}

		page.Items = append(page.Items, item)
	}

	return page, nil
}

// CalculateAge вычисляет число полных лет на заданную дату.
// Принимает: дату рождения birthDate и дату расчёта today.
// Возвращает: возраст в годах с учётом наступления дня рождения.
func CalculateAge(birthDate, today time.Time) int {
	age := today.Year() - birthDate.Year()
	if today.Month() < birthDate.Month() || (today.Month() == birthDate.Month() && today.Day() < birthDate.Day()) {
		age--
	}
	return age
}

// feedBigFive извлекает все пять координат из психопрофиля.
// Принимает: психопрофиль psycho, который может быть nil.
// Возвращает: вектор и true, если все координаты заданы, иначе nil и false; диапазон значений не проверяет.
func feedBigFive(psycho *model.ProfilePsycho) (*model.BigFive, bool) {
	if psycho == nil || psycho.Openness == nil || psycho.Conscientiousness == nil ||
		psycho.Extraversion == nil || psycho.Agreeableness == nil || psycho.Neuroticism == nil {
		return nil, false
	}
	return &model.BigFive{
		Openness: psycho.Openness, Conscientiousness: psycho.Conscientiousness,
		Extraversion: psycho.Extraversion, Agreeableness: psycho.Agreeableness,
		Neuroticism: psycho.Neuroticism,
	}, true
}

// AddProfilePsycho добавляет новую ревизию психопрофиля в отдельной транзакции.
// Принимает: контекст ctx, ID профиля profileID и данные результата input.
// Возвращает: nil при успехе или ошибку; ответы на вопросы этот метод не сохраняет.
func (s *ProfileServiceImpl) AddProfilePsycho(ctx context.Context, profileID int64, psycho *model.ProfilePsychoInput) error {
	if err := s.profileRepo.AddProfilePsycho(ctx, profileID, psycho); err != nil {
		return fmt.Errorf("add profile psycho profile id=%d: %w", profileID, err)
	}
	return nil
}
