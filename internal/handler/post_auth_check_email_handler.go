package handler

import (
	"dating-app/internal/handler/dto"
	"log/slog"
	"net/http"
)

func (h *AuthHandler) CheckEmail(w http.ResponseWriter, r *http.Request) {
	var req dto.CheckEmailRequest
	if !decodeJSON(w, r, &req) {
		return
	}

	req.Normalize()
	if errs := req.Validate(); len(errs) > 0 {
		writeError(w, http.StatusBadRequest, codeValidationError, "Проверьте поля запроса", errs)
		return
	}

	available, err := h.svc.IsEmailAvailable(r.Context(), req.Email)
	if err != nil {
		slog.Error("check email", "error", err)
		writeError(w, http.StatusInternalServerError, codeInternalError, msgInternalError, nil)
		return
	}

	writeJSON(w, http.StatusOK, dto.CheckEmailResponse{Available: available})
}
