package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"
)

const (
	codeValidationError    = "VALIDATION_ERROR"
	codeEmailAlreadyExists = "EMAIL_ALREADY_EXISTS"
	codeInvalidCredentials = "INVALID_CREDENTIALS"
	codeInternalError      = "INTERNAL_SERVER_ERROR"

	msgInternalError = "Внутренняя ошибка сервера"
)

type errorResponse struct {
	Error errorBody `json:"error"`
}

type errorBody struct {
	Code    string            `json:"code"`
	Message string            `json:"message"`
	Fields  map[string]string `json:"fields,omitempty"`
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	body, err := json.Marshal(payload)
	if err != nil {
		slog.Error("marshal response", "error", err)
		status = http.StatusInternalServerError
		body, _ = json.Marshal(errorResponse{Error: errorBody{Code: codeInternalError, Message: msgInternalError}})
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
	writeJSON(w, status, errorResponse{Error: errorBody{Code: code, Message: message, Fields: fields}})
}
