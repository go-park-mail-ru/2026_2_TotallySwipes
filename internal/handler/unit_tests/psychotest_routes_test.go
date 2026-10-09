package handler_test

import (
	"context"
	"dating-app/internal/auth"
	"dating-app/internal/handler"
	"dating-app/internal/middleware"
	"dating-app/internal/model"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/mux"
)

type psychoTestServiceStub struct {
	userID int64
	testID int64
}

var stubScores = [5]float64{.85, .8, .25, .55, .2}

func stubTestResult(testID int64) *model.TestResult {
	return &model.TestResult{ID: 9, TestID: testID, Revision: 1, CompletedA: time.Now(),
		PersonalityType: model.PersonalityStrategist, AboutPersonalityType: "Стратег: вам ближе новые идеи и продуманный подход.",
		BigFive: model.BigFive{Openness: &stubScores[0], Conscientiousness: &stubScores[1], Extraversion: &stubScores[2], Agreeableness: &stubScores[3], Neuroticism: &stubScores[4]},
	}
}

func (s *psychoTestServiceStub) GetCurrentTest(context.Context) (*model.Test, error) {
	return &model.Test{
		ID: 42, Methodology: model.MethodologyTIPI, Title: "TIPI",
		AnswerOptions: []model.AnswerOption{{Value: 1, Label: "Нет"}, {Value: 7, Label: "Да"}},
		Questions:     []model.Question{{ID: 1, OperationID: 1, Body: "Вопрос"}},
	}, nil
}
func (s *psychoTestServiceStub) SubmitTestAnswers(_ context.Context, userID int64, answers *model.TestAnswers) (*model.TestResult, error) {
	s.userID, s.testID = userID, answers.TestID
	return stubTestResult(answers.TestID), nil
}
func (s *psychoTestServiceStub) GetMyTestResult(_ context.Context, userID int64) (*model.TestResult, error) {
	if userID == 74 {
		return nil, model.ErrTestResultNotFound
	}
	return stubTestResult(42), nil
}

func TestProtectedTestRoutes(t *testing.T) {
	issuer := auth.NewJWTIssuer("test-secret", time.Minute)
	token, _, err := issuer.Issue(73, "session")
	if err != nil {
		t.Fatal(err)
	}
	noResultToken, _, err := issuer.Issue(74, "session")
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
		{"my result without cookie", "GET", "/api/v1/tests/results/me", "", "", "", 401},
		{"my result", "GET", "/api/v1/tests/results/me", token, "", "", 200},
		{"my result not passed", "GET", "/api/v1/tests/results/me", noResultToken, "", "", 404},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc := &psychoTestServiceStub{}
			r := mux.NewRouter()
			requireAuth := middleware.Auth(issuer)
			tests := handler.NewPsychoTestHandler(svc)
			r.Handle("/api/v1/tests/current", requireAuth(handler.WithUser(tests.Current))).Methods(http.MethodGet)
			r.Handle("/api/v1/tests/results/me", requireAuth(handler.WithUser(tests.MyResult))).Methods(http.MethodGet)
			r.Handle("/api/v1/tests/{test_id}/results", requireAuth(handler.WithUser(tests.Submit))).Methods(http.MethodPost)
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
			if tc.status == 201 || (tc.status == 200 && strings.HasSuffix(tc.path, "/results/me")) {
				var body struct {
					PersonalityType string             `json:"personality_type"`
					About           string             `json:"about_personality_type"`
					BigFive         map[string]float64 `json:"big_five"`
				}
				if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
					t.Fatal(err)
				}
				if body.PersonalityType != "STRATEGIST" || body.About == "" || len(body.BigFive) != 5 {
					t.Fatalf("incomplete result: %s", rec.Body.String())
				}
				for field, want := range map[string]float64{"openness": .85, "conscientiousness": .8, "extraversion": .25, "agreeableness": .55, "neuroticism": .2} {
					if got, ok := body.BigFive[field]; !ok || got != want {
						t.Fatalf("%s = %v, want %v", field, got, want)
					}
				}
			}
			if tc.status != 201 && svc.userID != 0 {
				t.Fatal("unexpected submission")
			}
		})
	}
}
