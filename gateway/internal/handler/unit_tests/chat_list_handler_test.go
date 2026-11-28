package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"dating-app/gateway/internal/handler"
	"dating-app/gateway/internal/model"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type fakeChatListService struct {
	handler.ChaHTTPService
	page   *model.ChatPage
	err    error
	userID int64
	query  model.ChatListQuery
	called bool
}

func (s *fakeChatListService) GetChats(_ context.Context, userID int64, query model.ChatListQuery) (*model.ChatPage, error) {
	s.called = true
	s.userID = userID
	s.query = query
	return s.page, s.err
}

func TestChatListHandler(t *testing.T) {

	next := "20"
	for _, tc := range []struct {
		name  string
		query string
		page  *model.ChatPage
		err   error
		code  int
	}{
		{name: "page", query: "?limit=20&cursor=0", page: &model.ChatPage{Items: []model.ChatCard{{ChatID: 42}}, NextCursor: &next}, code: 200},
		{name: "empty", page: &model.ChatPage{}, code: 200},
		{name: "invalid query", query: "?limit=101", code: 400},
		{name: "nil page", code: 500},
		{name: "service error", err: status.Error(codes.PermissionDenied, "denied"), code: 403},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc := &fakeChatListService{page: tc.page, err: tc.err}
			h := handler.NewChatHandler(svc)
			response := httptest.NewRecorder()
			h.List(response, httptest.NewRequest(http.MethodGet, "/api/v1/chats"+tc.query, nil), 7)

			if response.Code != tc.code {
				t.Fatalf("status=%d body=%s", response.Code, response.Body)
			}

			if tc.name == "invalid query" {
				if svc.called {
					t.Fatal("invalid query reached service")
				}
				return
			}

			if svc.userID != 7 {
				t.Fatal("authenticated user ID lost")
			}

			if tc.code != 200 {
				return
			}

			var body struct {
				Items      []map[string]any `json:"items"`
				NextCursor *string          `json:"next_cursor"`
			}
			if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
				t.Fatal(err)
			}

			if body.Items == nil {
				t.Fatal("items must be an array")
			}
			if tc.name == "page" {
				if svc.query.Limit == nil || *svc.query.Limit != 20 || svc.query.Cursor == nil || *svc.query.Cursor != "0" || len(body.Items) != 1 || body.Items[0]["chat_id"] != "42" || body.NextCursor == nil || *body.NextCursor != next {
					t.Fatalf("unexpected query or response: %+v %s", svc.query, response.Body)
				}
			} else if len(body.Items) != 0 || body.NextCursor != nil || svc.query.Limit != nil || svc.query.Cursor != nil {
				t.Fatalf("unexpected empty response or defaults: %+v %s", svc.query, response.Body)
			}
		})
	}
}
