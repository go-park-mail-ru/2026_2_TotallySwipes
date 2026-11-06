package service

import (
	"context"
	"errors"
	"fmt"
	"math"
	"reflect"
	"testing"

	"dating-app/internal/model"
	"dating-app/internal/service"
	"github.com/google/uuid"
	"go.uber.org/mock/gomock"
)

func TestLocationServicePreviewInvalidCoordinates(t *testing.T) {
	for _, tc := range []struct {
		name        string
		coordinates *model.Coordinates
	}{
		{"nil", nil},
		{"latitude", &model.Coordinates{Latitude: 91}},
		{"longitude", &model.Coordinates{Longitude: -181}},
		{"NaN", &model.Coordinates{Latitude: math.NaN()}},
		{"infinity", &model.Coordinates{Longitude: math.Inf(1)}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			// Никакие зависимости не должны вызываться при неверных координатах.
			svc := service.NewLocationService(NewMockLocationRepository(ctrl), NewMockReverseGeocoder(ctrl), NewMockLocationPreviewRepository(ctrl))
			got, err := svc.GetLocationPreview(context.Background(), 42, tc.coordinates)
			if got != nil || !errors.Is(err, model.ErrInvalidCoordinates) {
				t.Fatalf("got %+v, %v", got, err)
			}
		})
	}
}

func TestLocationServicePreview(t *testing.T) {
	redisErr := errors.New("Redis unavailable")
	for _, tc := range []struct {
		name            string
		geoErr, saveErr error
	}{
		{"success", nil, nil},
		{"quota", fmt.Errorf("provider: %w", model.ErrGeocoderQuotaExceeded), nil},
		{"not found", model.ErrLocationNotFound, nil},
		{"city missing", model.ErrCityNotFound, nil},
		{"geocoder unavailable", errors.New("timeout"), nil},
		{"Redis unavailable", nil, redisErr},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			repo := NewMockLocationRepository(ctrl)
			geo := NewMockReverseGeocoder(ctrl)
			previews := NewMockLocationPreviewRepository(ctrl)
			svc := service.NewLocationService(repo, geo, previews)
			ctx := context.Background()
			coordinates := model.Coordinates{Latitude: 56.251234567, Longitude: 37.991234567}
			district := "Сергиево-Посадский городской округ"
			address := &model.ResolvedAddress{City: "Хотьково", District: &district}
			geoCall := geo.EXPECT().Reverse(ctx, &coordinates).Return(address, tc.geoErr)
			var saved model.UserLocationPreview
			if tc.geoErr == nil {
				previews.EXPECT().Save(ctx, int64(42), gomock.Any(), gomock.Any()).After(geoCall).
					DoAndReturn(func(_ context.Context, userID int64, id string, preview *model.UserLocationPreview) error {
						if _, err := uuid.Parse(id); err != nil {
							t.Fatalf("invalid preview ID: %q", id)
						}
						if preview.PreviewID != id || preview.UserID != userID || preview.Coordinates != coordinates || !reflect.DeepEqual(preview.Address, *address) {
							t.Fatalf("wrong saved preview: %+v", preview)
						}
						saved = *preview
						return tc.saveErr
					})
			}
			got, err := svc.GetLocationPreview(ctx, 42, &coordinates)
			wantErr := tc.geoErr
			if wantErr == nil {
				wantErr = tc.saveErr
			}
			if !errors.Is(err, wantErr) {
				t.Fatalf("error = %v, want %v", err, wantErr)
			}
			if wantErr != nil {
				if got != nil {
					t.Fatalf("returned preview after failure: %+v", got)
				}
			} else if !reflect.DeepEqual(got, &saved) {
				t.Fatalf("got %+v, want %+v", got, saved)
			}
			// Нет ожидания Upsert: preview не меняет сохранённое местоположение.
		})
	}
}

func TestLocationServiceSaveLocation(t *testing.T) {
	for _, tc := range []struct {
		name              string
		getErr, upsertErr error
	}{
		{"success", nil, nil},
		{"expired or unavailable", fmt.Errorf("preview: %w", model.ErrLocationPreviewUnavailable), nil},
		{"Redis failure", errors.New("Redis unavailable"), nil},
		{"database failure", nil, errors.New("database unavailable")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			repo := NewMockLocationRepository(ctrl)
			geo := NewMockReverseGeocoder(ctrl)
			previews := NewMockLocationPreviewRepository(ctrl)
			svc := service.NewLocationService(repo, geo, previews)
			ctx := context.Background()
			preview := &model.UserLocationPreview{
				PreviewID: "preview-id", UserID: 42,
				Address:     model.ResolvedAddress{City: "Хотьково"},
				Coordinates: model.Coordinates{Latitude: 56.251234567, Longitude: 37.991234567},
			}
			getCall := previews.EXPECT().Get(ctx, int64(42), "preview-id").Return(preview, tc.getErr)
			want := &model.UserLocation{UserID: 42, City: preview.Address.City, Coordinates: preview.Coordinates}
			if tc.getErr == nil {
				repo.EXPECT().Upsert(ctx, want).After(getCall).Return(tc.upsertErr)
			}
			got, err := svc.SaveLocation(ctx, 42, "preview-id")
			wantErr := tc.getErr
			if wantErr == nil {
				wantErr = tc.upsertErr
			}
			if !errors.Is(err, wantErr) {
				t.Fatalf("error = %v, want %v", err, wantErr)
			}
			if wantErr != nil {
				if got != nil {
					t.Fatalf("returned location after failure: %+v", got)
				}
			} else if !reflect.DeepEqual(got, want) {
				t.Fatalf("got %+v, want %+v", got, want)
			}
		})
	}
}
