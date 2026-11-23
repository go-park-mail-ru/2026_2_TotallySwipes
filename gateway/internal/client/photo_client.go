package client

import (
	"bytes"
	"context"
	profilepb "dating-app/gen/go/profile/v1"
	"fmt"
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

	if response == nil {
		return nil, time.Time{}, fmt.Errorf("proto: missing photo response")
	}

	if err := response.GetLastModified().CheckValid(); err != nil {
		return nil, time.Time{}, fmt.Errorf("proto: invalid photo.last_modified: %w", err)
	}

	return &photoReader{Reader: bytes.NewReader(response.GetData())}, response.GetLastModified().AsTime(), nil
}

type photoReader struct {
	*bytes.Reader
}

func (r *photoReader) Close() error {
	return nil
}
