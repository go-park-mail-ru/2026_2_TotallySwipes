package handler

import (
	"dating-app/gateway/internal/handler/dto"
	"net/http"
)

// GetHealth - GET /health: сообщает, что HTTP-сервер отвечает
func GetHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(r.Context(), w, http.StatusOK, dto.HealthResponse{Status: "ok"})
}
