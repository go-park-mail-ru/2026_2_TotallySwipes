package handler

import (
	"dating-app/internal/handler/dto"
	"net/http"
)

// GetHealth - GET /health: сообщает, что HTTP-сервер отвечает
func GetHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(r.Context(), w, http.StatusOK, dto.HealthResponse{Status: "ok"})
}
