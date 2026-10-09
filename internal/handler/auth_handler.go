package handler

import (
	"context"
	"dating-app/internal/handler/dto"
	"dating-app/internal/model"
	"errors"
	"log/slog"
	"net/http"
	"time"
)

const (
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

// Register - POST /auth/register: аккаунт по почте и паролю с пустой анкетой и сессией
func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	var req dto.RegisterRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	req.Normalize()
	if errs := req.Validate(); len(errs) > 0 {
		writeError(w, http.StatusBadRequest, codeValidationError, msgInvalidData, errs)
		return
	}

	res, err := h.svc.Register(r.Context(), req.ToModel())
	switch {
	case errors.Is(err, model.ErrSessionNotOpened):
		slog.Error("register user: session not opened", "user_id", res.UserID, "error", err)
	case err != nil:
		writeServiceError(w, "register user", err)
		return
	default:
		h.setSessionCookies(w, res.Tokens)
	}

	writeJSON(w, http.StatusCreated, dto.NewAuthResponse(res))
}

// Login - POST /auth/login: открывает сессию и сообщает, чего не хватает анкете
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req dto.LoginRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	req.Normalize()
	if errs := req.Validate(); len(errs) > 0 {
		writeError(w, http.StatusBadRequest, codeValidationError, msgInvalidData, errs)
		return
	}

	res, err := h.svc.Login(r.Context(), req.Email, req.Password)
	if err != nil {
		writeServiceError(w, "login", err)
		return
	}

	h.setSessionCookies(w, res.Tokens)
	writeJSON(w, http.StatusOK, dto.NewAuthResponse(res))
}

// Refresh отвечает 204 с новыми cookies, при недействительной сессии - 401
func (h *AuthHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	c, err := r.Cookie(refreshTokenCookie)
	if err != nil {
		h.clearSessionCookies(w)
		writeError(w, http.StatusUnauthorized, codeUnauthorized, msgUnauthorized, nil)
		return
	}

	tokens, err := h.svc.Refresh(r.Context(), c.Value)
	if errors.Is(err, model.ErrInvalidSession) {
		h.clearSessionCookies(w)
	}
	if err != nil {
		writeServiceError(w, "refresh", err)
		return
	}

	h.setSessionCookies(w, tokens)
	w.WriteHeader(http.StatusNoContent)
}

// Logout всегда чистит cookies и отвечает 204, даже без действующей сессии
func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	h.clearSessionCookies(w)

	if c, err := r.Cookie(refreshTokenCookie); err == nil {
		if err := h.svc.Logout(r.Context(), c.Value); err != nil {
			writeServiceError(w, "logout", err)
			return
		}
	}

	w.WriteHeader(http.StatusNoContent)
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
