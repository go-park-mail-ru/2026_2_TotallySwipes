package redis_test

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"testing"
	"time"

	"dating-app/internal/model"
	redisrepo "dating-app/internal/repository/redis"
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
			repo := redisrepo.NewEventRepository(client, time.Hour)
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

func TestGetPartnerFinishedByMatchIDs(t *testing.T) {
	for _, name := range []string{"partner finished", "only current finished", "second participant", "missing", "expired", "cancelled", "dangling", "redis error", "bad JSON", "outsider", "empty"} {
		t.Run(name, func(t *testing.T) {
			client, mock := redismock.NewClientMock()
			t.Cleanup(func() {
				if err := mock.ExpectationsWereMet(); err != nil {
					t.Error(err)
				}
				_ = client.Close()
			})
			repo := redisrepo.NewEventRepository(client, time.Hour)
			ids := []int64{42, 43, 42}
			state := model.PlaceSearch{
				SearchID: uuid.New(), MatchID: 42, Status: model.SearchStatusVoting,
				ExpiresAt:    time.Now().Add(time.Hour),
				Participants: [2]model.ParticipantResult{{UserID: 1}, {UserID: 2, Submitted: true}},
			}
			userID := int64(1)
			want, wantErr := false, false
			switch name {
			case "partner finished":
				want = true
			case "only current finished":
				state.Participants[0].Submitted = true
				state.Participants[1].Submitted = false
			case "second participant":
				userID = 2
				state.Participants[0].Submitted = true
				want = true
			case "expired":
				state.ExpiresAt = time.Now().Add(-time.Hour)
			case "cancelled":
				state.Status = model.SearchStatusCancelled
			case "outsider":
				userID = 99
				wantErr = true
			case "redis error", "bad JSON":
				wantErr = true
			case "empty":
				ids = nil
			}
			if name != "empty" {
				call := mock.ExpectMGet("place-search:active:42", "place-search:active:43")
				switch name {
				case "redis error":
					call.SetErr(errors.New("redis unavailable"))
				case "missing":
					call.SetVal([]interface{}{nil, nil})
				default:
					call.SetVal([]interface{}{state.SearchID.String(), nil})
					data, _ := json.Marshal(state)
					var value interface{} = string(data)
					if name == "bad JSON" {
						value = "{"
					}
					if name == "dangling" {
						value = nil
					}
					mock.ExpectMGet("place-search:" + state.SearchID.String()).SetVal([]interface{}{value})
				}
			}
			got, err := repo.GetPartnerFinishedByMatchIDs(context.Background(), userID, ids)
			if (err != nil) != wantErr {
				t.Fatalf("error=%v wantErr=%v", err, wantErr)
			}
			if !wantErr {
				expected := map[int64]bool{42: want, 43: false}
				if name == "empty" {
					expected = map[int64]bool{}
				}
				if !reflect.DeepEqual(got, expected) {
					t.Fatalf("got=%v want=%v", got, expected)
				}
			}
		})
	}
}
