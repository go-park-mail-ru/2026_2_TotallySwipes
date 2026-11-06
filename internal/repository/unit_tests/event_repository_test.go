package repository

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"testing"
	"time"

	"dating-app/internal/model"
	"dating-app/internal/repository"
	"github.com/go-redis/redismock/v9"
	"github.com/google/uuid"
)

func TestEventRepositoryReads(t *testing.T) {
	failure := errors.New("redis unavailable")
	for _, name := range []string{"by ID", "missing ID", "bad JSON", "get error", "active", "no active", "dangling pointer", "bad UUID", "pointer error"} {
		t.Run(name, func(t *testing.T) {
			client, mock := redismock.NewClientMock()
			t.Cleanup(func() {
				if err := mock.ExpectationsWereMet(); err != nil {
					t.Error(err)
				}
				_ = client.Close()
			})
			repo := repository.NewEventRepository(client, time.Hour)
			state := model.PlaceSearch{SearchID: uuid.New(), MatchID: 42, City: "Москва"}
			data, _ := json.Marshal(state)
			key := "place-search:" + state.SearchID.String()
			active := name == "active" || name == "no active" || name == "dangling pointer" || name == "bad UUID" || name == "pointer error"
			switch name {
			case "no active":
				mock.ExpectGet("place-search:active:42").RedisNil()
			case "bad UUID":
				mock.ExpectGet("place-search:active:42").SetVal("broken")
			case "pointer error":
				mock.ExpectGet("place-search:active:42").SetErr(failure)
			case "active", "dangling pointer":
				mock.ExpectGet("place-search:active:42").SetVal(state.SearchID.String())
			}
			switch name {
			case "by ID", "active":
				mock.ExpectGet(key).SetVal(string(data))
			case "missing ID", "dangling pointer":
				mock.ExpectGet(key).RedisNil()
			case "bad JSON":
				mock.ExpectGet(key).SetVal("{")
			case "get error":
				mock.ExpectGet(key).SetErr(failure)
			}
			var got *model.PlaceSearch
			var err error
			if active {
				got, err = repo.GetActive(context.Background(), 42)
			} else {
				got, err = repo.GetBySearchID(context.Background(), state.SearchID)
			}
			switch name {
			case "by ID", "active":
				if err != nil || !reflect.DeepEqual(got, &state) {
					t.Fatalf("got=%+v err=%v", got, err)
				}
			case "no active", "dangling pointer":
				if got != nil || err != nil {
					t.Fatalf("got=%+v err=%v", got, err)
				}
			case "missing ID":
				if !errors.Is(err, model.ErrNotFound) {
					t.Fatal(err)
				}
			case "pointer error", "get error":
				if !errors.Is(err, failure) {
					t.Fatal(err)
				}
			default:
				if err == nil {
					t.Fatal("expected decoding error")
				}
			}
		})
	}
}
