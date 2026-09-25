package handler

import (
	"dating-app/internal/service"
	"dating-app/internal/validate"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
)

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type LoginResponse struct {
	ID               int64 `json:"user_id"`
	ProfileCompleted bool  `json:"profile_completed"`
}

type LoginHandler struct {
	authSvc service.AuthService
}

func NewLoginHandler(svc service.AuthService) *LoginHandler {
	return &LoginHandler{authSvc: svc}
}

func (h *LoginHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {

	var input LoginRequest

	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		WriteError(w, http.StatusBadRequest, "INVALID_REQUEST", "Некорректный JSON")
		return
	}

	input.Email = strings.TrimSpace(input.Email)

	if err := validate.ValidateEmail(input.Email); err != nil {
		WriteError(w, http.StatusBadRequest, "INVALID_CREDENTIALS", "Неверная почта или пароль")
		return
	}

	if input.Password == "" {
		WriteError(w, http.StatusBadRequest,
			"INVALID_REQUEST", "Пароль обязателен")
		return
	}

	if err := validate.ValidatePassword(input.Password); err != nil {
		WriteError(w, http.StatusBadRequest, "INVALID_CREDENTIALS", "Неверная почта или пароль")
		return
	}

	user, err := h.authSvc.LoginWithEmail(r.Context(), input.Email, input.Password)

	if errors.Is(err, service.ErrInvalidCredentials) {
		WriteError(w, http.StatusUnauthorized,
			"INVALID_CREDENTIALS", "Неверная почта или пароль")
		return
	}

	if err != nil {
		WriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Внутренняя ошибка сервера")
		return
	}

	err = h.authSvc.CheckProfileByUserID(r.Context(), user.ID)

	if err != nil {
		Write(w, http.StatusOK, LoginResponse{user.ID, false})
		return
	}

	Write(w, http.StatusOK, LoginResponse{user.ID, true})
}
