package handler

import (
	"dating-app/internal/handler/dto"
	"encoding/json"
	"log/slog"
	"net/http"
)

const (
	codeValidationError      = "VALIDATION_ERROR"
	codeEmailAlreadyExists   = "EMAIL_ALREADY_EXISTS"
	codeInvalidCredentials   = "INVALID_CREDENTIALS"
	codeInternalError        = "INTERNAL_SERVER_ERROR"
	codeUnauthorized         = "UNAUTHORIZED"
	codePayloadTooLarge      = "PAYLOAD_TOO_LARGE"
	codeUnsupportedMediaType = "UNSUPPORTED_MEDIA_TYPE"

	msgInternalError = "Внутренняя ошибка сервера"
	msgUnauthorized  = "Необходимо войти заново"
	msgInvalidData   = "Некорректные данные"
)

func writeJSON(w http.ResponseWriter, status int, payload any) {
	body, err := json.Marshal(payload)
	if err != nil {
		slog.Error("marshal response", "error", err)
		status = http.StatusInternalServerError
		body, _ = json.Marshal(dto.ErrorResponse{Error: dto.ErrorBody{Code: codeInternalError, Message: msgInternalError}})
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if _, err := w.Write(body); err != nil {
		slog.Error("write response", "error", err)
	}
}

// writeError отдаёт ошибку; fields - ошибки по полям запроса или nil
func writeError(w http.ResponseWriter, status int, code, message string, fields map[string]string) {
	writeJSON(w, status, dto.ErrorResponse{Error: dto.ErrorBody{Code: code, Message: message, Fields: fields}})
}
