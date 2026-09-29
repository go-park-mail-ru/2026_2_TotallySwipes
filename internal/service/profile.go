package service

import (
	"context"
	"dating-app/internal/model"
	"errors"
	"fmt"
	"io/fs"
	"net/url"
	"strings"
	"time"
)

type ProfileService interface {
	GetNextFeed(ctx context.Context, userID int64, limit int, cursor *int64) (*model.FeedPage, error)

	AddProfilePsycho(ctx context.Context, profileID int64, psycho *model.ProfilePsychoInput) error
	GetByUserIDCurrentProfile(ctx context.Context, id int64) (*model.Profile, error)
}

type ProfileRepository interface {
	GetByIDCurrentProfile(ctx context.Context, id int64) (*model.Profile, error)
	GetByUserIDCurrentProfile(ctx context.Context, id int64) (*model.Profile, error)

	CreateProfile(ctx context.Context, input *model.ProfileInput) (int64, error)

	AddProfileVersion(ctx context.Context, profileID int64, version *model.ProfileVersionInput) error
	AddProfilePsycho(ctx context.Context, profileID int64, psycho *model.ProfilePsychoInput) error

	SetProfileTags(ctx context.Context, profileID int64, tagIDs []int64) error
	AddProfilePhotos(ctx context.Context, profileID int64, photos []model.PhotoInput) error

	GetProfilesByCursorAndLimit(ctx context.Context, userID int64, limit int, cursor *int64) ([]model.Profile, *int64, error)
}

type URLProvider interface {
	GetURL(ctx context.Context, storageKey string) (string, error)
}

func (p *LocalPhotoURLProvider) GetURL(ctx context.Context, storageKey string) (string, error) {

	if storageKey == "." || !fs.ValidPath(storageKey) || strings.Contains(storageKey, "\\") {
		return "", fmt.Errorf("invalid photo storage key: %q", storageKey)
	}
	photoPath := (&url.URL{Path: "/cats/" + storageKey}).EscapedPath()
	return strings.TrimRight(p.mediaBaseURL, "/") + photoPath, nil
}

type LocalPhotoURLProvider struct {
	mediaBaseURL string
}

func NewLocalPhotoURLProvider(baseURL string) *LocalPhotoURLProvider {
	return &LocalPhotoURLProvider{
		mediaBaseURL: baseURL,
	}
}

type ProfileServiceImpl struct {
	profileRepo      ProfileRepository
	media            URLProvider
	compatibilitySvc CompatibilityService
}

func NewProfileService(repo ProfileRepository, svc CompatibilityService, media URLProvider) *ProfileServiceImpl {
	return &ProfileServiceImpl{profileRepo: repo, compatibilitySvc: svc, media: media}
}

func (s *ProfileServiceImpl) GetByUserIDCurrentProfile(ctx context.Context, id int64) (*model.Profile, error) {
	return s.profileRepo.GetByUserIDCurrentProfile(ctx, id)
}

func (s *ProfileServiceImpl) GetNextFeed(ctx context.Context, userID int64, limit int, cursor *int64) (*model.FeedPage, error) {

	if userID <= 0 || limit < 1 || limit > 10 || (cursor != nil && *cursor <= 0) {
		return nil, fmt.Errorf("%w: invalid user ID, limit or cursor", ErrInvalidFeedRequest)
	}

	userProfile, err := s.profileRepo.GetByUserIDCurrentProfile(ctx, userID)
	if errors.Is(err, model.ErrNotFound) {
		return nil, ErrProfileRequired
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

// CalculateAge returns age in full calendar years at today.
func CalculateAge(birthDate, today time.Time) int {
	age := today.Year() - birthDate.Year()
	if today.Month() < birthDate.Month() || (today.Month() == birthDate.Month() && today.Day() < birthDate.Day()) {
		age--
	}
	return age
}

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

func (s *ProfileServiceImpl) AddProfilePsycho(ctx context.Context, profileID int64, psycho *model.ProfilePsychoInput) error {
	return s.profileRepo.AddProfilePsycho(ctx, profileID, psycho)
}
