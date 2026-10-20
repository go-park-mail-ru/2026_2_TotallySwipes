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
	client *minio.Client
	bucket string
	ttl    time.Duration
}

func OpenMinIOStorage(ctx context.Context, cfg config.MinIOConfig) (*MinIOStorage, error) {

	client, err := minio.New(cfg.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""),
		Secure: cfg.UseTLS,
	})

	if err != nil {
		return nil, fmt.Errorf("create minio client: %w", err)
	}

	exists, err := client.BucketExists(ctx, cfg.Bucket)
	if err != nil {
		return nil, fmt.Errorf("check bucket %q: %w", cfg.Bucket, err)
	}

	if !exists {
		err := client.MakeBucket(ctx, cfg.Bucket, minio.MakeBucketOptions{})
		if err != nil {
			if minio.ToErrorResponse(err).Code != "BucketAlreadyOwnedByYou" {
				return nil, fmt.Errorf("create bucket %q: %w", cfg.Bucket, err)
			}
		}
	}

	return &MinIOStorage{
		Client: client,
		Bucket: cfg.Bucket,
	}, nil

}

func NewMinIOURLProvider(cfg config.MinIOConfig) (*MinIOURLProvider, error) {
	client, err := minio.New(cfg.PublicEndpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""),
		Secure: cfg.UseTLS,
	})

	if err != nil {
		return nil, fmt.Errorf("create photo URL client: %w", err)
	}

	return &MinIOURLProvider{
		client: client,
		bucket: cfg.Bucket,
		ttl:    cfg.PhotoURLTTL,
	}, nil
}

func (p *MinIOURLProvider) GetPhotoURL(ctx context.Context, key string) (string, error) {

	u, err := p.client.PresignedGetObject(ctx, p.bucket, key, p.ttl, nil)
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
