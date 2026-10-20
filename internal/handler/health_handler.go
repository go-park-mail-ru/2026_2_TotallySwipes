package handler

import (
	"dating-app/internal/handler/dto"
	"net/http"
)

func GetHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(r.Context(), w, http.StatusOK, dto.HealthResponse{Status: "ok"})
}
