package handler

import (
	"dating-app/internal/model"
	"dating-app/internal/service"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/gorilla/mux"
)

type SubmitTestAnswersRequest struct {
	Answers []TestAnswerRequest `json:"answers"`
}

type TestAnswerRequest struct {
	QuestionID string `json:"question_id"`
	Value      *int   `json:"value"`
}

type TestAnswersHandler struct {
	testSvc service.TestService
}

func NewTestAnswersHandler(svc service.TestService) *TestAnswersHandler {
	return &TestAnswersHandler{testSvc: svc}
}

type SubmitTestAnswersResponse struct {
	ResultID    string    `json:"result_id"`
	TestID      string    `json:"test_id"`
	Revision    int       `json:"revision"`
	CompletedAt time.Time `json:"completed_at"`
}

func (h *TestAnswersHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {

	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		WriteError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Метод не поддерживается")
		return
	}

	userID := int64(1)
	testIDRaw := mux.Vars(r)["test_id"]

	testID, err := strconv.ParseInt(testIDRaw, 10, 64)

	if err != nil || testID <= 0 {
		WriteError(w, http.StatusBadRequest,
			"INVALID_REQUEST", "Некорректный test_id")
		return
	}

	var input SubmitTestAnswersRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(&input); err != nil {
		WriteError(w, http.StatusBadRequest,
			"INVALID_REQUEST", "Некорректный JSON")
		return
	}

	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		WriteError(w, http.StatusBadRequest,
			"INVALID_REQUEST", "Ожидается один JSON-объект")
		return
	}

	if input.Answers == nil {
		WriteError(w, http.StatusBadRequest, "INVALID_REQUEST", "Поле answers обязательно и не может быть null")
		return
	}

	answers := &model.TestAnswers{
		TestID:  testID,
		Answers: make([]model.Answer, 0, len(input.Answers)),
	}

	for _, answer := range input.Answers {
		questionID, err := strconv.ParseInt(answer.QuestionID, 10, 64)

		if err != nil || questionID <= 0 || answer.Value == nil {
			WriteError(w, http.StatusBadRequest,
				"INVALID_REQUEST", "Некорректный ID вопроса или отсутствует ответ")
			return
		}

		answers.Answers = append(answers.Answers, model.Answer{
			QuestionID: questionID,
			Value:      *answer.Value,
		})
	}

	result, err := h.testSvc.SubmitTestAnswers(r.Context(), userID, answers)

	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidTestRequest):
			WriteError(w, http.StatusBadRequest, "INVALID_REQUEST", "Некорректный запрос")
		case errors.Is(err, service.ErrTestNotFound):
			WriteError(w, http.StatusNotFound, "TEST_NOT_FOUND", "Тест не найден")
		case errors.Is(err, service.ErrProfileRequired):
			WriteError(w, http.StatusConflict, "PROFILE_REQUIRED", "Для прохождения теста необходимо создать профиль")
		case errors.Is(err, service.ErrInvalidAnswers):
			WriteError(w, http.StatusUnprocessableEntity, "INVALID_ANSWERS", "Проверьте полноту и допустимость ответов")
		default:
			WriteError(w, http.StatusInternalServerError, "INTERNAL_SERVER_ERROR", "Не удалось сохранить результаты теста")
		}
		return
	}

	if result == nil {
		WriteError(w, http.StatusInternalServerError, "INTERNAL_SERVER_ERROR", "Не удалось сохранить результаты теста")
		return
	}

	response := SubmitTestAnswersResponse{
		ResultID:    strconv.FormatInt(result.ID, 10),
		TestID:      strconv.FormatInt(result.TestID, 10),
		Revision:    result.Revision,
		CompletedAt: result.CompletedA,
	}

	Write(w, http.StatusCreated, response)
}
