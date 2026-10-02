package handler

import (
	"dating-app/internal/model"
	"errors"
	"log/slog"
	"net/http"
)

// Refresh отвечает 204 с новыми cookies, при недействительной сессии - 401
func (h *AuthHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	c, err := r.Cookie(refreshTokenCookie)
	if err != nil {
		h.clearSessionCookies(w)
		writeError(w, http.StatusUnauthorized, codeUnauthorized, msgUnauthorized, nil)
		return
	}

	tokens, err := h.svc.Refresh(r.Context(), c.Value)
	switch {
	case errors.Is(err, model.ErrInvalidSession):
		h.clearSessionCookies(w)
		writeError(w, http.StatusUnauthorized, codeUnauthorized, msgUnauthorized, nil)
		return
	case err != nil:
		slog.Error("refresh", "error", err)
		writeError(w, http.StatusInternalServerError, codeInternalError, msgInternalError, nil)
		return
	}

	h.setSessionCookies(w, tokens)
	w.WriteHeader(http.StatusNoContent)
}
