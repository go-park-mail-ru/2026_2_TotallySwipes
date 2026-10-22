package service

import (
	"context"
	"dating-app/internal/model"

	"github.com/google/uuid"
)

type LocationRepository interface {
	Upsert(ctx context.Context, location *model.UserLocation) error
}

type ReverseGeocoder interface {
	Reverse(ctx context.Context, coordinates *model.Coordinates) (*model.ResolvedAddress, error)
}

type LocationPreviewRepository interface {
	Save(ctx context.Context, userID int64, previewID string, preview *model.UserLocationPreview) error
	Get(ctx context.Context, userID int64, previewID string) (*model.UserLocationPreview, error)
}

type LocationService struct {
	locationRepo LocationRepository
	geocoder     ReverseGeocoder
	previewRepo  LocationPreviewRepository
}

func NewLocationService(repo LocationRepository, geocoder ReverseGeocoder, previews LocationPreviewRepository) *LocationService {
	return &LocationService{locationRepo: repo, geocoder: geocoder, previewRepo: previews}
}

func (s *LocationService) GetLocationPreview(ctx context.Context, userID int64, coordinates *model.Coordinates) (*model.UserLocationPreview, error) {

	if coordinates == nil {
		return nil, model.ErrInvalidCoordinates
	}
	if err := coordinates.Validate(); err != nil {
		return nil, err
	}

	address, err := s.geocoder.Reverse(ctx, coordinates)

	if err != nil {
		return nil, err
	}

	previewID := uuid.New().String()

	preview := &model.UserLocationPreview{
		PreviewID:   previewID,
		UserID:      userID,
		Address:     *address,
		Coordinates: *coordinates,
	}

	err = s.previewRepo.Save(ctx, userID, previewID, preview)

	if err != nil {
		return nil, err
	}

	return preview, nil
}

func (s *LocationService) SaveLocation(ctx context.Context, userID int64, previewID string) (*model.UserLocation, error) {

	preview, err := s.previewRepo.Get(ctx, userID, previewID)

	if err != nil {
		return nil, err
	}

	location := &model.UserLocation{
		UserID:      userID,
		City:        preview.Address.City,
		District:    preview.Address.District,
		Coordinates: preview.Coordinates,
	}

	err = s.locationRepo.Upsert(ctx, location)

	if err != nil {
		return nil, err
	}

	return location, nil
}
