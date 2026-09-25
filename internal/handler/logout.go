package handler

import (
	"dating-app/internal/service"
	"errors"
	"net/http"
)

type LogoutHandler struct {
	authSvc service.AuthService
}

func NewLogoutHandler(svc service.AuthService) *LogoutHandler {
	return &LogoutHandler{authSvc: svc}
}

func (h *LogoutHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("refresh_token")

	if err == nil {
		if err := h.authSvc.Logout(r.Context(), cookie.Value); err != nil {
			WriteError(w, http.StatusInternalServerError,
				"INTERNAL_ERROR", "Не удалось завершить сессию")
			return
		}
	} else if !errors.Is(err, http.ErrNoCookie) {
		WriteError(w, http.StatusBadRequest,
			"INVALID_REQUEST", "Некорректная cookie")
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "refresh_token",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
	})

	w.WriteHeader(http.StatusNoContent)
}
