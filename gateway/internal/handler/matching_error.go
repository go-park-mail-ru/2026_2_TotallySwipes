package handler

import (
	"net/http"

	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// matchingAPIError сохраняет HTTP-коды монолита после перехода Matching на gRPC.
func matchingAPIError(err error) (apiError, bool) {
	for _, detail := range status.Convert(err).Details() {
		info, ok := detail.(*errdetails.ErrorInfo)
		if !ok || info.GetDomain() != "matching" {
			continue
		}
		result := apiError{code: info.GetReason()}
		switch info.GetReason() {
		case "VALIDATION_ERROR":
			result.status, result.message = http.StatusBadRequest, msgInvalidData
		case "PROFILE_REQUIRED":
			result.status, result.message = http.StatusConflict, "Сначала необходимо заполнить анкету"
			if status.Code(err) == codes.PermissionDenied {
				result.status = http.StatusForbidden
			}
		case "LOCATION_REQUIRED":
			result.status, result.message = http.StatusConflict, "Сначала необходимо указать город"
		case "MATCH_NOT_FOUND":
			result.status, result.message = http.StatusNotFound, "Мэтч не найден"
		case "PROFILE_NOT_FOUND":
			result.status, result.message = http.StatusNotFound, "Целевая анкета не найдена или недоступна"
		case "NOT_FOUND":
			result.status, result.message = http.StatusNotFound, "Ресурс не найден или недоступен"
		case "NO_EVENTS":
			result.status, result.message = http.StatusConflict, "Нет подходящих событий"
		case "RESULT_ALREADY_SUBMITTED":
			result.status, result.message = http.StatusConflict, "Результат уже принят. Получите актуальное состояние подбора."
		case "SEARCH_STATE_CONFLICT":
			result.status, result.message = http.StatusConflict, "Подбор уже завершён или отменён"
		case "TIMEPAD_TIMEOUT":
			result.status, result.message = http.StatusRequestTimeout, "Сервис подбора событий временно недоступен. Попробуйте позже."
		default:
			continue
		}
		return result, true
	}
	return apiError{}, false
}
