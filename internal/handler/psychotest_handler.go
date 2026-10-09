package handler

import (
	"context"
	"dating-app/internal/handler/dto"
	"dating-app/internal/model"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
)

type PsychoTestService interface {
	GetCurrentTest(ctx context.Context) (*model.Test, error)
	SubmitTestAnswers(ctx context.Context, userID int64, answers *model.TestAnswers) (*model.TestResult, error)
	GetMyTestResult(ctx context.Context, userID int64) (*model.TestResult, error)
}

type PsychoTestHandler struct {
	svc PsychoTestService
}

func NewPsychoTestHandler(svc PsychoTestService) *PsychoTestHandler {
	return &PsychoTestHandler{svc: svc}
}

// Current - GET /tests/current: вопросы и шкала ответов
func (h *PsychoTestHandler) Current(w http.ResponseWriter, r *http.Request, _ int64) {
	test, err := h.svc.GetCurrentTest(r.Context())
	if err != nil {
		writeServiceError(w, "get current test", err)
		return
	}
	writeJSON(w, http.StatusOK, dto.NewCurrentTestResponse(test))
}

// Submit - POST /tests/{test_id}/results: сохраняет ответы и отдаёт результат попытки
func (h *PsychoTestHandler) Submit(w http.ResponseWriter, r *http.Request, userID int64) {
	testID, err := strconv.ParseInt(mux.Vars(r)["test_id"], 10, 64)
	if err != nil || testID <= 0 {
		writeError(w, http.StatusBadRequest, codeValidationError, "Некорректный test_id", nil)
		return
	}

	var req dto.SubmitTestAnswersRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	answers, err := req.ToModel(testID)
	if err != nil {
		writeError(w, http.StatusBadRequest, codeValidationError, err.Error(), nil)
		return
	}

	result, err := h.svc.SubmitTestAnswers(r.Context(), userID, answers)
	if err != nil {
		writeServiceError(w, "submit test answers", err)
		return
	}
	writeJSON(w, http.StatusCreated, dto.NewTestResultResponse(result))
}

// MyResult - GET /tests/results/me: последний сохранённый результат
func (h *PsychoTestHandler) MyResult(w http.ResponseWriter, r *http.Request, userID int64) {
	result, err := h.svc.GetMyTestResult(r.Context(), userID)
	if err != nil {
		writeServiceError(w, "get my test result", err)
		return
	}
	writeJSON(w, http.StatusOK, dto.NewTestResultResponse(result))
}
