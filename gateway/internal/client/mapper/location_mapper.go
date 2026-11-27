package mapper

import (
	"dating-app/gateway/internal/model"
	profilepb "dating-app/gen/go/profile/v1"
	"fmt"
)

func LocationFromProto(location *profilepb.Location, userID int64) (*model.UserLocation, error) {
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

func CoordinatesToProto(coordinates *model.Coordinates) (*profilepb.Coordinates, error) {

	if coordinates == nil {
		return nil, fmt.Errorf("missing coordinates")
	}

	return &profilepb.Coordinates{Latitude: &coordinates.Latitude, Longitude: &coordinates.Longitude}, nil
}

func LocationPreviewFromProto(response *profilepb.PreviewLocationResponse, userID int64, coordinates model.Coordinates) (*model.UserLocationPreview, error) {

	if response == nil {
		return nil, fmt.Errorf("proto: missing location preview")
	}

	return &model.UserLocationPreview{PreviewID: response.GetPreviewId(), UserID: userID, Address: model.ResolvedAddress{City: response.GetCity(), District: response.District}, Coordinates: coordinates}, nil
}
