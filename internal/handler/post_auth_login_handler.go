package handler

import (
	"dating-app/internal/handler/dto"
	"dating-app/internal/model"
	"errors"
	"log/slog"
	"net/http"
)

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req dto.LoginRequest
	if !decodeJSON(w, r, &req) {
		return
	}

	req.Normalize()
	if errs := req.Validate(); len(errs) > 0 {
		writeError(w, http.StatusBadRequest, codeValidationError, "Проверьте поля запроса", errs)
		return
	}

	res, err := h.svc.Login(r.Context(), req.Email, req.Password)
	switch {
	case errors.Is(err, model.ErrInvalidCredentials):
		writeError(w, http.StatusUnauthorized, codeInvalidCredentials, "Неверная почта или пароль", nil)
		return
	case err != nil:
		slog.Error("login", "error", err)
		writeError(w, http.StatusInternalServerError, codeInternalError, msgInternalError, nil)
		return
	}

	h.setSessionCookies(w, res.Tokens)
	writeJSON(w, http.StatusOK, dto.AuthResponse{
		UserID:           res.UserID,
		ProfileCompleted: res.ProfileCompleted,
	})
}
