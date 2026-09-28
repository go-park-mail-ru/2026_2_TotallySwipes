package handler

import (
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

type currentTestQuestion struct {
	ID   string `json:"id"`
	Body string `json:"body"`
}
type currentTestOption struct {
	Value int    `json:"value"`
	Label string `json:"label"`
}
type currentTestResponse struct {
	TestID        string                `json:"test_id"`
	Title         string                `json:"title"`
	Instructions  string                `json:"instructions"`
	AnswerOptions []currentTestOption   `json:"answer_options"`
	Questions     []currentTestQuestion `json:"questions"`
}

func (h *GetCurrentTestHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {

	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		WriteError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Метод не поддерживается")
		return
	}
	result, err := h.testSvc.GetCurrentTest(r.Context())

	if errors.Is(err, service.ErrActiveTestNotFound) {
		WriteError(w, http.StatusNotFound, "ACTIVE_TEST_NOT_FOUND", "Текущий тест не настроен")
		return
	}

	if err != nil || result == nil {
		WriteError(w, http.StatusInternalServerError,
			"INTERNAL_SERVER_ERROR", "Не удалось получить тест")
		return
	}

	response := currentTestResponse{
		TestID: strconv.FormatInt(result.ID, 10), Title: result.Title, Instructions: result.Instructions,
		AnswerOptions: make([]currentTestOption, 0, len(result.AnswerOptions)),
		Questions:     make([]currentTestQuestion, 0, len(result.Questions)),
	}

	for _, option := range result.AnswerOptions {
		response.AnswerOptions = append(response.AnswerOptions, currentTestOption{Value: option.Value, Label: option.Label})
	}

	for _, question := range result.Questions {
		response.Questions = append(response.Questions, currentTestQuestion{ID: strconv.FormatInt(question.ID, 10), Body: question.Body})
	}

	Write(w, http.StatusOK, response)
}
