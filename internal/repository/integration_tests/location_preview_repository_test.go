package repository_test

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"dating-app/internal/model"
	"dating-app/internal/repository"
	"dating-app/internal/service"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

var _ service.LocationPreviewRepository = (*repository.LocationPreviewRepository)(nil)

func TestLocationPreviewRepository(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { rdb.Close() })
	repo := repository.NewLocationPreviewRepository(rdb, 10*time.Minute)
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
	if ttl := mr.TTL("location:preview:42:preview-id"); ttl <= 0 || ttl > 10*time.Minute {
		t.Fatalf("unexpected TTL: %v", ttl)
	}
	if _, err := repo.Get(ctx, 43, "preview-id"); !errors.Is(err, model.ErrLocationPreviewUnavailable) {
		t.Fatalf("another user's preview: %v", err)
	}
	if _, err := repo.Get(ctx, 42, "missing"); !errors.Is(err, model.ErrLocationPreviewUnavailable) {
		t.Fatalf("missing preview: %v", err)
	}
	mr.FastForward(10 * time.Minute)
	if _, err := repo.Get(ctx, 42, "preview-id"); !errors.Is(err, model.ErrLocationPreviewUnavailable) {
		t.Fatalf("expired preview: %v", err)
	}
	mr.Close()
	if _, err := repo.Get(ctx, 42, "preview-id"); err == nil || errors.Is(err, model.ErrLocationPreviewUnavailable) {
		t.Fatalf("Redis failure must remain infrastructure error: %v", err)
	}
}
