package repository

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"reflect"
	"testing"
	"time"

	"dating-app/internal/model"
	"dating-app/internal/repository"
	"dating-app/internal/service"
	"github.com/go-redis/redismock/v9"
)

var _ service.LocationPreviewRepository = (*repository.LocationPreviewRepository)(nil)

func newLocationPreviewMock(t *testing.T) (*repository.LocationPreviewRepository, redismock.ClientMock) {
	t.Helper()
	client, mock := redismock.NewClientMock()
	t.Cleanup(func() {
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Error(err)
		}
		client.Close()
	})
	return repository.NewLocationPreviewRepository(client, 10*time.Minute), mock
}

func locationPreviewFixture() model.UserLocationPreview {
	return model.UserLocationPreview{
		PreviewID: "preview-id", UserID: 42,
		Address:     model.ResolvedAddress{City: "Хотьково"},
		Coordinates: model.Coordinates{Latitude: 56.251234567, Longitude: 37.991234567},
	}
}

func TestLocationPreviewRepositorySave(t *testing.T) {
	for _, fail := range []bool{false, true} {
		name := "success"
		if fail {
			name = "redis error"
		}
		t.Run(name, func(t *testing.T) {
			repo, mock := newLocationPreviewMock(t)
			preview := locationPreviewFixture()
			data, err := json.Marshal(preview)
			if err != nil {
				t.Fatal(err)
			}
			expected := mock.ExpectSet("location:preview:42:preview-id", data, 10*time.Minute)
			backendErr := errors.New("Redis unavailable")
			if fail {
				expected.SetErr(backendErr)
			} else {
				expected.SetVal("OK")
			}
			err = repo.Save(context.Background(), 42, "preview-id", &preview)
			if fail && !errors.Is(err, backendErr) {
				t.Fatalf("got %v, want Redis error", err)
			}
			if !fail && err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestLocationPreviewRepositorySaveEncodingError(t *testing.T) {
	repo, _ := newLocationPreviewMock(t)
	preview := locationPreviewFixture()
	preview.Coordinates.Latitude = math.NaN()
	err := repo.Save(context.Background(), 42, "preview-id", &preview)
	var encodingErr *json.UnsupportedValueError
	if !errors.As(err, &encodingErr) {
		t.Fatalf("got %v, want JSON encoding error", err)
	}
}

func TestLocationPreviewRepositoryGet(t *testing.T) {
	backendErr := errors.New("Redis unavailable")
	for _, name := range []string{"success", "missing", "other user", "redis error", "invalid JSON"} {
		t.Run(name, func(t *testing.T) {
			repo, mock := newLocationPreviewMock(t)
			userID := int64(42)
			key := "location:preview:42:preview-id"
			if name == "other user" {
				userID = 43
				key = "location:preview:43:preview-id"
			}
			expected := mock.ExpectGet(key)
			want := locationPreviewFixture()
			switch name {
			case "success":
				data, err := json.Marshal(want)
				if err != nil {
					t.Fatal(err)
				}
				expected.SetVal(string(data))
			case "missing", "other user":
				expected.RedisNil()
			case "redis error":
				expected.SetErr(backendErr)
			case "invalid JSON":
				expected.SetVal("{invalid")
			}
			got, err := repo.Get(context.Background(), userID, "preview-id")
			switch name {
			case "success":
				if err != nil || !reflect.DeepEqual(got, &want) {
					t.Fatalf("got %+v, %v; want %+v", got, err, want)
				}
			case "missing", "other user":
				if got != nil || !errors.Is(err, model.ErrLocationPreviewUnavailable) {
					t.Fatalf("got %+v, %v", got, err)
				}
			case "redis error":
				if got != nil || !errors.Is(err, backendErr) {
					t.Fatalf("got %+v, %v", got, err)
				}
			case "invalid JSON":
				var syntaxErr *json.SyntaxError
				if got != nil || !errors.As(err, &syntaxErr) {
					t.Fatalf("got %+v, %v", got, err)
				}
			}
		})
	}
}
