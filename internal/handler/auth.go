package handler

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"dating-app/internal/model"
	"dating-app/internal/service"
	"dating-app/internal/validate"
)

const (
	maxRequestBodySize = 1 << 20

	accessTokenCookie      = "access_token"
	refreshTokenCookie     = "refresh_token"
	refreshTokenCookiePath = "/api/v1/auth"
)

type AuthService interface {
	Register(ctx context.Context, in service.RegisterInput) (service.AuthResult, error)
	Login(ctx context.Context, email, password string) (service.AuthResult, error)
	Logout(ctx context.Context, refreshToken string) error
}

type AuthResponse struct {
	UserID           int64 `json:"user_id"`
	ProfileCompleted bool  `json:"profile_completed"`
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

func (h *AuthHandler) setSessionCookies(w http.ResponseWriter, t service.Tokens) {
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

type RegisterRequest struct {
	Name          string   `json:"name"`
	Email         string   `json:"email"`
	Password      string   `json:"password"`
	BirthDate     string   `json:"birth_date"`
	Sex           string   `json:"sex"`
	SearchSex     string   `json:"search_sex"`
	DatingIntent  string   `json:"dating_intent"`
	AboutMe       *string  `json:"about_me"`
	SearchAgeFrom int      `json:"search_age_from"`
	SearchAgeTo   int      `json:"search_age_to"`
	Tags          []string `json:"tags"`
}

// Normalize приводит поля к виду, в котором их проверяют и сохраняют:
// email обрезается по краям и приводится к нижнему регистру, пароль не трогается
func (r *RegisterRequest) Normalize() {
	r.Email = normalizeEmail(r.Email)
}

// Validate собирает ошибки по всем полям сразу, чтобы клиент получил их
// одним ответом. Пустая map - запрос корректен. Вызывать после Normalize
func (r RegisterRequest) Validate() map[string]string {
	errs := make(map[string]string)

	addErr(errs, "name", validate.ValidateName(r.Name))
	addErr(errs, "email", validate.ValidateEmail(r.Email))
	addErr(errs, "password", validate.ValidatePassword(r.Password))

	if birthDate, err := validate.ParseBirthDate(r.BirthDate); err != nil {
		addErr(errs, "birth_date", err)
	} else {
		addErr(errs, "birth_date", validate.ValidateAge(birthDate))
	}

	addErr(errs, "sex", validate.ValidateSex(r.Sex))
	addErr(errs, "search_sex", validate.ValidateSearchSex(r.SearchSex))
	addErr(errs, "dating_intent", validate.ValidateDatingIntent(r.DatingIntent))

	fromErr := validate.ValidateSearchAge(r.SearchAgeFrom)
	addErr(errs, "search_age_from", fromErr)
	if err := validate.ValidateSearchAge(r.SearchAgeTo); err != nil {
		addErr(errs, "search_age_to", err)
	} else if fromErr == nil {
		addErr(errs, "search_age_to", validate.ValidateSearchAgeRange(r.SearchAgeFrom, r.SearchAgeTo))
	}

	addErr(errs, "tags", validate.ValidateTags(r.Tags))

	return errs
}

func addErr(errs map[string]string, field string, err error) {
	if err != nil {
		errs[field] = err.Error()
	}
}

func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	var req RegisterRequest
	if !decodeJSON(w, r, &req) {
		return
	}

	req.Normalize()
	if errs := req.Validate(); len(errs) > 0 {
		writeError(w, http.StatusBadRequest, codeValidationError, "Некорректные данные", errs)
		return
	}

	// Ошибки тут быть не может - дату уже проверил Validate
	birthDate, _ := validate.ParseBirthDate(req.BirthDate)

	var aboutMe string
	if req.AboutMe != nil {
		aboutMe = *req.AboutMe
	}

	res, err := h.svc.Register(r.Context(), service.RegisterInput{
		Name:          req.Name,
		Email:         req.Email,
		Password:      req.Password,
		BirthDate:     birthDate,
		Sex:           model.Sex(req.Sex),
		SearchSex:     model.SearchSex(req.SearchSex),
		DatingGoal:    model.DatingGoalByIntent[req.DatingIntent],
		AboutMe:       aboutMe,
		SearchAgeFrom: req.SearchAgeFrom,
		SearchAgeTo:   req.SearchAgeTo,
		Tags:          req.Tags,
	})
	switch {
	case errors.Is(err, model.ErrEmailAlreadyExists):
		writeError(w, http.StatusConflict, codeEmailAlreadyExists, "Почта уже занята", nil)
		return
	case errors.Is(err, model.ErrPasswordTooLong):
		writeError(w, http.StatusBadRequest, codeValidationError, "Некорректные данные",
			map[string]string{"password": validate.ErrPasswordTooLong.Error()})
		return
	case errors.Is(err, service.ErrSessionNotOpened):
		// Аккаунт уже есть, но выдать токены не получилось
		slog.Error("register user: session not opened", "user_id", res.UserID, "error", err)
		writeJSON(w, http.StatusCreated, AuthResponse{
			UserID:           res.UserID,
			ProfileCompleted: res.ProfileCompleted,
		})
		return
	case err != nil:
		slog.Error("register user", "error", err)
		writeError(w, http.StatusInternalServerError, codeInternalError, msgInternalError, nil)
		return
	}

	h.setSessionCookies(w, res.Tokens)
	writeJSON(w, http.StatusCreated, AuthResponse{
		UserID:           res.UserID,
		ProfileCompleted: res.ProfileCompleted,
	})
}

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (r *LoginRequest) Normalize() {
	r.Email = normalizeEmail(r.Email)
}

// normalizeEmail приводит строку к lower case и тримит ее
func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

func (r LoginRequest) Validate() map[string]string {
	errs := make(map[string]string)

	addErr(errs, "email", validate.ValidateEmail(r.Email))
	addErr(errs, "password", validate.ValidateLoginPassword(r.Password))

	return errs
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req LoginRequest
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
	case errors.Is(err, service.ErrInvalidCredentials):
		writeError(w, http.StatusUnauthorized, codeInvalidCredentials, "Неверная почта или пароль", nil)
		return
	case err != nil:
		slog.Error("login", "error", err)
		writeError(w, http.StatusInternalServerError, codeInternalError, msgInternalError, nil)
		return
	}

	h.setSessionCookies(w, res.Tokens)
	writeJSON(w, http.StatusOK, AuthResponse{
		UserID:           res.UserID,
		ProfileCompleted: res.ProfileCompleted,
	})
}

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
