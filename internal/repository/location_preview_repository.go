package repository

import (
	"context"
	"dating-app/internal/model"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

type LocationPreviewRepository struct {
	rdb *redis.Client
	ttl time.Duration
}

func NewLocationPreviewRepository(rdb *redis.Client, ttl time.Duration) *LocationPreviewRepository {
	return &LocationPreviewRepository{rdb: rdb, ttl: ttl}
}

func locationPreviewKey(userID int64, previewID string) string {
	return fmt.Sprintf("location:preview:%d:%s", userID, previewID)
}

func (r *LocationPreviewRepository) Save(ctx context.Context, userID int64, previewID string, preview *model.UserLocationPreview) error {

	data, err := json.Marshal(preview)

	if err != nil {
		return fmt.Errorf("encode preview: %w", err)
	}

	key := locationPreviewKey(userID, previewID)

	if err := r.rdb.Set(ctx, key, data, r.ttl).Err(); err != nil {
		return fmt.Errorf("save preview %w", err)
	}

	return nil
}

func (r *LocationPreviewRepository) Get(ctx context.Context, userID int64, previewID string) (*model.UserLocationPreview, error) {

	data, err := r.rdb.Get(ctx, locationPreviewKey(userID, previewID)).Bytes()

	if errors.Is(err, redis.Nil) {
		return nil, model.ErrLocationPreviewUnavailable
	}

	if err != nil {
		return nil, fmt.Errorf("get preview: %w", err)
	}

	var preview model.UserLocationPreview
	if err := json.Unmarshal(data, &preview); err != nil {
		return nil, fmt.Errorf("decode preview: %w", err)
	}

	return &preview, nil
}
