package mapper

import (
	"bytes"

	profilepb "dating-app/gen/go/profile/v1"
	"fmt"
	"io"
	"time"
)

func PhotoFromProto(response *profilepb.GetPhotoResponse) (io.ReadSeekCloser, time.Time, error) {
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
