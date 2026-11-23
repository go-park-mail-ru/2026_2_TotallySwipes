package handler

import (
	"context"
	. "dating-app/internal/handler"
	"dating-app/internal/handler/dto"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"dating-app/internal/model"
)

type fakeAuth struct {
	called   bool
	got      model.RegisterInput
	email    string
	password string
	logout   string
	refresh  string
	res      model.AuthResult
	err      error
}

func (f *fakeAuth) Register(_ context.Context, in model.RegisterInput) (model.AuthResult, error) {
	f.called, f.got = true, in
	return f.res, f.err
}

func (f *fakeAuth) Login(_ context.Context, email, password string) (model.AuthResult, error) {
	f.called, f.email, f.password = true, email, password
	return f.res, f.err
}

func (f *fakeAuth) Logout(_ context.Context, refreshToken string) error {
	f.called, f.logout = true, refreshToken
	return f.err
}

func (f *fakeAuth) Refresh(_ context.Context, refreshToken string) (model.Tokens, error) {
	f.called, f.refresh = true, refreshToken
	return f.res.Tokens, f.err
}

var testTokens = model.Tokens{
	Access:           "acc",
	AccessExpiresAt:  time.Now().Add(15 * time.Minute),
	Refresh:          "ref",
	RefreshExpiresAt: time.Now().Add(720 * time.Hour),
}

func do(t *testing.T, h http.HandlerFunc, path, body string, cookies ...*http.Cookie) (*httptest.ResponseRecorder, map[string]any) {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	for _, c := range cookies {
		req.AddCookie(c)
	}
	rec := httptest.NewRecorder()
	h(rec, req)

	if rec.Body.Len() == 0 {
		return rec, nil
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q", ct)
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("response is not JSON: %q", rec.Body.String())
	}
	return rec, resp
}

func errCode(resp map[string]any) any {
	e, _ := resp["error"].(map[string]any)
	return e["code"]
}

func checkSessionCookies(t *testing.T, rec *httptest.ResponseRecorder) {
	t.Helper()
	got := map[string]*http.Cookie{}
	for _, c := range rec.Result().Cookies() {
		got[c.Name] = c
	}
	for name, want := range map[string]struct{ value, path string }{
		"access_token":  {"acc", "/"},
		"refresh_token": {"ref", "/api/v1/auth"},
	} {
		c, ok := got[name]
		if !ok {
			t.Errorf("cookie %s not set", name)
			continue
		}
		if c.Value != want.value || c.Path != want.path || !c.HttpOnly || !c.Secure {
			t.Errorf("cookie %s = %+v", name, c)
		}
	}
}

func doRegister(t *testing.T, svc *fakeAuth, body string) (*httptest.ResponseRecorder, map[string]any) {
	t.Helper()
	return do(t, NewAuthHandler(svc, true).Register, "/api/v1/auth/register", body)
}

const validRegisterBody = `{"email": "  Alex@Example.com ", "password": "qwerty123"}`

func errFields(resp map[string]any) map[string]any {
	e, _ := resp["error"].(map[string]any)
	f, _ := e["fields"].(map[string]any)
	return f
}

func TestRegisterHandler_Created(t *testing.T) {
	svc := &fakeAuth{res: model.AuthResult{UserID: 12, Missing: []string{"name", "photos"}, Tokens: testTokens}}
	rec, resp := doRegister(t, svc, validRegisterBody)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body)
	}
	if resp["user_id"] != float64(12) || !reflect.DeepEqual(resp["missing"], []any{"name", "photos"}) {
		t.Errorf("body = %v", resp)
	}
	if svc.got != (model.RegisterInput{Email: "alex@example.com", Password: "qwerty123"}) {
		t.Errorf("input = %+v", svc.got)
	}
	checkSessionCookies(t, rec)
}

func TestRegisterHandler_ValidationError(t *testing.T) {
	for name, body := range map[string]string{
		"пусто":         `{}`,
		"плохие поля":   `{"email": "nope", "password": "123"}`,
		"старый формат": `{"email": "a@b.ru", "password": "qwerty123", "name": "alex"}`,
	} {
		svc := &fakeAuth{}
		rec, resp := doRegister(t, svc, body)
		if rec.Code != http.StatusBadRequest || errCode(resp) != "VALIDATION_ERROR" || svc.called {
			t.Errorf("%s: status = %d, body = %s", name, rec.Code, rec.Body)
		}
	}

	_, resp := doRegister(t, &fakeAuth{}, `{"email": "nope", "password": "123"}`)
	if got := errFields(resp); got["email"] == nil || got["password"] == nil {
		t.Errorf("fields = %v", got)
	}
}

func TestRegisterHandler_ServiceErrors(t *testing.T) {
	for _, tc := range []struct {
		name   string
		err    error
		status int
		code   string
	}{
		{"почта занята", model.ErrEmailAlreadyExists, http.StatusConflict, "EMAIL_ALREADY_EXISTS"},
		{"пароль длиннее bcrypt", model.ErrPasswordTooLong, http.StatusBadRequest, "VALIDATION_ERROR"},
		{"сбой", errors.New("db is down"), http.StatusInternalServerError, "INTERNAL_SERVER_ERROR"},
	} {
		rec, resp := doRegister(t, &fakeAuth{err: tc.err}, validRegisterBody)
		if rec.Code != tc.status || errCode(resp) != tc.code || strings.Contains(rec.Body.String(), "db is down") {
			t.Errorf("%s: status = %d, body = %s", tc.name, rec.Code, rec.Body)
		}
		if len(rec.Result().Cookies()) != 0 {
			t.Errorf("%s: cookies must not be set", tc.name)
		}
	}
}

func TestRegisterHandler_SessionNotOpened(t *testing.T) {
	svc := &fakeAuth{
		res: model.AuthResult{UserID: 12},
		err: fmt.Errorf("%w: redis down", model.ErrSessionNotOpened),
	}
	rec, resp := doRegister(t, svc, validRegisterBody)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body)
	}
	if resp["user_id"] != float64(12) || !reflect.DeepEqual(resp["missing"], []any{}) {
		t.Errorf("body = %v", resp)
	}
	if cookies := rec.Result().Cookies(); len(cookies) != 0 {
		t.Errorf("no session cookies expected, got %v", cookies)
	}
}

func TestRegisterRequest(t *testing.T) {
	r := dto.RegisterRequest{Email: "  Alex@Example.COM \n", Password: " qwerty123 "}
	r.Normalize()
	if r.Email != "alex@example.com" || r.Password != " qwerty123 " {
		t.Errorf("normalized = %+v", r)
	}
	if errs := r.Validate(); len(errs) != 0 {
		t.Errorf("unexpected errors: %v", errs)
	}
	if got := keys(dto.RegisterRequest{}.Validate()); !reflect.DeepEqual(got, []string{"email", "password"}) {
		t.Errorf("fields = %v", got)
	}
}

func TestLoginRequestNormalize(t *testing.T) {
	r := dto.LoginRequest{Email: " Alex@Example.COM ", Password: " Qwerty123 "}
	r.Normalize()

	if r.Email != "alex@example.com" {
		t.Errorf("email = %q", r.Email)
	}
	if r.Password != " Qwerty123 " {
		t.Errorf("password must not be changed, got %q", r.Password)
	}
}

func keys(m map[string]string) []string {
	if len(m) == 0 {
		return nil
	}
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func TestLoginRequestValidate(t *testing.T) {
	tests := []struct {
		name string
		req  dto.LoginRequest
		want []string
	}{
		{"ok", dto.LoginRequest{Email: "a@b.ru", Password: "x"}, nil},
		{"пароль без правил сложности", dto.LoginRequest{Email: "a@b.ru", Password: "abc"}, nil},
		{"пусто", dto.LoginRequest{}, []string{"email", "password"}},
		{"кривой email", dto.LoginRequest{Email: "nope", Password: "x"}, []string{"email"}},
	}
	for _, tt := range tests {
		if got := keys(tt.req.Validate()); !reflect.DeepEqual(got, tt.want) {
			t.Errorf("%s: fields = %v, want %v", tt.name, got, tt.want)
		}
	}
}

func doLogin(t *testing.T, svc *fakeAuth, body string) (*httptest.ResponseRecorder, map[string]any) {
	t.Helper()
	return do(t, NewAuthHandler(svc, true).Login, "/api/v1/auth/login", body)
}

func TestLoginHandler_OK(t *testing.T) {
	svc := &fakeAuth{res: model.AuthResult{UserID: 7, Missing: []string{}, Tokens: testTokens}}
	rec, resp := doLogin(t, svc, `{"email": " alex@example.com ", "password": "abc"}`)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body)
	}
	if resp["user_id"] != float64(7) || !reflect.DeepEqual(resp["missing"], []any{}) {
		t.Errorf("body = %v", resp)
	}
	if svc.email != "alex@example.com" || svc.password != "abc" {
		t.Errorf("service got %q / %q", svc.email, svc.password)
	}
	checkSessionCookies(t, rec)
}

func TestLoginHandler_Errors(t *testing.T) {
	tests := []struct {
		name   string
		body   string
		err    error
		status int
		code   string
	}{
		{"битый JSON", `{`, nil, http.StatusBadRequest, "VALIDATION_ERROR"},
		{"невалидные поля", `{"email": "nope"}`, nil, http.StatusBadRequest, "VALIDATION_ERROR"},
		{"неверный пароль", `{"email": "a@b.ru", "password": "x"}`, model.ErrInvalidCredentials, http.StatusUnauthorized, "INVALID_CREDENTIALS"},
		{"ошибка сервиса", `{"email": "a@b.ru", "password": "x"}`, errors.New("db down"), http.StatusInternalServerError, "INTERNAL_SERVER_ERROR"},
	}
	for _, tt := range tests {
		rec, resp := doLogin(t, &fakeAuth{err: tt.err}, tt.body)
		if rec.Code != tt.status || errCode(resp) != tt.code {
			t.Errorf("%s: status = %d, body = %s", tt.name, rec.Code, rec.Body)
		}
		if len(rec.Result().Cookies()) != 0 {
			t.Errorf("%s: cookies must not be set", tt.name)
		}
	}
}

func TestLogoutHandler(t *testing.T) {
	svc := &fakeAuth{}
	h := NewAuthHandler(svc, true).Logout

	rec, _ := do(t, h, "/api/v1/auth/logout", "", &http.Cookie{Name: "refresh_token", Value: "ref"})
	if rec.Code != http.StatusNoContent || svc.logout != "ref" {
		t.Fatalf("status = %d, logout(%q)", rec.Code, svc.logout)
	}
	assertSessionCleared(t, rec)

	svc = &fakeAuth{}
	rec, _ = do(t, NewAuthHandler(svc, true).Logout, "/api/v1/auth/logout", "")
	if rec.Code != http.StatusNoContent || svc.called {
		t.Errorf("no cookie: status = %d, called = %v", rec.Code, svc.called)
	}

	rec, resp := do(t, NewAuthHandler(&fakeAuth{err: errors.New("redis down")}, true).Logout, "/api/v1/auth/logout", "",
		&http.Cookie{Name: "refresh_token", Value: "ref"})
	if rec.Code != http.StatusInternalServerError || errCode(resp) != "INTERNAL_SERVER_ERROR" {
		t.Errorf("service error: status = %d", rec.Code)
	}
	assertSessionCleared(t, rec)
}

func assertSessionCleared(t *testing.T, rec *httptest.ResponseRecorder) {
	t.Helper()
	cleared := 0
	for _, c := range rec.Result().Cookies() {
		if (c.Name == "access_token" || c.Name == "refresh_token") && c.MaxAge < 0 {
			cleared++
		}
	}
	if cleared != 2 {
		t.Errorf("both session cookies must be cleared, got %v", rec.Result().Cookies())
	}
}

func TestRefreshHandler(t *testing.T) {
	refreshCookie := &http.Cookie{Name: "refresh_token", Value: "ref-old"}

	svc := &fakeAuth{res: model.AuthResult{Tokens: testTokens}}
	rec, resp := do(t, NewAuthHandler(svc, true).Refresh, "/api/v1/auth/refresh", "", refreshCookie)
	if rec.Code != http.StatusNoContent || resp != nil || svc.refresh != "ref-old" {
		t.Fatalf("status = %d, body = %v, refresh(%q)", rec.Code, resp, svc.refresh)
	}
	checkSessionCookies(t, rec)

	svc = &fakeAuth{}
	rec, resp = do(t, NewAuthHandler(svc, true).Refresh, "/api/v1/auth/refresh", "")
	if rec.Code != http.StatusUnauthorized || errCode(resp) != "UNAUTHORIZED" || svc.called {
		t.Errorf("no cookie: status = %d, code = %v, called = %v", rec.Code, errCode(resp), svc.called)
	}
	assertSessionCleared(t, rec)

	rec, resp = do(t, NewAuthHandler(&fakeAuth{err: fmt.Errorf("x: %w", model.ErrInvalidSession)}, true).Refresh,
		"/api/v1/auth/refresh", "", refreshCookie)
	if rec.Code != http.StatusUnauthorized || errCode(resp) != "UNAUTHORIZED" {
		t.Errorf("invalid session: status = %d, code = %v", rec.Code, errCode(resp))
	}
	assertSessionCleared(t, rec)

	rec, resp = do(t, NewAuthHandler(&fakeAuth{err: errors.New("redis down")}, true).Refresh,
		"/api/v1/auth/refresh", "", refreshCookie)
	if rec.Code != http.StatusInternalServerError || errCode(resp) != "INTERNAL_SERVER_ERROR" {
		t.Errorf("service error: status = %d", rec.Code)
	}
	if len(rec.Result().Cookies()) != 0 {
		t.Errorf("cookies must be kept on 500, got %v", rec.Result().Cookies())
	}
}

func TestLoginHandler_StrictJSON(t *testing.T) {
	for _, tc := range []struct {
		name, contentType, body string
		status                  int
	}{
		{"без Content-Type", "", `{"email":"a@b.ru","password":"x"}`, http.StatusUnsupportedMediaType},
		{"неизвестное поле", "application/json", `{"email":"a@b.ru","password":"x","admin":true}`, http.StatusBadRequest},
		{"два объекта", "application/json", `{"email":"a@b.ru","password":"x"}{}`, http.StatusBadRequest},
	} {
		svc := &fakeAuth{}
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(tc.body))
		req.Header.Set("Content-Type", tc.contentType)
		rec := httptest.NewRecorder()
		NewAuthHandler(svc, true).Login(rec, req)
		if rec.Code != tc.status || svc.called {
			t.Errorf("%s: status = %d, called = %v", tc.name, rec.Code, svc.called)
		}
	}
}
