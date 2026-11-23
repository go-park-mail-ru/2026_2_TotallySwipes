package dto

import (
	"dating-app/internal/model"
	"dating-app/internal/validate"
	"strings"
)

type LocationPreviewRequest struct {
	Latitude  *float64 `json:"latitude"`
	Longitude *float64 `json:"longitude"`
}

// ToModel проверяет наличие координат. Диапазоны проверяет сервис через Coordinates.Validate.
func (r LocationPreviewRequest) ToModel() (*model.Coordinates, map[string]string) {
	errs := make(map[string]string)
	if r.Latitude == nil {
		errs["latitude"] = validate.ErrRequired.Error()
	}

	if r.Longitude == nil {
		errs["longitude"] = validate.ErrRequired.Error()
	}

	if len(errs) > 0 {
		return nil, errs
	}

	return &model.Coordinates{Latitude: *r.Latitude, Longitude: *r.Longitude}, errs
}

type LocationPreviewResponse struct {
	PreviewID string  `json:"preview_id"`
	City      string  `json:"city"`
	District  *string `json:"district"`
}

func NewLocationPreviewResponse(preview *model.UserLocationPreview) LocationPreviewResponse {
	return LocationPreviewResponse{
		PreviewID: preview.PreviewID,
		City:      preview.Address.City,
		District:  preview.Address.District,
	}
}

type LocationConfirmRequest struct {
	PreviewID string `json:"preview_id"`
}

func (r LocationConfirmRequest) Validate() map[string]string {
	errs := make(map[string]string)

	if strings.TrimSpace(r.PreviewID) == "" {
		errs["preview_id"] = validate.ErrRequired.Error()
	}

	return errs
}

type LocationCoordinates struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}

type Location struct {
	City        string              `json:"city"`
	District    *string             `json:"district"`
	Coordinates LocationCoordinates `json:"coordinates"`
}

type LocationResponse struct {
	Location *Location `json:"location"`
}

// NewLocationResponse возвращает location: null, если местоположение отсутствует.
func NewLocationResponse(location *model.UserLocation) LocationResponse {

	if location == nil {
		return LocationResponse{}
	}

	return LocationResponse{Location: &Location{
		City:     location.City,
		District: location.District,
		Coordinates: LocationCoordinates{
			Latitude:  location.Coordinates.Latitude,
			Longitude: location.Coordinates.Longitude,
		},
	}}
}
