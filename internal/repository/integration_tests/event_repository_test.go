package repository_test

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	"dating-app/internal/model"
	"dating-app/internal/repository"
	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

func TestEventSearchConcurrentLifecycle(t *testing.T) {
	server := miniredis.RunT(t)
	server.SetTime(time.Now())
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	repo := repository.NewEventRepository(client, 24*time.Hour)
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
	server.FastForward(time.Hour)
	ttl := server.TTL(key)
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
	if !stored.ExpiresAt.Equal(deadline) || server.TTL(key) != ttl {
		t.Fatal("TTL extended")
	}
	if server.Exists("place-search:active:42") {
		t.Fatal("active pointer retained")
	}
	newer, _, err := repo.CreateSearch(ctx, candidate())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repo.SubmitResults(ctx, 10, 42, search.SearchID, []int64{2, 1}); !errors.Is(err, model.ErrResultAlreadySubmitted) {
		t.Fatal(err)
	}
	active, err = repo.GetActive(ctx, 42)
	if err != nil || active.SearchID != newer.SearchID {
		t.Fatal("retry affected new search")
	}
	if _, err := repo.SubmitResults(ctx, 10, 42, search.SearchID, []int64{1}); !errors.Is(err, model.ErrResultAlreadySubmitted) {
		t.Fatalf("changed result: %v", err)
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
	server.FastForward(24 * time.Hour)
	if _, err := repo.GetBySearchID(ctx, newer.SearchID); !errors.Is(err, model.ErrNotFound) {
		t.Fatalf("expired: %v", err)
	}
}

// Redis emulator checks supplement mocks; they do not replace tests against a deployed Redis.
func TestEventRepositoryRejectsInvalidResults(t *testing.T) {
	for _, name := range []string{"duplicate IDs", "unknown ID", "wrong match", "outsider", "cancelled", "expired deadline", "repeated identical", "repeated changed", "missing"} {
		t.Run(name, func(t *testing.T) {
			server := miniredis.RunT(t)
			server.SetTime(time.Now())
			client := redis.NewClient(&redis.Options{Addr: server.Addr()})
			t.Cleanup(func() { _ = client.Close() })
			repo := repository.NewEventRepository(client, time.Hour)
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
				server.Del(key)
			}
			before, _ := server.Get(key)
			_, err = repo.SubmitResults(ctx, user, match, search.SearchID, likes)
			if !errors.Is(err, want) {
				t.Fatalf("error=%v want=%v", err, want)
			}
			after, _ := server.Get(key)
			if before != after {
				t.Fatal("rejected request changed stored state")
			}
		})
	}
}

func TestEventRepositoryCreationAndPointerSafety(t *testing.T) {
	server := miniredis.RunT(t)
	server.SetTime(time.Now())
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	repo := repository.NewEventRepository(client, time.Hour)
	ctx := context.Background()
	candidate := func() *model.PlaceSearch {
		return &model.PlaceSearch{SearchID: uuid.New(), MatchID: 42, City: "Москва", Status: model.SearchStatusVoting, Events: []model.Event{{ID: 1}}, Participants: [2]model.ParticipantResult{{UserID: 10}, {UserID: 20}}}
	}
	if _, _, err := repo.CreateSearch(ctx, nil); err == nil {
		t.Fatal("nil search accepted")
	}
	if _, _, err := repository.NewEventRepository(client, 0).CreateSearch(ctx, candidate()); err == nil {
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
	if server.TTL(key) != server.TTL("place-search:active:42") {
		t.Fatal("deadlines differ")
	}
	server.FastForward(time.Minute)
	ttl := server.TTL(key)
	same, created, err := repo.CreateSearch(ctx, candidate())
	if err != nil || created || same.SearchID != search.SearchID || server.TTL(key) != ttl {
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
	if got, _ := server.Get("place-search:active:42"); got != newer {
		t.Fatal("deleted another search pointer")
	}
	// An orphan pointer whose state no longer exists can be replaced.
	recovered, created, err := repo.CreateSearch(ctx, candidate())
	if err != nil || !created || recovered.SearchID == search.SearchID {
		t.Fatalf("orphan recovery: %v", err)
	}
}
