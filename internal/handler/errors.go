package handler

import (
	"dating-app/internal/model"
	"dating-app/internal/validate"
	"errors"
	"log/slog"
	"net/http"
)

type apiError struct {
	status  int
	code    string
	message string
	fields  map[string]string
}

// domainErrors - единая таблица перевода доменных ошибок в HTTP-ответы.
// Проверяется по порядку через errors.Is, первая подходящая запись выигрывает
var domainErrors = []struct {
	err error
	api apiError
}{
	{model.ErrInvalidCredentials, apiError{http.StatusUnauthorized, codeInvalidCredentials, "Неверная почта или пароль", nil}},
	{model.ErrInvalidSession, apiError{http.StatusUnauthorized, codeUnauthorized, msgUnauthorized, nil}},
	{model.ErrEmailAlreadyExists, apiError{http.StatusConflict, codeEmailAlreadyExists, "Почта уже занята", nil}},
	{model.ErrPasswordTooLong, apiError{http.StatusBadRequest, codeValidationError, msgInvalidData,
		map[string]string{"password": validate.ErrPasswordTooLong.Error()}}},
	{model.ErrUnknownTag, apiError{http.StatusBadRequest, codeValidationError, msgInvalidData,
		map[string]string{"tags": validate.ErrTagUnknown.Error()}}},
	{model.ErrInvalidFeedRequest, apiError{http.StatusBadRequest, codeValidationError, "Некорректные параметры ленты", nil}},
	{model.ErrInvalidTestRequest, apiError{http.StatusBadRequest, codeValidationError, "Некорректный запрос", nil}},
	{model.ErrInvalidAnswers, apiError{http.StatusUnprocessableEntity, "INVALID_ANSWERS", "Проверьте полноту и допустимость ответов", nil}},
	{model.ErrProfileRequired, apiError{http.StatusConflict, "PROFILE_REQUIRED", "Сначала необходимо заполнить анкету", nil}},
	{model.ErrPhotoLimit, apiError{http.StatusConflict, "PHOTO_LIMIT", "Можно загрузить не больше 6 фотографий", nil}},
	{model.ErrLastPhoto, apiError{http.StatusConflict, "LAST_PHOTO", "Нельзя удалить единственное фото заполненной анкеты", nil}},
	{model.ErrPhotoNotFound, apiError{http.StatusNotFound, "PHOTO_NOT_FOUND", "Фото не найдено", nil}},
	{model.ErrTestNotFound, apiError{http.StatusNotFound, "TEST_NOT_FOUND", "Тест не найден", nil}},
	{model.ErrTestResultNotFound, apiError{http.StatusNotFound, "TEST_RESULT_NOT_FOUND", "Тест ещё не пройден", nil}},
}

// writeServiceError отдаёт ответ по ошибке сервиса: известные доменные ошибки
// берутся из domainErrors, остальные логируются с op и превращаются в 500
func writeServiceError(w http.ResponseWriter, op string, err error) {
	for _, e := range domainErrors {
		if errors.Is(err, e.err) {
			writeError(w, e.api.status, e.api.code, e.api.message, e.api.fields)
			return
		}
	}
	slog.Error(op, "error", err)
	writeError(w, http.StatusInternalServerError, codeInternalError, msgInternalError, nil)
}
