package s3

import (
	"bytes"
	"context"
	"dating-app/internal/config"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type MinIOStorage struct {
	Client *minio.Client
	Bucket string
}

type MinIOURLProvider struct {
	storage *MinIOStorage
	ttl     time.Duration
}

func OpenMinIOStorage(cfg config.MinIOConfig) (*MinIOStorage, error) {

	client, err := minio.New(cfg.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""),
		Secure: cfg.UseTLS,
	})

	if err != nil {
		return nil, fmt.Errorf("create minio client: %w", err)
	}

	return &MinIOStorage{
		Client: client,
		Bucket: cfg.Bucket,
	}, nil

}

func NewMinIOURLProvider(storage *MinIOStorage, ttl time.Duration) *MinIOURLProvider {
	return &MinIOURLProvider{
		storage: storage,
		ttl:     ttl,
	}
}

func (p *MinIOURLProvider) GetPhotoURL(ctx context.Context, key string) (string, error) {

	u, err := p.storage.Client.PresignedGetObject(ctx, p.storage.Bucket, key, p.ttl, nil)
	if err != nil {
		return "", fmt.Errorf("presigned get object: %w", err)
	}

	return u.String(), nil
}

func (s *MinIOStorage) Save(ctx context.Context, data []byte, ext string) (key string, err error) {

	contentType, err := photoContentType(ext)
	if err != nil {
		return "", fmt.Errorf("get photo content type: %w", err)
	}

	key = "photos/" + uuid.New().String() + ext

	_, err = s.Client.PutObject(ctx, s.Bucket, key, bytes.NewReader(data), int64(len(data)), minio.PutObjectOptions{ContentType: contentType})

	if err != nil {
		return "", fmt.Errorf("put object: %w", err)
	}

	return key, nil
}

func (s *MinIOStorage) Delete(ctx context.Context, key string) error {

	err := s.Client.RemoveObject(ctx, s.Bucket, key, minio.RemoveObjectOptions{})

	if err != nil {
		return fmt.Errorf("remove object: %w", err)
	}

	return nil
}

func photoContentType(ext string) (string, error) {
	switch ext {
	case ".jpg", ".jpeg":
		return "image/jpeg", nil
	case ".png":
		return "image/png", nil
	case ".webp":
		return "image/webp", nil
	default:
		return "", fmt.Errorf("unsupported extension %q", ext)
	}
}
