package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"
)

func Write(w http.ResponseWriter, status int, data any) {
	body, err := json.Marshal(data)
	if err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if _, err := w.Write(body); err != nil {
		slog.Error("write response", "error", err)
	}
}

func WriteError(w http.ResponseWriter, status int, code, message string) {
	Write(w, status, map[string]any{
		"error": map[string]string{
			"code":    code,
			"message": message,
		},
	})
}
