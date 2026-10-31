package handler

import (
	"context"
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

// domainErrors переводит доменные ошибки в HTTP; выигрывает первая совпавшая по errors.Is
var domainErrors = []struct {
	err error
	api apiError
}{
	{model.ErrInvalidCity, apiError{http.StatusBadRequest, codeValidationError, "Некорректный город", nil}},
	{model.ErrInvalidEventResults, apiError{http.StatusBadRequest, codeValidationError, "Некорректный список понравившихся событий", nil}},
	{model.ErrNoEvents, apiError{http.StatusConflict, "NO_EVENTS", "Нет подходящих событий", nil}},
	{model.ErrSearchAlreadyActive, apiError{http.StatusConflict, "SEARCH_ALREADY_ACTIVE", "Для матча уже существует подбор в другом городе", nil}},
	{model.ErrResultAlreadySubmitted, apiError{http.StatusConflict, "RESULT_ALREADY_SUBMITTED", "Результат уже принят. Получите актуальное состояние подбора.", nil}},
	{model.ErrSearchStateConflict, apiError{http.StatusConflict, "SEARCH_STATE_CONFLICT", "Подбор уже завершён или отменён", nil}},
	{model.ErrTimepadTimeout, apiError{http.StatusRequestTimeout, "TIMEPAD_TIMEOUT", "Сервис подбора событий временно недоступен. Попробуйте позже.", nil}},
	{model.ErrInvalidCoordinates, apiError{http.StatusBadRequest, codeValidationError, "Некорректные координаты", nil}},
	{model.ErrLocationRequired, apiError{http.StatusConflict, "LOCATION_REQUIRED", "Сначала необходимо указать город", nil}},
	{model.ErrLocationPreviewUnavailable, apiError{http.StatusGone, "LOCATION_PREVIEW_UNAVAILABLE", "Повторите определение местоположения", nil}},
	{model.ErrLocationNotFound, apiError{http.StatusUnprocessableEntity, "LOCATION_NOT_FOUND", "Не удалось определить местоположение", nil}},
	{model.ErrCityNotFound, apiError{http.StatusUnprocessableEntity, "LOCATION_NOT_FOUND", "Не удалось определить город", nil}},
	{model.ErrGeocoderQuotaExceeded, apiError{http.StatusTooManyRequests, "GEOCODER_QUOTA_EXCEEDED", "Quota exceeded", nil}},
	{model.ErrInvalidCredentials, apiError{http.StatusUnauthorized, codeInvalidCredentials, "Неверная почта или пароль", nil}},
	{model.ErrInvalidSession, apiError{http.StatusUnauthorized, codeUnauthorized, msgUnauthorized, nil}},
	{model.ErrEmailAlreadyExists, apiError{http.StatusConflict, codeEmailAlreadyExists, "Почта уже занята", nil}},
	{model.ErrPasswordTooLong, apiError{http.StatusBadRequest, codeValidationError, msgInvalidData,
		map[string]string{"password": validate.ErrPasswordTooLong.Error()}}},
	{model.ErrUnknownTag, apiError{http.StatusBadRequest, codeValidationError, msgInvalidData,
		map[string]string{"tags": validate.ErrTagUnknown.Error()}}},
	{model.ErrInvalidFeedRequest, apiError{http.StatusBadRequest, codeValidationError, "Некорректные параметры ленты", nil}},
	{model.ErrInvalidMatchesRequest, apiError{http.StatusBadRequest, codeValidationError, msgInvalidMatchesQuery, nil}},
	{model.ErrInvalidTestRequest, apiError{http.StatusBadRequest, codeValidationError, "Некорректный запрос", nil}},
	{model.ErrInvalidAnswers, apiError{http.StatusUnprocessableEntity, "INVALID_ANSWERS", "Проверьте полноту и допустимость ответов", nil}},
	{model.ErrProfileRequired, apiError{http.StatusConflict, "PROFILE_REQUIRED", "Сначала необходимо заполнить анкету", nil}},
	{model.ErrProfileIncomplete, apiError{http.StatusForbidden, "PROFILE_REQUIRED", "Сначала необходимо заполнить анкету", nil}},
	{model.ErrPhotoLimit, apiError{http.StatusConflict, "PHOTO_LIMIT", "Можно загрузить не больше 6 фотографий", nil}},
	{model.ErrLastPhoto, apiError{http.StatusConflict, "LAST_PHOTO", "Нельзя удалить единственное фото заполненной анкеты", nil}},
	{model.ErrMatchNotFound, apiError{http.StatusNotFound, "MATCH_NOT_FOUND", "Мэтч не найден", nil}},
	{model.ErrPhotoNotFound, apiError{http.StatusNotFound, "PHOTO_NOT_FOUND", "Фото не найдено", nil}},
	{model.ErrPhotoOrderMismatch, apiError{http.StatusBadRequest, codeValidationError, msgInvalidData,
		map[string]string{"photo_ids": validate.ErrPhotoOrder.Error()}}},
	{model.ErrTestNotFound, apiError{http.StatusNotFound, "TEST_NOT_FOUND", "Тест не найден", nil}},
	{model.ErrTestResultNotFound, apiError{http.StatusNotFound, "TEST_RESULT_NOT_FOUND", "Тест ещё не пройден", nil}},
	{model.ErrInvalidSwipe, apiError{http.StatusBadRequest, codeValidationError, msgInvalidData,
		map[string]string{"target_user_id": "Нельзя свайпать собственную анкету"}}},
	{model.ErrProfileNotFound, apiError{http.StatusNotFound, "PROFILE_NOT_FOUND", "Целевая анкета не найдена или недоступна", nil}},
	{model.ErrNotFound, apiError{http.StatusNotFound, "NOT_FOUND", "Ресурс не найден или недоступен", nil}},
}

// writeServiceError отвечает по domainErrors, остальные ошибки логирует и отдаёт 500
func writeServiceError(ctx context.Context, w http.ResponseWriter, op string, err error) {
	for _, e := range domainErrors {
		if errors.Is(err, e.err) {
			writeError(ctx, w, e.api.status, e.api.code, e.api.message, e.api.fields)
			return
		}
	}
	slog.ErrorContext(ctx, op, "error", err)
	writeError(ctx, w, http.StatusInternalServerError, codeInternalError, msgInternalError, nil)
}
