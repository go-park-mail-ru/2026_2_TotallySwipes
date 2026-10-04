package handler

import (
	"log/slog"
	"net/http"
)

// Logout отвечает 204, даже без действующей сессии. Cookies чистятся всегда:
// если отозвать сессию не удалось (500), клиент всё равно должен разлогиниться
func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	h.clearSessionCookies(w)

	// Ошибка тут только http.ErrNoCookie - тогда отзывать нечего
	if c, err := r.Cookie(refreshTokenCookie); err == nil {
		if err := h.svc.Logout(r.Context(), c.Value); err != nil {
			slog.Error("logout", "error", err)
			writeError(w, http.StatusInternalServerError, codeInternalError, msgInternalError, nil)
			return
		}
	}

	w.WriteHeader(http.StatusNoContent)
}
