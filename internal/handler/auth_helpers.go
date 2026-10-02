package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"dating-app/internal/model"
)

const (
	maxRequestBodySize = 1 << 20

	accessTokenCookie      = "access_token"
	refreshTokenCookie     = "refresh_token"
	refreshTokenCookiePath = "/api/v1/auth"
)

type AuthService interface {
	Register(ctx context.Context, in model.RegisterInput) (model.AuthResult, error)
	Login(ctx context.Context, email, password string) (model.AuthResult, error)
	Logout(ctx context.Context, refreshToken string) error
	Refresh(ctx context.Context, refreshToken string) (model.Tokens, error)
}

type AuthHandler struct {
	svc          AuthService
	cookieSecure bool
}

func NewAuthHandler(svc AuthService, cookieSecure bool) *AuthHandler {
	return &AuthHandler{svc: svc, cookieSecure: cookieSecure}
}

func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodySize)
	if err := json.NewDecoder(r.Body).Decode(dst); err != nil {
		writeError(w, http.StatusBadRequest, codeValidationError, "Некорректный JSON", nil)
		return false
	}
	return true
}

func (h *AuthHandler) setSessionCookies(w http.ResponseWriter, t model.Tokens) {
	http.SetCookie(w, h.sessionCookie(accessTokenCookie, "/", t.Access, t.AccessExpiresAt))
	http.SetCookie(w, h.sessionCookie(refreshTokenCookie, refreshTokenCookiePath, t.Refresh, t.RefreshExpiresAt))
}

func (h *AuthHandler) clearSessionCookies(w http.ResponseWriter) {
	for _, c := range []*http.Cookie{
		h.sessionCookie(accessTokenCookie, "/", "", time.Time{}),
		h.sessionCookie(refreshTokenCookie, refreshTokenCookiePath, "", time.Time{}),
	} {
		c.MaxAge = -1
		http.SetCookie(w, c)
	}
}

func (h *AuthHandler) sessionCookie(name, path, value string, expires time.Time) *http.Cookie {
	return &http.Cookie{
		Name:     name,
		Value:    value,
		Path:     path,
		Expires:  expires,
		HttpOnly: true,
		Secure:   h.cookieSecure,
		SameSite: http.SameSiteLaxMode,
	}
}
