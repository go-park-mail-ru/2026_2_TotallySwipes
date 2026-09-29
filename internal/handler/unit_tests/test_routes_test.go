package handler_test

import (
	"context"
	"dating-app/internal/auth"
	"dating-app/internal/handler"
	"dating-app/internal/middleware"
	"dating-app/internal/model"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/mux"
)

type testServiceStub struct {
	userID int64
	testID int64
}

func (s *testServiceStub) GetCurrentTest(context.Context) (*model.Test, error) {
	test := model.NewTIPITest(42)
	test.Title = "TIPI"
	test.Questions = []model.Question{{ID: 101, Body: "Вопрос"}}
	return &test, nil
}
func (s *testServiceStub) SubmitTestAnswers(_ context.Context, userID int64, answers *model.TestAnswers) (*model.TestResult, error) {
	s.userID, s.testID = userID, answers.TestID
	return &model.TestResult{ID: 9, TestID: answers.TestID, Revision: 1, CompletedA: time.Now()}, nil
}

func TestProtectedTestRoutes(t *testing.T) {
	issuer := auth.NewJWTIssuer("test-secret", time.Minute)
	token, _, err := issuer.Issue(73, "session")
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, method, path, token, contentType, body string
		status                                       int
	}{
		{"get without cookie", "GET", "/api/v1/tests/current", "", "", "", 401},
		{"get invalid token", "GET", "/api/v1/tests/current", "invalid", "", "", 401},
		{"get current", "GET", "/api/v1/tests/current", token, "", "", 200},
		{"post without cookie", "POST", "/api/v1/tests/42/results", "", "application/json", "{}", 401},
		{"post wrong content type", "POST", "/api/v1/tests/42/results", token, "text/plain", "{}", 415},
		{"post invalid id", "POST", "/api/v1/tests/no/results", token, "application/json", "{}", 400},
		{"post answers", "POST", "/api/v1/tests/42/results", token, "application/json; charset=utf-8", `{"answers":[{"question_id":"101","value":4}]}`, 201},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc := &testServiceStub{}
			r := mux.NewRouter()
			requireAuth := middleware.Auth(issuer)
			r.Handle("/api/v1/tests/current", requireAuth(handler.NewGetCurrentTestHandler(svc))).Methods(http.MethodGet)
			r.Handle("/api/v1/tests/{test_id}/results", requireAuth(handler.NewTestAnswersHandler(svc))).Methods(http.MethodPost)
			req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
			if tc.token != "" {
				req.AddCookie(&http.Cookie{Name: "access_token", Value: tc.token})
			}
			req.Header.Set("Content-Type", tc.contentType)
			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, req)
			if rec.Code != tc.status {
				t.Fatalf("status = %d, want %d: %s", rec.Code, tc.status, rec.Body.String())
			}
			if rec.Header().Get("Content-Type") != "application/json" {
				t.Fatalf("expected JSON: %s", rec.Body.String())
			}
			if tc.status == 201 && (svc.userID != 73 || svc.testID != 42) {
				t.Fatalf("service received userID=%d testID=%d", svc.userID, svc.testID)
			}
			if tc.status != 201 && svc.userID != 0 {
				t.Fatal("unexpected submission")
			}
		})
	}
}
