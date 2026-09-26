package service

import (
	"context"
	"dating-app/internal/model"
	"fmt"
	"time"
)

type ProfileService interface {
	GetNextFeed(ctx context.Context, userID int64, limit int, cursor *int64) (*model.FeedPage, error)
}

type ProfileRepository interface {
	GetByIDCurrentProfile(ctx context.Context, id int64) (*model.Profile, error)

	CreateProfile(ctx context.Context, input *model.ProfileInput) (int64, error)

	AddProfileVersion(ctx context.Context, profileID int64, version *model.ProfileVersionInput) error
	AddProfilePsycho(ctx context.Context, profileID int64, psycho *model.ProfilePsychoInput) error

	SetProfileTags(ctx context.Context, profileID int64, tagIDs []int64) error
	AddProfilePhotos(ctx context.Context, profileID int64, photos []model.PhotoInput) error

	GetProfilesByCursorAndLimit(ctx context.Context, userID int64, limit int, cursor *int64) ([]model.Profile, *int64, error)
}

// Нужно реализовать
type PhotoURLProvider interface {
	GetURL(ctx context.Context, storageKey string) (string, error)
}

type ProfileServiceImpl struct {
	profileRepo ProfileRepository
	media       PhotoURLProvider
}

func NewProfileService(repo ProfileRepository, media PhotoURLProvider) *ProfileServiceImpl {
	return &ProfileServiceImpl{profileRepo: repo, media: media}
}

func (s *ProfileServiceImpl) GetNextFeed(ctx context.Context, userID int64, limit int, cursor *int64) (*model.FeedPage, error) {

	if userID <= 0 || limit < 1 || (cursor != nil && *cursor <= 0) {
		return nil, fmt.Errorf("get feed: invalid user ID, limit or cursor")
	}

	profiles, nextCursor, err := s.profileRepo.GetProfilesByCursorAndLimit(ctx, userID, limit, cursor)

	if err != nil {
		return nil, fmt.Errorf("get feed profiles: %w", err)
	}

	page := &model.FeedPage{Items: make([]model.FeedItem, 0, len(profiles)), NextCursor: nextCursor}
	for _, profile := range profiles {

		item := model.FeedItem{
			UserID: profile.UserID, Name: profile.Name,
			Age:     calculateAge(profile.CurrentVersion.BirthDate, time.Now()),
			AboutMe: profile.CurrentVersion.AboutMe,
			Tags:    make([]string, 0, len(profile.Tags)),
			Photos:  make([]model.FeedPhoto, 0, len(profile.Photos)),
		}

		for _, tag := range profile.Tags {
			item.Tags = append(item.Tags, tag.Name)
		}

		for _, photo := range profile.Photos {

			// реализвать minio provider или как то иначе
			if s.media == nil {
				return nil, fmt.Errorf("get feed: photo URL provider is nil")
			}

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

func calculateAge(birthDate, today time.Time) int {
	age := today.Year() - birthDate.Year()
	if today.Month() < birthDate.Month() || (today.Month() == birthDate.Month() && today.Day() < birthDate.Day()) {
		age--
	}
	return age
}
