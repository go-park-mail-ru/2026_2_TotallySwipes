package handler

import (
	"dating-app/internal/handler/dto"
	"encoding/json"
	"log/slog"
	"net/http"
)

const (
	codeValidationError    = "VALIDATION_ERROR"
	codeEmailAlreadyExists = "EMAIL_ALREADY_EXISTS"
	codeInvalidCredentials = "INVALID_CREDENTIALS"
	codeInternalError      = "INTERNAL_SERVER_ERROR"
	codeUnauthorized       = "UNAUTHORIZED"
	codePayloadTooLarge    = "PAYLOAD_TOO_LARGE"

	msgInternalError = "Внутренняя ошибка сервера"
	msgUnauthorized  = "Необходимо войти заново"
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

// writeError отдаёт ошибку в формате: fields - ошибки по полям
// запроса, для остальных ошибок nil
func writeError(w http.ResponseWriter, status int, code, message string, fields map[string]string) {
	writeJSON(w, status, dto.ErrorResponse{Error: dto.ErrorBody{Code: code, Message: message, Fields: fields}})
}

// Write sends a JSON response using the shared response encoder.
func Write(w http.ResponseWriter, status int, data any) {
	writeJSON(w, status, data)
}

// WriteError sends an error without field-level validation details.
func WriteError(w http.ResponseWriter, status int, code, message string) {
	writeError(w, status, code, message, nil)
}
