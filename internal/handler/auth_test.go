package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"dating-app/internal/model"
	"dating-app/internal/service"
)

type fakeAuth struct {
	called   bool
	got      service.RegisterInput
	email    string
	password string
	logout   string
	res      service.AuthResult
	err      error
}

func (f *fakeAuth) Register(_ context.Context, in service.RegisterInput) (service.AuthResult, error) {
	f.called, f.got = true, in
	return f.res, f.err
}

func (f *fakeAuth) Login(_ context.Context, email, password string) (service.AuthResult, error) {
	f.called, f.email, f.password = true, email, password
	return f.res, f.err
}

func (f *fakeAuth) Logout(_ context.Context, refreshToken string) error {
	f.called, f.logout = true, refreshToken
	return f.err
}

var testTokens = service.Tokens{
	Access:           "acc",
	AccessExpiresAt:  time.Now().Add(15 * time.Minute),
	Refresh:          "ref",
	RefreshExpiresAt: time.Now().Add(720 * time.Hour),
}

// do вызывает метод AuthHandler и разбирает JSON-ответ (nil, если тела нет)
func do(t *testing.T, h http.HandlerFunc, path, body string, cookies ...*http.Cookie) (*httptest.ResponseRecorder, map[string]any) {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
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

// checkSessionCookies проверяет, что выставлены обе cookie сессии
func checkSessionCookies(t *testing.T, rec *httptest.ResponseRecorder) {
	t.Helper()
	got := map[string]*http.Cookie{}
	for _, c := range rec.Result().Cookies() {
		got[c.Name] = c
	}
	for name, want := range map[string]struct{ value, path string }{
		accessTokenCookie:  {"acc", "/"},
		refreshTokenCookie: {"ref", refreshTokenCookiePath},
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

const validRegisterBody = `{
	"name": "alex",
	"email": "  alex@example.com ",
	"password": "qwerty123",
	"birth_date": "2000-01-01",
	"sex": "male",
	"search_sex": "female",
	"dating_intent": "Ищу встречи",
	"search_age_from": 18,
	"search_age_to": 30,
	"about_me": "Люблю горы",
	"tags": ["sport"]
}`

func doRegister(t *testing.T, svc *fakeAuth, body string) (*httptest.ResponseRecorder, map[string]any) {
	t.Helper()
	return do(t, NewAuthHandler(svc).Register, "/api/v1/auth/register", body)
}

func TestRegisterHandler_Created(t *testing.T) {
	svc := &fakeAuth{res: service.AuthResult{UserID: 12, Tokens: testTokens}}
	rec, resp := doRegister(t, svc, validRegisterBody)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body)
	}
	if resp["user_id"] != float64(12) || resp["profile_completed"] != false {
		t.Errorf("body = %v", resp)
	}
	if svc.got.Email != "alex@example.com" {
		t.Errorf("email must be trimmed, got %q", svc.got.Email)
	}
	if svc.got.DatingGoal != model.DatingGoalCasual {
		t.Errorf("dating goal = %q, want %q", svc.got.DatingGoal, model.DatingGoalCasual)
	}
	if svc.got.BirthDate.Year() != 2000 || svc.got.AboutMe != "Люблю горы" {
		t.Errorf("input = %+v", svc.got)
	}
	checkSessionCookies(t, rec)
}

func TestRegisterHandler_PasswordTooLongForHash(t *testing.T) {
	rec, resp := doRegister(t, &fakeAuth{err: model.ErrPasswordTooLong}, validRegisterBody)
	if rec.Code != http.StatusBadRequest || errCode(resp) != codeValidationError {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body)
	}
}

func TestRegisterHandler_ValidationError(t *testing.T) {
	svc := &fakeAuth{}
	rec, resp := doRegister(t, svc, `{"email": "nope", "password": "123"}`)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d", rec.Code)
	}
	if svc.called {
		t.Error("service must not be called on invalid input")
	}
	e := resp["error"].(map[string]any)
	if e["code"] != codeValidationError {
		t.Errorf("code = %v", e["code"])
	}
	fields, _ := e["fields"].(map[string]any)
	for _, f := range []string{"email", "password", "name", "birth_date"} {
		if _, ok := fields[f]; !ok {
			t.Errorf("fields has no %q: %v", f, fields)
		}
	}
}

func TestRegisterHandler_BadJSON(t *testing.T) {
	for _, body := range []string{`{`, `[]`, `{"search_age_from": "18"}`} {
		svc := &fakeAuth{}
		rec, resp := doRegister(t, svc, body)
		if rec.Code != http.StatusBadRequest || svc.called {
			t.Errorf("body %s: status = %d, called = %v", body, rec.Code, svc.called)
		}
		if resp["error"].(map[string]any)["code"] != codeValidationError {
			t.Errorf("body %s: resp = %v", body, resp)
		}
	}
}

func TestRegisterHandler_EmailTaken(t *testing.T) {
	svc := &fakeAuth{err: model.ErrEmailAlreadyExists}
	rec, resp := doRegister(t, svc, validRegisterBody)

	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d", rec.Code)
	}
	if resp["error"].(map[string]any)["code"] != codeEmailAlreadyExists {
		t.Errorf("resp = %v", resp)
	}
}

func TestRegisterHandler_InternalError(t *testing.T) {
	svc := &fakeAuth{err: errors.New("db is down")}
	rec, resp := doRegister(t, svc, validRegisterBody)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d", rec.Code)
	}
	e := resp["error"].(map[string]any)
	if e["code"] != codeInternalError || strings.Contains(rec.Body.String(), "db is down") {
		t.Errorf("internal details must not leak: %s", rec.Body)
	}
}

func validRegister() RegisterRequest {
	return RegisterRequest{
		Name:          "alex",
		Email:         "alex@example.com",
		Password:      "qwerty123",
		BirthDate:     "2000-01-01",
		Sex:           "male",
		SearchSex:     "female",
		DatingIntent:  "Ищу половинку",
		SearchAgeFrom: 18,
		SearchAgeTo:   30,
		Tags:          []string{"sport"},
	}
}

func TestRegisterRequestValidate_OK(t *testing.T) {
	if errs := validRegister().Validate(); len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
}

func TestRegisterRequestValidate_CollectsAllFields(t *testing.T) {
	var r RegisterRequest
	errs := r.Validate()

	want := []string{"birth_date", "dating_intent", "email", "name", "password", "search_age_from", "search_age_to", "search_sex", "sex"}
	if got := keys(errs); !reflect.DeepEqual(got, want) {
		t.Fatalf("fields = %v, want %v", got, want)
	}
}

func TestRegisterRequestValidate_Underage(t *testing.T) {
	r := validRegister()
	r.BirthDate = "2099-01-01"
	if _, ok := r.Validate()["birth_date"]; !ok {
		t.Fatal("expected birth_date error")
	}
}

func TestRegisterRequestValidate_SearchAgeRange(t *testing.T) {
	r := validRegister()
	r.SearchAgeFrom, r.SearchAgeTo = 30, 20
	if got := keys(r.Validate()); !reflect.DeepEqual(got, []string{"search_age_to"}) {
		t.Fatalf("fields = %v, want [search_age_to]", got)
	}
}

func TestRegisterRequestNormalize(t *testing.T) {
	r := validRegister()
	r.Email = "  alex@example.com \n"
	r.Password = " qwerty123 "
	r.Normalize()

	if r.Email != "alex@example.com" {
		t.Errorf("email = %q", r.Email)
	}
	if r.Password != " qwerty123 " {
		t.Errorf("password must not be trimmed, got %q", r.Password)
	}
	if errs := r.Validate(); len(errs) != 0 {
		t.Errorf("unexpected errors: %v", errs)
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
		req  LoginRequest
		want []string
	}{
		{"ok", LoginRequest{Email: "a@b.ru", Password: "x"}, nil},
		{"пароль без правил сложности", LoginRequest{Email: "a@b.ru", Password: "abc"}, nil},
		{"пусто", LoginRequest{}, []string{"email", "password"}},
		{"кривой email", LoginRequest{Email: "nope", Password: "x"}, []string{"email"}},
	}
	for _, tt := range tests {
		if got := keys(tt.req.Validate()); !reflect.DeepEqual(got, tt.want) {
			t.Errorf("%s: fields = %v, want %v", tt.name, got, tt.want)
		}
	}
}

func doLogin(t *testing.T, svc *fakeAuth, body string) (*httptest.ResponseRecorder, map[string]any) {
	t.Helper()
	return do(t, NewAuthHandler(svc).Login, "/api/v1/auth/login", body)
}

func TestLoginHandler_OK(t *testing.T) {
	svc := &fakeAuth{res: service.AuthResult{UserID: 7, ProfileCompleted: true, Tokens: testTokens}}
	rec, resp := doLogin(t, svc, `{"email": " alex@example.com ", "password": "abc"}`)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body)
	}
	if resp["user_id"] != float64(7) || resp["profile_completed"] != true {
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
		{"битый JSON", `{`, nil, http.StatusBadRequest, codeValidationError},
		{"невалидные поля", `{"email": "nope"}`, nil, http.StatusBadRequest, codeValidationError},
		{"неверный пароль", `{"email": "a@b.ru", "password": "x"}`, service.ErrInvalidCredentials, http.StatusUnauthorized, codeInvalidCredentials},
		{"ошибка сервиса", `{"email": "a@b.ru", "password": "x"}`, errors.New("db down"), http.StatusInternalServerError, codeInternalError},
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
	h := NewAuthHandler(svc).Logout

	rec, _ := do(t, h, "/api/v1/auth/logout", "", &http.Cookie{Name: refreshTokenCookie, Value: "ref"})
	if rec.Code != http.StatusNoContent || svc.logout != "ref" {
		t.Fatalf("status = %d, logout(%q)", rec.Code, svc.logout)
	}
	cleared := 0
	for _, c := range rec.Result().Cookies() {
		if (c.Name == accessTokenCookie || c.Name == refreshTokenCookie) && c.MaxAge < 0 {
			cleared++
		}
	}
	if cleared != 2 {
		t.Errorf("both session cookies must be cleared, got %v", rec.Result().Cookies())
	}

	// Без cookie - всё равно 204, сервис не вызывается
	svc = &fakeAuth{}
	rec, _ = do(t, NewAuthHandler(svc).Logout, "/api/v1/auth/logout", "")
	if rec.Code != http.StatusNoContent || svc.called {
		t.Errorf("no cookie: status = %d, called = %v", rec.Code, svc.called)
	}

	rec, resp := do(t, NewAuthHandler(&fakeAuth{err: errors.New("redis down")}).Logout, "/api/v1/auth/logout", "",
		&http.Cookie{Name: refreshTokenCookie, Value: "ref"})
	if rec.Code != http.StatusInternalServerError || errCode(resp) != codeInternalError {
		t.Errorf("service error: status = %d", rec.Code)
	}
}
