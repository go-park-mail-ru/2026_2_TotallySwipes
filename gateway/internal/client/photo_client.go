package client

import (
	"dating-app/gateway/internal/client/mapper"

	"context"
	profilepb "dating-app/gen/go/profile/v1"

	"io"
	"time"
)

type PhotoAdapter struct {
	client profilepb.ProfileServiceClient
}

func NewPhotoAdapter(client profilepb.ProfileServiceClient) *PhotoAdapter {
	return &PhotoAdapter{client: client}
}

func (a *PhotoAdapter) OpenPhoto(ctx context.Context, key string) (io.ReadSeekCloser, time.Time, error) {
	response, err := a.client.GetPhoto(ctx, &profilepb.GetPhotoRequest{Key: key})

	if err != nil {
		return nil, time.Time{}, err
	}

	return mapper.PhotoFromProto(response)
}
