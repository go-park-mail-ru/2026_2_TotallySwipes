package client

import (
	"context"
	profilepb "dating-app/gen/go/profile/v1"
	"dating-app/internal/model"
	"fmt"
)

type LocationAdapter struct {
	client profilepb.ProfileServiceClient
}

func NewLocationAdapter(client profilepb.ProfileServiceClient) *LocationAdapter {
	return &LocationAdapter{client: client}
}

func (a *LocationAdapter) GetLocationPreview(ctx context.Context, userID int64, coordinates *model.Coordinates) (*model.UserLocationPreview, error) {

	if coordinates == nil {
		return nil, fmt.Errorf("missing coordinates")
	}
	response, err := a.client.PreviewLocation(
		ctx,
		&profilepb.PreviewLocationRequest{UserId: userID, Coordinates: &profilepb.Coordinates{Latitude: &coordinates.Latitude, Longitude: &coordinates.Longitude}})

	if err != nil {
		return nil, err
	}

	if response == nil {
		return nil, fmt.Errorf("proto: missing location preview")
	}

	return &model.UserLocationPreview{
		PreviewID:   response.GetPreviewId(),
		UserID:      userID,
		Address:     model.ResolvedAddress{City: response.GetCity(), District: response.District},
		Coordinates: *coordinates}, nil
}

func (a *LocationAdapter) SaveLocation(ctx context.Context, userID int64, previewID string) (*model.UserLocation, error) {
	response, err := a.client.SaveLocation(ctx, &profilepb.SaveLocationRequest{UserId: userID, PreviewId: previewID})

	if err != nil {
		return nil, err
	}

	if response.GetLocation() == nil {
		return nil, fmt.Errorf("proto: missing location")
	}

	return locationFromProto(response.GetLocation(), userID)
}

func (a *LocationAdapter) GetLocation(ctx context.Context, userID int64) (*model.UserLocation, error) {
	response, err := a.client.GetLocation(ctx, &profilepb.GetLocationRequest{UserId: userID})

	if err != nil {
		return nil, err
	}

	if response == nil {
		return nil, fmt.Errorf("proto: missing location response")
	}

	if response.GetLocation() == nil {
		return nil, nil
	}

	return locationFromProto(response.GetLocation(), userID)
}

func locationFromProto(location *profilepb.Location, userID int64) (*model.UserLocation, error) {
	coordinates := location.GetCoordinates()

	if coordinates == nil {
		return nil, fmt.Errorf("proto: missing location.coordinates")
	}

	if coordinates.Latitude == nil {
		return nil, fmt.Errorf("proto: missing coordinates.latitude")
	}

	if coordinates.Longitude == nil {
		return nil, fmt.Errorf("proto: missing coordinates.longitude")
	}

	return &model.UserLocation{
			UserID:      userID,
			City:        location.GetCity(),
			District:    location.District,
			Coordinates: model.Coordinates{Latitude: coordinates.GetLatitude(), Longitude: coordinates.GetLongitude()}},
		nil
}
