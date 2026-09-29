package handler

import (
	"dating-app/internal/handler/dto"
	"net/http"
)

func GetHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, dto.HealthResponse{Status: "ok"})
}
