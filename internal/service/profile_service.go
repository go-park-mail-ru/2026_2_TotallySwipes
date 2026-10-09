package service

import (
	"context"
	"dating-app/internal/model"
	"errors"
	"fmt"
	"log/slog"
	"time"
)

type PhotoStorage interface {
	// Save сохраняет файл и возвращает его ключ в хранилище.
	Save(ctx context.Context, data []byte, ext string) (key string, err error)
	Delete(ctx context.Context, key string) error
}

type ProfileRepository interface {
	// GetByUserIDCurrentProfile возвращает анкету с тегами и фото или model.ErrNotFound.
	GetByUserIDCurrentProfile(ctx context.Context, userID int64) (*model.Profile, error)
	// PatchProfile сохраняет новую версию анкеты; ошибки model.ErrNotFound и model.ErrUnknownTag.
	PatchProfile(ctx context.Context, userID int64, patch *model.ProfilePatch) error
	// AddPhoto добавляет фото в конец; model.ErrPhotoLimit, если фото уже model.MaxPhotos.
	AddPhoto(ctx context.Context, userID int64, storageKey string) error
	// DeletePhoto удаляет фото и возвращает ключ его файла; model.ErrPhotoNotFound и model.ErrLastPhoto.
	DeletePhoto(ctx context.Context, userID, photoID int64) (string, error)
	// GetProfilesByCursorAndLimit возвращает страницу заполненных анкет и курсор следующей (nil в конце).
	GetProfilesByCursorAndLimit(ctx context.Context, userID int64, limit int, cursor *int64) ([]model.Profile, *int64, error)
}

type ProfileService struct {
	profileRepo      ProfileRepository
	media            URLProvider
	photos           PhotoStorage
	compatibilitySvc CompatibilityService
}

// NewProfileService создаёт сервис профилей и ленты.
// Принимает: репозиторий repo, сервис совместимости svc, поставщик URL media и хранилище файлов photos.
// Возвращает: экземпляр ProfileService.
func NewProfileService(repo ProfileRepository, svc CompatibilityService, media URLProvider, photos PhotoStorage) *ProfileService {
	return &ProfileService{profileRepo: repo, compatibilitySvc: svc, media: media, photos: photos}
}

// GetByUserIDCurrentProfile загружает профиль с актуальной версией, психопрофилем, тегами и фото.
// Принимает: контекст ctx и ID пользователя.
// Возвращает: профиль или ошибку; при отсутствии — model.ErrNotFound.
func (s *ProfileService) GetByUserIDCurrentProfile(ctx context.Context, id int64) (*model.Profile, error) {
	profile, err := s.profileRepo.GetByUserIDCurrentProfile(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("get user profile id=%d: %w", id, err)
	}
	return profile, nil
}

// GetMyProfile получает анкету пользователя целиком, с URL фотографий.
// Принимает: контекст ctx и ID пользователя userID.
// Возвращает: профиль или ошибку; при отсутствии профиля — model.ErrProfileRequired.
func (s *ProfileService) GetMyProfile(ctx context.Context, userID int64) (*model.Profile, error) {
	profile, err := s.profileRepo.GetByUserIDCurrentProfile(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("get my profile user id=%d: %w", userID, profileRequired(err))
	}
	if err := s.fillPhotoURLs(ctx, profile.Photos); err != nil {
		return nil, fmt.Errorf("get my profile user id=%d: %w", userID, err)
	}
	return profile, nil
}

// UpdateProfile сохраняет новую версию анкеты с наложенным патчем.
// Принимает: контекст ctx, ID пользователя userID и непустой проверенный patch.
// Возвращает: обновлённый профиль или ошибку, включая model.ErrUnknownTag.
func (s *ProfileService) UpdateProfile(ctx context.Context, userID int64, patch *model.ProfilePatch) (*model.Profile, error) {
	if err := s.profileRepo.PatchProfile(ctx, userID, patch); err != nil {
		return nil, fmt.Errorf("update profile user id=%d: %w", userID, profileRequired(err))
	}
	return s.GetMyProfile(ctx, userID)
}

// AddPhoto сохраняет файл и добавляет фото в конец списка анкеты.
// Принимает: контекст ctx, ID пользователя userID и проверенный upload.
// Возвращает: актуальный список фото или ошибку, включая model.ErrPhotoLimit.
func (s *ProfileService) AddPhoto(ctx context.Context, userID int64, upload model.PhotoUpload) ([]model.Photo, error) {
	key, err := s.photos.Save(ctx, upload.Data, upload.Ext)
	if err != nil {
		return nil, fmt.Errorf("add photo user id=%d: save file: %w", userID, err)
	}

	if err := s.profileRepo.AddPhoto(ctx, userID, key); err != nil {
		s.deleteFile(ctx, key)
		return nil, fmt.Errorf("add photo user id=%d: %w", userID, profileRequired(err))
	}
	return s.myPhotos(ctx, userID)
}

// DeletePhoto удаляет фото анкеты и его файл.
// Принимает: контекст ctx, ID пользователя userID и ID фото photoID.
// Возвращает: актуальный список фото или ошибку, включая model.ErrPhotoNotFound и model.ErrLastPhoto.
func (s *ProfileService) DeletePhoto(ctx context.Context, userID, photoID int64) ([]model.Photo, error) {
	key, err := s.profileRepo.DeletePhoto(ctx, userID, photoID)
	if err != nil {
		return nil, fmt.Errorf("delete photo id=%d user id=%d: %w", photoID, userID, profileRequired(err))
	}
	// Запись уже удалена; если файл не удалился, он просто останется сиротой
	s.deleteFile(ctx, key)
	return s.myPhotos(ctx, userID)
}

// Missing возвращает обязательные поля, которых не хватает анкете пользователя.
// Принимает: контекст ctx и ID пользователя userID.
// Возвращает: список полей (пустой - анкета заполнена) или ошибку.
func (s *ProfileService) Missing(ctx context.Context, userID int64) ([]string, error) {
	profile, err := s.profileRepo.GetByUserIDCurrentProfile(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("profile missing user id=%d: %w", userID, err)
	}
	return profile.Missing(), nil
}

func (s *ProfileService) myPhotos(ctx context.Context, userID int64) ([]model.Photo, error) {
	profile, err := s.GetMyProfile(ctx, userID)
	if err != nil {
		return nil, err
	}
	return profile.Photos, nil
}

func (s *ProfileService) fillPhotoURLs(ctx context.Context, photos []model.Photo) error {
	for i := range photos {
		url, err := s.media.GetURL(ctx, photos[i].StorageKey)
		if err != nil {
			return fmt.Errorf("photo id=%d url: %w", photos[i].ID, err)
		}
		photos[i].URL = url
	}
	return nil
}

func (s *ProfileService) deleteFile(ctx context.Context, key string) {
	if err := s.photos.Delete(context.WithoutCancel(ctx), key); err != nil {
		slog.Error("delete photo file", "key", key, "error", err)
	}
}

// GetShortProfile получает краткий профиль: имя, возраст, главное фото, missing.
// Принимает: контекст ctx и ID пользователя userID.
// Возвращает: краткий профиль или ошибку; при отсутствии профиля — model.ErrProfileRequired.
func (s *ProfileService) GetShortProfile(ctx context.Context, userID int64) (*model.ProfileShort, error) {
	profile, err := s.GetMyProfile(ctx, userID)
	if err != nil {
		return nil, err
	}

	fields := profile.CurrentVersion.ProfileFields
	short := &model.ProfileShort{
		UserID:        profile.UserID,
		Name:          fields.Name,
		Missing:       profile.Missing(),
	}
	if fields.BirthDate != nil {
		age := model.AgeAt(*fields.BirthDate, time.Now())
		short.Age = &age
	}
	if len(profile.Photos) > 0 {
		short.MainPhotoURL = &profile.Photos[0].URL
	}
	return short, nil
}

// GetNextFeed формирует страницу анкет с возрастом, URL фото и совместимостью при наличии обоих векторов.
// Принимает: контекст ctx, ID пользователя userID, limit от 1 до 10 и необязательный cursor.
// Возвращает: страницу ленты или ошибку, включая model.ErrInvalidFeedRequest и model.ErrProfileRequired.
func (s *ProfileService) GetNextFeed(ctx context.Context, userID int64, limit int, cursor *int64) (*model.FeedPage, error) {

	if userID <= 0 || limit < 1 || limit > 10 || (cursor != nil && *cursor <= 0) {
		return nil, fmt.Errorf("%w: invalid user ID, limit or cursor", model.ErrInvalidFeedRequest)
	}

	userProfile, err := s.profileRepo.GetByUserIDCurrentProfile(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("get feed viewer profile: %w", profileRequired(err))
	}

	if len(userProfile.Missing()) > 0 {
		return nil, fmt.Errorf("get feed user id=%d: profile incomplete: %w", userID, model.ErrProfileRequired)
	}
	userVector, userHasTest := feedBigFive(userProfile.CurrentPsycho)
	profiles, nextCursor, err := s.profileRepo.GetProfilesByCursorAndLimit(ctx, userID, limit, cursor)

	if err != nil {
		return nil, fmt.Errorf("get feed profiles: %w", err)
	}

	page := &model.FeedPage{Items: make([]model.FeedItem, 0, len(profiles)), NextCursor: nextCursor}
	for _, profile := range profiles {
		// SQL уже отбирает заполненные анкеты; проверка страхует от расхождения
		// SQL-фильтра с Missing, иначе ниже разыменовался бы nil
		if missing := profile.Missing(); len(missing) > 0 {
			slog.Warn("feed: incomplete profile passed SQL filter", "user_id", profile.UserID, "missing", missing)
			continue
		}

		profileVector, candidateHasTest := feedBigFive(profile.CurrentPsycho)
		var compatibilityRes *float64
		if userHasTest && candidateHasTest {
			compatibility, err := s.compatibilitySvc.CalculateDistance(*userVector, *profileVector)
			if err != nil {
				return nil, fmt.Errorf("get feed compatibility user id=%d: %w", profile.UserID, err)
			}
			compatibilityRes = &compatibility
		}

		// В ленту попадают только заполненные анкеты, обязательные поля заданы
		fields := profile.CurrentVersion.ProfileFields
		item := model.FeedItem{
			UserID:        profile.UserID,
			Name:          *fields.Name,
			Age:           model.AgeAt(*fields.BirthDate, time.Now()),
			AboutMe:       fields.AboutMe,
			DatingGoal:    *fields.DatingGoal,
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

// profileRequired переводит отсутствие анкеты в ошибку, которую видит клиент (409 PROFILE_REQUIRED).
// Остальные ошибки возвращает без изменений.
func profileRequired(err error) error {
	if errors.Is(err, model.ErrNotFound) {
		return model.ErrProfileRequired
	}
	return err
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
