package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"

	. "dating-app/internal/handler"
	"dating-app/internal/model"
)

type fakeMatchService struct {
	page   *model.MatchPage
	err    error
	userID int64
	limit  int
	cursor *int64
}

func (f *fakeMatchService) GetMatches(_ context.Context, userID int64, limit int, cursor *int64) (*model.MatchPage, error) {
	f.userID, f.limit, f.cursor = userID, limit, cursor
	return f.page, f.err
}

func TestMatchHandlerList(t *testing.T) {
	name, age, url, compatibility := "Anna", 25, "https://media.example/cats/anna.jpg", 0.87
	created := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)
	next, cursor := int64(4), int64(9)

	t.Run("ok", func(t *testing.T) {
		svc := &fakeMatchService{page: &model.MatchPage{
			Items: []model.Match{
				{ID: 5, UserID: 8, Name: &name, Age: &age, MainPhotoURL: &url, Compatibility: &compatibility, IsNew: true, CreatedAt: created},
				{ID: 4, UserID: 9},
			},
			Total:      12,
			NextCursor: &next,
		}}
		rec := httptest.NewRecorder()
		NewMatchHandler(svc).List(rec, httptest.NewRequest(http.MethodGet, "/api/v1/matches?limit=2&cursor=9", nil), 7)
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, body = %s", rec.Code, rec.Body)
		}
		if svc.userID != 7 || svc.limit != 2 || !reflect.DeepEqual(svc.cursor, &cursor) {
			t.Fatalf("service got user=%d limit=%d cursor=%v", svc.userID, svc.limit, svc.cursor)
		}
		var got map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
			t.Fatal(err)
		}
		want := map[string]any{
			"next_cursor": float64(4),
			"total":       float64(12),
			"items": []any{
				map[string]any{"match_id": float64(5), "user_id": float64(8), "name": "Anna", "age": float64(25),
					"photo_url": url, "compatibility": 0.87, "is_new": true, "created_at": "2026-10-18T12:00:00Z"},
				map[string]any{"match_id": float64(4), "user_id": float64(9), "name": nil, "age": nil,
					"photo_url": nil, "compatibility": nil, "is_new": false, "created_at": "0001-01-01T00:00:00Z"},
			},
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("body = %v, want %v", got, want)
		}
	})

	t.Run("empty with default limit", func(t *testing.T) {
		svc := &fakeMatchService{page: &model.MatchPage{}}
		rec := httptest.NewRecorder()
		NewMatchHandler(svc).List(rec, httptest.NewRequest(http.MethodGet, "/api/v1/matches", nil), 7)
		if svc.limit != model.MatchesDefaultLimit || svc.cursor != nil {
			t.Fatalf("service got limit=%d cursor=%v", svc.limit, svc.cursor)
		}
		if rec.Code != http.StatusOK || rec.Body.String() != `{"items":[],"total":0,"next_cursor":null}` {
			t.Fatalf("status = %d, body = %s", rec.Code, rec.Body)
		}
	})

	for _, tc := range []struct {
		name   string
		target string
		err    error
		status int
	}{
		{"bad limit", "/api/v1/matches?limit=0", nil, http.StatusBadRequest},
		{"limit too big", "/api/v1/matches?limit=49", nil, http.StatusBadRequest},
		{"bad cursor", "/api/v1/matches?cursor=x", nil, http.StatusBadRequest},
		{"invalid request", "/api/v1/matches", model.ErrInvalidMatchesRequest, http.StatusBadRequest},
		{"internal error", "/api/v1/matches", context.DeadlineExceeded, http.StatusInternalServerError},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			NewMatchHandler(&fakeMatchService{err: tc.err}).List(rec, httptest.NewRequest(http.MethodGet, tc.target, nil), 7)
			if rec.Code != tc.status {
				t.Fatalf("status = %d, want %d, body = %s", rec.Code, tc.status, rec.Body)
			}
		})
	}
}
