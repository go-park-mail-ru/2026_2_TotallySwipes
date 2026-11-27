package client

import (
	"context"
	"dating-app/gateway/internal/client/mapper"
	"dating-app/gateway/internal/model"
	profilepb "dating-app/gen/go/profile/v1"
	"fmt"
)

type LocationAdapter struct {
	client profilepb.ProfileServiceClient
}

func NewLocationAdapter(client profilepb.ProfileServiceClient) *LocationAdapter {
	return &LocationAdapter{client: client}
}

func (a *LocationAdapter) GetLocationPreview(ctx context.Context, userID int64, coordinates *model.Coordinates) (*model.UserLocationPreview, error) {

	coordinatesProto, err := mapper.CoordinatesToProto(coordinates)

	if err != nil {
		return nil, err
	}

	response, err := a.client.PreviewLocation(ctx, &profilepb.PreviewLocationRequest{UserId: userID, Coordinates: coordinatesProto})

	if err != nil {
		return nil, err
	}

	return mapper.LocationPreviewFromProto(response, userID, *coordinates)
}

func (a *LocationAdapter) SaveLocation(ctx context.Context, userID int64, previewID string) (*model.UserLocation, error) {
	response, err := a.client.SaveLocation(ctx, &profilepb.SaveLocationRequest{UserId: userID, PreviewId: previewID})

	if err != nil {
		return nil, err
	}

	if response.GetLocation() == nil {
		return nil, fmt.Errorf("proto: missing location")
	}

	return mapper.LocationFromProto(response.GetLocation(), userID)
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

	return mapper.LocationFromProto(response.GetLocation(), userID)
}
