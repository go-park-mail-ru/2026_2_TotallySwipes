//go:build integration

package redis_test

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	"dating-app/internal/model"
	redisrepo "dating-app/internal/repository/redis"
	"github.com/google/uuid"
)

func TestEventSearchConcurrentLifecycle(t *testing.T) {
	client := realRedis(t)
	t.Cleanup(func() { _ = client.Close() })
	repo := redisrepo.NewEventRepository(client, 24*time.Hour)
	ctx := context.Background()
	candidate := func() *model.PlaceSearch {
		return &model.PlaceSearch{SearchID: uuid.New(), MatchID: 42, City: "Москва", Status: model.SearchStatusVoting,
			Events: []model.Event{{ID: 1}, {ID: 2}}, Participants: [2]model.ParticipantResult{{UserID: 10}, {UserID: 20}}}
	}
	active, err := repo.GetActive(ctx, 42)
	if err != nil || active != nil {
		t.Fatalf("empty active: %v %v", active, err)
	}
	type result struct {
		search  *model.PlaceSearch
		created bool
		err     error
	}
	results := make(chan result, 2)
	var wg sync.WaitGroup
	for range 2 {
		wg.Add(1)
		go func() { defer wg.Done(); s, c, e := repo.CreateSearch(ctx, candidate()); results <- result{s, c, e} }()
	}
	wg.Wait()
	close(results)
	var search *model.PlaceSearch
	created := 0
	for r := range results {
		if r.err != nil {
			t.Fatal(r.err)
		}
		if search != nil && search.SearchID != r.search.SearchID {
			t.Fatal("two searches created")
		}
		search = r.search
		if r.created {
			created++
		}
	}
	if created != 1 {
		t.Fatalf("created count: %d", created)
	}
	key := "place-search:" + search.SearchID.String()
	deadline := search.ExpiresAt
	// Сравниваем абсолютный срок истечения: реальное время продолжает идти.
	ttl := client.PExpireTime(ctx, key).Val()
	errs := make(chan error, 2)
	for _, user := range []int64{10, 20} {
		wg.Add(1)
		go func(user int64) {
			defer wg.Done()
			_, e := repo.SubmitResults(ctx, user, 42, search.SearchID, []int64{1, 2})
			errs <- e
		}(user)
	}
	wg.Wait()
	close(errs)
	for e := range errs {
		if e != nil {
			t.Fatal(e)
		}
	}
	stored, err := repo.GetBySearchID(ctx, search.SearchID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.Status != model.SearchStatusCompleted || !stored.Participants[0].Submitted || !stored.Participants[1].Submitted {
		t.Fatalf("lost result: %+v", stored)
	}
	if !stored.ExpiresAt.Equal(deadline) || client.PExpireTime(ctx, key).Val() != ttl {
		t.Fatal("TTL extended")
	}
	// Завершённый подбор остаётся активным до истечения TTL: участники видят совпадения
	active, err = repo.GetActive(ctx, 42)
	if err != nil || active == nil || active.SearchID != search.SearchID || active.Status != model.SearchStatusCompleted {
		t.Fatalf("completed search must stay active: %+v %v", active, err)
	}
	again, againCreated, err := repo.CreateSearch(ctx, candidate())
	if err != nil || againCreated || again.SearchID != search.SearchID || again.Status != model.SearchStatusCompleted {
		t.Fatalf("completed search replaced before TTL: %+v created=%v %v", again, againCreated, err)
	}
	if _, err := repo.SubmitResults(ctx, 10, 42, search.SearchID, []int64{2, 1}); !errors.Is(err, model.ErrResultAlreadySubmitted) {
		t.Fatal(err)
	}
	if _, err := repo.SubmitResults(ctx, 10, 42, search.SearchID, []int64{1}); !errors.Is(err, model.ErrResultAlreadySubmitted) {
		t.Fatalf("changed result: %v", err)
	}
	// После TTL подбор и указатель истекают, и для матча создаётся новый подбор
	expireKeys(t, client, key, "place-search:active:42")
	newer, newerCreated, err := repo.CreateSearch(ctx, candidate())
	if err != nil || !newerCreated || newer.SearchID == search.SearchID {
		t.Fatalf("new search after TTL: %+v created=%v %v", newer, newerCreated, err)
	}
	active, err = repo.GetActive(ctx, 42)
	if err != nil || active.SearchID != newer.SearchID {
		t.Fatal("new search is not active")
	}
	if _, err := repo.SubmitResults(ctx, 30, 42, newer.SearchID, []int64{}); !errors.Is(err, model.ErrNotFound) {
		t.Fatalf("unauthorized: %v", err)
	}
	if _, err := repo.SubmitResults(ctx, 10, 42, newer.SearchID, []int64{3}); !errors.Is(err, model.ErrInvalidEventResults) {
		t.Fatalf("invalid id: %v", err)
	}
	if _, err := repo.SubmitResults(ctx, 10, 42, newer.SearchID, []int64{}); err != nil {
		t.Fatal(err)
	}
	stored, err = repo.SubmitResults(ctx, 20, 42, newer.SearchID, []int64{})
	if err != nil || stored.Status != model.SearchStatusCompleted || stored.Participants[0].LikedEventsIDs == nil {
		t.Fatalf("empty results: %+v %v", stored, err)
	}
	expireKeys(t, client, "place-search:"+newer.SearchID.String(), "place-search:active:42")
	if _, err := repo.GetBySearchID(ctx, newer.SearchID); !errors.Is(err, model.ErrNotFound) {
		t.Fatalf("expired: %v", err)
	}
}

func TestEventRepositoryRejectsInvalidResults(t *testing.T) {
	for _, name := range []string{"duplicate IDs", "unknown ID", "wrong match", "outsider", "cancelled", "expired deadline", "repeated identical", "repeated changed", "missing"} {
		t.Run(name, func(t *testing.T) {
			client := realRedis(t)
			t.Cleanup(func() { _ = client.Close() })
			repo := redisrepo.NewEventRepository(client, time.Hour)
			ctx := context.Background()
			search := &model.PlaceSearch{SearchID: uuid.New(), MatchID: 42, City: "Москва", Status: model.SearchStatusVoting,
				Events: []model.Event{{ID: 1}, {ID: 2}}, Participants: [2]model.ParticipantResult{{UserID: 10}, {UserID: 20}}}
			search, _, err := repo.CreateSearch(ctx, search)
			if err != nil {
				t.Fatal(err)
			}
			likes := []int64{1}
			user, match := int64(10), int64(42)
			want := model.ErrInvalidEventResults
			switch name {
			case "duplicate IDs":
				likes = []int64{1, 1}
			case "unknown ID":
				likes = []int64{99}
			case "wrong match":
				match = 43
				want = model.ErrNotFound
			case "outsider":
				user = 30
				want = model.ErrNotFound
			case "cancelled":
				search.Status = model.SearchStatusCancelled
				want = model.ErrSearchStateConflict
			case "expired deadline":
				search.ExpiresAt = time.Now().Add(-time.Second)
				want = model.ErrNotFound
			case "repeated identical", "repeated changed":
				search.Participants[0].Submitted = true
				search.Participants[0].LikedEventsIDs = []int64{1}
				want = model.ErrResultAlreadySubmitted
				if name == "repeated changed" {
					likes = []int64{2}
				}
			case "missing":
				want = model.ErrNotFound
			}
			key := "place-search:" + search.SearchID.String()
			data, err := json.Marshal(search)
			if err != nil {
				t.Fatal(err)
			}
			if err := client.Set(ctx, key, data, time.Hour).Err(); err != nil {
				t.Fatal(err)
			}
			if name == "missing" {
				client.Del(ctx, key)
			}
			before, _ := client.Get(ctx, key).Result()
			_, err = repo.SubmitResults(ctx, user, match, search.SearchID, likes)
			if !errors.Is(err, want) {
				t.Fatalf("error=%v want=%v", err, want)
			}
			after, _ := client.Get(ctx, key).Result()
			if before != after {
				t.Fatal("rejected request changed stored state")
			}
		})
	}
}

func TestEventRepositoryCreationAndPointerSafety(t *testing.T) {
	client := realRedis(t)
	t.Cleanup(func() { _ = client.Close() })
	repo := redisrepo.NewEventRepository(client, time.Hour)
	ctx := context.Background()
	candidate := func() *model.PlaceSearch {
		return &model.PlaceSearch{SearchID: uuid.New(), MatchID: 42, City: "Москва", Status: model.SearchStatusVoting, Events: []model.Event{{ID: 1}}, Participants: [2]model.ParticipantResult{{UserID: 10}, {UserID: 20}}}
	}
	if _, _, err := repo.CreateSearch(ctx, nil); err == nil {
		t.Fatal("nil search accepted")
	}
	if _, _, err := redisrepo.NewEventRepository(client, 0).CreateSearch(ctx, candidate()); err == nil {
		t.Fatal("zero TTL accepted")
	}
	original := candidate()
	search, created, err := repo.CreateSearch(ctx, original)
	if err != nil || !created {
		t.Fatalf("create: %v", err)
	}
	if !original.ExpiresAt.IsZero() {
		t.Fatal("input mutated")
	}
	key := "place-search:" + search.SearchID.String()
	if client.PExpireTime(ctx, key).Val() != client.PExpireTime(ctx, "place-search:active:42").Val() {
		t.Fatal("deadlines differ")
	}

	ttl := client.PExpireTime(ctx, key).Val()
	same, created, err := repo.CreateSearch(ctx, candidate())
	if err != nil || created || same.SearchID != search.SearchID || client.PExpireTime(ctx, key).Val() != ttl {
		t.Fatalf("reopen: %+v %v", same, err)
	}
	// A stale operation must never delete a pointer belonging to a newer search.
	newer := uuid.New().String()
	if err := client.Set(ctx, "place-search:active:42", newer, time.Hour).Err(); err != nil {
		t.Fatal(err)
	}
	for _, user := range []int64{10, 20} {
		if _, err := repo.SubmitResults(ctx, user, 42, search.SearchID, []int64{}); err != nil {
			t.Fatal(err)
		}
	}
	if got, _ := client.Get(ctx, "place-search:active:42").Result(); got != newer {
		t.Fatal("deleted another search pointer")
	}
	// An orphan pointer whose state no longer exists can be replaced.
	recovered, created, err := repo.CreateSearch(ctx, candidate())
	if err != nil || !created || recovered.SearchID == search.SearchID {
		t.Fatalf("orphan recovery: %v", err)
	}
}
