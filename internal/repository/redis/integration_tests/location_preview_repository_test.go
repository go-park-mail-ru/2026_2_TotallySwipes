//go:build integration

package redis_test

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"dating-app/internal/model"
	redisrepo "dating-app/internal/repository/redis"
	"dating-app/internal/service"
)

var _ service.LocationPreviewRepository = (*redisrepo.LocationPreviewRepository)(nil)

func TestLocationPreviewRepository(t *testing.T) {
	rdb := realRedis(t)
	t.Cleanup(func() { rdb.Close() })
	repo := redisrepo.NewLocationPreviewRepository(rdb, 10*time.Minute)
	ctx := context.Background()
	preview := model.UserLocationPreview{
		PreviewID:   "preview-id",
		UserID:      42,
		Address:     model.ResolvedAddress{City: "Хотьково"},
		Coordinates: model.Coordinates{Latitude: 56.251234567, Longitude: 37.991234567},
	}
	err := repo.Save(ctx, 42, "preview-id", &preview)
	if err != nil {
		t.Fatal(err)
	}
	want := preview
	for range 2 {
		got, err := repo.Get(ctx, 42, "preview-id")
		if err != nil || !reflect.DeepEqual(got, &want) {
			t.Fatalf("Get = %+v, %v; want %+v", got, err, want)
		}
	}
	if ttl := rdb.PTTL(ctx, "location:preview:42:preview-id").Val(); ttl <= 0 || ttl > 10*time.Minute {
		t.Fatalf("unexpected TTL: %v", ttl)
	}
	if _, err := repo.Get(ctx, 43, "preview-id"); !errors.Is(err, model.ErrLocationPreviewUnavailable) {
		t.Fatalf("another user's preview: %v", err)
	}
	if _, err := repo.Get(ctx, 42, "missing"); !errors.Is(err, model.ErrLocationPreviewUnavailable) {
		t.Fatalf("missing preview: %v", err)
	}
	expireKeys(t, rdb, "location:preview:42:preview-id")
	if _, err := repo.Get(ctx, 42, "preview-id"); !errors.Is(err, model.ErrLocationPreviewUnavailable) {
		t.Fatalf("expired preview: %v", err)
	}
	rdb.Close()
	if _, err := repo.Get(ctx, 42, "preview-id"); err == nil || errors.Is(err, model.ErrLocationPreviewUnavailable) {
		t.Fatalf("Redis failure must remain infrastructure error: %v", err)
	}
}
