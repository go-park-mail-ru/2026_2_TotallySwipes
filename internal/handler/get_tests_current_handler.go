package handler

import (
	"dating-app/internal/handler/dto"
	"dating-app/internal/model"
	"dating-app/internal/service"
	"errors"
	"net/http"
	"strconv"
)

type GetCurrentTestHandler struct {
	testSvc service.TestService
}

func NewGetCurrentTestHandler(svc service.TestService) *GetCurrentTestHandler {
	return &GetCurrentTestHandler{testSvc: svc}
}

func (h *GetCurrentTestHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {

	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		WriteError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Метод не поддерживается")
		return
	}
	result, err := h.testSvc.GetCurrentTest(r.Context())

	if errors.Is(err, model.ErrActiveTestNotFound) {
		WriteError(w, http.StatusNotFound, "ACTIVE_TEST_NOT_FOUND", "Текущий тест не настроен")
		return
	}

	if err != nil || result == nil {
		WriteError(w, http.StatusInternalServerError,
			"INTERNAL_SERVER_ERROR", "Не удалось получить тест")
		return
	}

	response := dto.CurrentTestResponse{
		TestID: strconv.FormatInt(result.ID, 10), Title: result.Title, Instructions: result.Instructions,
		AnswerOptions: make([]dto.TestOption, 0, len(result.AnswerOptions)),
		Questions:     make([]dto.TestQuestion, 0, len(result.Questions)),
	}

	for _, option := range result.AnswerOptions {
		response.AnswerOptions = append(response.AnswerOptions, dto.TestOption{Value: option.Value, Label: option.Label})
	}

	for _, question := range result.Questions {
		response.Questions = append(response.Questions, dto.TestQuestion{ID: strconv.FormatInt(question.ID, 10), Body: question.Body})
	}

	Write(w, http.StatusOK, response)
}
