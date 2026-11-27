package s3

import (
	"bytes"
	"context"
	"fmt"
	"io/fs"
	"mime"
	"strings"
	"time"

	"dating-app/services/chat/internal/config"
	"dating-app/services/chat/internal/model"

	"github.com/google/uuid"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

const (
	minioRegion           = "us-east-1"
	voiceKeyPrefix        = "voice/"
	videoNoteKeyPrefix    = "video_note/"
	defaultDownloadURLTTL = 15 * time.Minute
	maxDownloadURLTTL     = 7 * 24 * time.Hour
)

type MinIOStorage struct {
	Client         *minio.Client
	Bucket         string
	downloadURLTTL time.Duration
}

func OpenMinIOStorage(ctx context.Context, cfg config.MinIOConfig) (*MinIOStorage, error) {

	if cfg.DownloadURLTTL == 0 {
		cfg.DownloadURLTTL = defaultDownloadURLTTL
	}

	if cfg.DownloadURLTTL < time.Second || cfg.DownloadURLTTL > maxDownloadURLTTL {
		return nil, fmt.Errorf("download URL TTL must be between one second and seven days")
	}

	client, err := minio.New(cfg.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""),
		Secure: cfg.UseTLS,
		Region: minioRegion,
	})

	if err != nil {
		return nil, fmt.Errorf("create minio client: %w", err)
	}

	exists, err := client.BucketExists(ctx, cfg.Bucket)

	if err != nil {
		return nil, fmt.Errorf("check bucket %q: %w", cfg.Bucket, err)
	}

	if !exists {

		err := client.MakeBucket(ctx, cfg.Bucket, minio.MakeBucketOptions{Region: minioRegion})

		if err != nil && minio.ToErrorResponse(err).Code != "BucketAlreadyOwnedByYou" {
			return nil, fmt.Errorf("create bucket %q: %w", cfg.Bucket, err)
		}
	}

	return &MinIOStorage{
		Client:         client,
		Bucket:         cfg.Bucket,
		downloadURLTTL: cfg.DownloadURLTTL,
	}, nil
}

func (s *MinIOStorage) Save(ctx context.Context, data []byte, mediaType model.ChatMessageType, contentType string) (string, error) {

	if len(data) == 0 {
		return "", fmt.Errorf("save media: empty data")
	}

	parsedType, _, err := mime.ParseMediaType(contentType)

	if err != nil {
		return "", fmt.Errorf("save media: invalid content type: %w", err)
	}

	var prefix string

	switch mediaType {

	case model.ChatMessageTypeVoice:

		if !strings.HasPrefix(parsedType, "audio/") {
			return "", fmt.Errorf("save voice: expected audio content type")
		}

		prefix = voiceKeyPrefix

	case model.ChatMessageTypeVideoNote:

		if !strings.HasPrefix(parsedType, "video/") {
			return "", fmt.Errorf("save video note: expected video content type")
		}

		prefix = videoNoteKeyPrefix

	default:
		return "", fmt.Errorf("save media: unsupported message type %q", mediaType)
	}

	key := prefix + uuid.NewString()

	_, err = s.Client.PutObject(ctx, s.Bucket, key, bytes.NewReader(data), int64(len(data)), minio.PutObjectOptions{ContentType: contentType})

	if err != nil {
		return "", fmt.Errorf("put media object: %w", err)
	}

	return key, nil
}

func (s *MinIOStorage) Delete(ctx context.Context, key string) error {

	if !validMediaKey(key) {
		return fmt.Errorf("delete media: invalid storage key")
	}

	if err := s.Client.RemoveObject(ctx, s.Bucket, key, minio.RemoveObjectOptions{}); err != nil {
		return fmt.Errorf("remove media object: %w", err)
	}

	return nil
}

func (s *MinIOStorage) GetPresignedDownloadURL(ctx context.Context, key string) (string, error) {

	if !validMediaKey(key) {
		return "", fmt.Errorf("get media URL: invalid storage key")
	}

	url, err := s.Client.PresignedGetObject(ctx, s.Bucket, key, s.downloadURLTTL, nil)

	if err != nil {
		return "", fmt.Errorf("presigned get media object: %w", err)
	}

	return url.String(), nil
}

func validMediaKey(key string) bool {

	if !fs.ValidPath(key) || strings.Contains(key, "\\") {
		return false
	}

	return (strings.HasPrefix(key, voiceKeyPrefix) && len(key) > len(voiceKeyPrefix)) ||
		(strings.HasPrefix(key, videoNoteKeyPrefix) && len(key) > len(videoNoteKeyPrefix))
}
