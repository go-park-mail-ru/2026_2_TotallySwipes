package handler

import (
	"dating-app/internal/middleware"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
)

const maxRequestBodySize = 1 << 20

// decodeJSON читает тело как ровно один JSON-объект без неизвестных полей.
// При ошибке сам отвечает клиенту и возвращает false
func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		writeError(w, http.StatusUnsupportedMediaType, codeUnsupportedMediaType, "Требуется Content-Type application/json", nil)
		return false
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodySize)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()

	err = dec.Decode(dst)
	if err == nil && dec.Decode(&struct{}{}) != io.EOF {
		err = errors.New("more than one JSON value")
	}
	if _, ok := errors.AsType[*http.MaxBytesError](err); ok {
		writeError(w, http.StatusRequestEntityTooLarge, codePayloadTooLarge, "Слишком большой запрос", nil)
		return false
	}
	if err != nil {
		writeError(w, http.StatusBadRequest, codeValidationError, "Некорректный JSON", nil)
		return false
	}
	return true
}

// UserHandlerFunc - хендлер защищённой ручки, которому уже передан ID пользователя
type UserHandlerFunc func(w http.ResponseWriter, r *http.Request, userID int64)

// WithUser достаёт ID пользователя, положенный middleware.Auth, и передаёт его в fn
func WithUser(fn UserHandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := middleware.UserIDFromContext(r.Context())
		if !ok {
			writeError(w, http.StatusUnauthorized, codeUnauthorized, "Необходима авторизация", nil)
			return
		}
		fn(w, r, userID)
	}
}
