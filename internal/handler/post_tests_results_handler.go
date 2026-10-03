package handler

import (
	"dating-app/internal/handler/dto"
	"dating-app/internal/middleware"
	"dating-app/internal/model"
	"dating-app/internal/service"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
)

type PostTestResultsHandler struct {
	testSvc service.TestService
}

func NewPostTestResultsHandler(svc service.TestService) *PostTestResultsHandler {
	return &PostTestResultsHandler{testSvc: svc}
}

func (h *PostTestResultsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {

	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		WriteError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Метод не поддерживается")
		return
	}

	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		WriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Необходима авторизация")
		return
	}
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		WriteError(w, http.StatusUnsupportedMediaType, "UNSUPPORTED_MEDIA_TYPE", "Требуется Content-Type application/json")
		return
	}
	testIDRaw := mux.Vars(r)["test_id"]

	testID, err := strconv.ParseInt(testIDRaw, 10, 64)

	if err != nil || testID <= 0 {
		WriteError(w, http.StatusBadRequest,
			"INVALID_REQUEST", "Некорректный test_id")
		return
	}

	var input dto.SubmitTestAnswersRequest
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
		case errors.Is(err, model.ErrInvalidTestRequest):
			WriteError(w, http.StatusBadRequest, "INVALID_REQUEST", "Некорректный запрос")
		case errors.Is(err, model.ErrTestNotFound):
			WriteError(w, http.StatusNotFound, "TEST_NOT_FOUND", "Тест не найден")
		case errors.Is(err, model.ErrProfileRequired):
			WriteError(w, http.StatusConflict, "PROFILE_REQUIRED", "Для прохождения теста необходимо создать профиль")
		case errors.Is(err, model.ErrInvalidAnswers):
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

	Write(w, http.StatusCreated, newTestResultResponse(result))
}

func newTestResultResponse(result *model.TestResult) dto.SubmitTestAnswersResponse {
	return dto.SubmitTestAnswersResponse{
		ResultID:             strconv.FormatInt(result.ID, 10),
		TestID:               strconv.FormatInt(result.TestID, 10),
		Revision:             result.Revision,
		CompletedAt:          result.CompletedA,
		PersonalityType:      result.PersonalityType,
		AboutPersonalityType: result.AboutPersonalityType,
		BigFive: dto.TestBigFive{
			Openness: result.BigFive.Openness, Conscientiousness: result.BigFive.Conscientiousness,
			Extraversion: result.BigFive.Extraversion, Agreeableness: result.BigFive.Agreeableness,
			Neuroticism: result.BigFive.Neuroticism,
		},
	}
}
