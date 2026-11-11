package timepad

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"dating-app/internal/model"
	"dating-app/internal/timepad"
)

func TestGetEventsQuery(t *testing.T) {
	startsAt := time.Date(2026, 11, 23, 18, 30, 0, 0, time.FixedZone("MSK", 3*60*60))
	var got *http.Request

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"total":1,"values":[{"id":7,"name":"Онлайн-лекция","starts_at":"2026-11-23T19:00:00+0300","url":"https://t.pad/7"}]}`))
	}))
	t.Cleanup(srv.Close)

	p := timepad.NewTimepadProvider(srv.Client(), srv.URL, "token")
	city, skip, limit := "Москва", 0, 10
	cats := []int64{374, 460}

	page, err := p.GetEvents(context.Background(), &model.EventQuery{City: &city, Skip: &skip, Limit: &limit, CategoryIDs: &cats, StartsAtMin: &startsAt})
	if err != nil {
		t.Fatal(err)
	}

	q := got.URL.Query()
	want := map[string]string{
		"cities":        "Москва",
		"skip":          "0",
		"limit":         "10",
		"category_ids":  "374,460",
		"fields":        "location",
		"sort":          "starts_at",
		"starts_at_min": "2026-11-23T18:30:00+0300",
	}
	for key, value := range want {
		if q.Get(key) != value {
			t.Errorf("%s = %q, want %q (query %s)", key, q.Get(key), value, got.URL.RawQuery)
		}
	}
	for _, key := range []string{"City", "Skip", "Limit"} {
		if q.Has(key) {
			t.Errorf("Timepad ignores capitalized %s", key)
		}
	}
	if got.Header.Get("Authorization") != "Bearer token" {
		t.Errorf("Authorization = %q", got.Header.Get("Authorization"))
	}

	// событие без location (онлайн) не должно ронять разбор ответа
	if len(page.Events) != 1 || page.Events[0].Location.City != "" || page.Events[0].Location.Address != nil {
		t.Fatalf("events = %+v", page.Events)
	}
}
