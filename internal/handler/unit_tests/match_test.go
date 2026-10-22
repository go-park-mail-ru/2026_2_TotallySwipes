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

	"github.com/gorilla/mux"
)

type fakeMatchService struct {
	page    *model.MatchPage
	err     error
	userID  int64
	limit   int
	cursor  *int64
	matchID int64
	summary *model.MatchSummary
}

func (f *fakeMatchService) GetSummary(_ context.Context, userID int64) (*model.MatchSummary, error) {
	f.userID = userID
	return f.summary, f.err
}

func (f *fakeMatchService) MarkSeen(_ context.Context, userID, matchID int64) error {
	f.userID, f.matchID = userID, matchID
	return f.err
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

func TestMatchHandlerMarkSeen(t *testing.T) {
	for _, tc := range []struct {
		name        string
		target      string
		err         error
		status      int
		wantMatchID int64
	}{
		{"ok", "/api/v1/matches/15/seen", nil, http.StatusNoContent, 15},
		{"bad id", "/api/v1/matches/abc/seen", nil, http.StatusBadRequest, 0},
		{"zero id", "/api/v1/matches/0/seen", nil, http.StatusBadRequest, 0},
		{"not found", "/api/v1/matches/15/seen", model.ErrMatchNotFound, http.StatusNotFound, 15},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc := &fakeMatchService{err: tc.err}
			h := NewMatchHandler(svc)
			r := mux.NewRouter()
			r.HandleFunc("/api/v1/matches/{match_id}/seen", func(w http.ResponseWriter, req *http.Request) { h.MarkSeen(w, req, 7) })
			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, tc.target, nil))
			if rec.Code != tc.status || svc.matchID != tc.wantMatchID {
				t.Fatalf("status = %d, match id = %d; want %d, %d; body = %s", rec.Code, svc.matchID, tc.status, tc.wantMatchID, rec.Body)
			}
		})
	}
}

func TestMatchHandlerSummary(t *testing.T) {
	for _, tc := range []struct {
		name    string
		summary *model.MatchSummary
		err     error
		status  int
		want    string
	}{
		{"has new", &model.MatchSummary{Total: 12, New: 3}, nil, http.StatusOK, `{"total":12,"new_count":3,"has_new":true}`},
		{"all seen", &model.MatchSummary{Total: 12}, nil, http.StatusOK, `{"total":12,"new_count":0,"has_new":false}`},
		{"no matches", &model.MatchSummary{}, nil, http.StatusOK, `{"total":0,"new_count":0,"has_new":false}`},
		{"internal error", nil, context.DeadlineExceeded, http.StatusInternalServerError, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc := &fakeMatchService{summary: tc.summary, err: tc.err}
			rec := httptest.NewRecorder()
			NewMatchHandler(svc).Summary(rec, httptest.NewRequest(http.MethodGet, "/api/v1/matches/summary", nil), 7)
			if rec.Code != tc.status || svc.userID != 7 || (tc.want != "" && rec.Body.String() != tc.want) {
				t.Fatalf("status = %d, user = %d, body = %s", rec.Code, svc.userID, rec.Body)
			}
		})
	}
}
