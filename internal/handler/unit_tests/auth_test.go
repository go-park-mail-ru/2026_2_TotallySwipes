package handler

import (
	"bytes"
	"context"
	. "dating-app/internal/handler"
	"dating-app/internal/handler/dto"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
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
	taken    bool
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

func (f *fakeAuth) IsEmailAvailable(_ context.Context, email string) (bool, error) {
	f.called, f.email = true, email
	return !f.taken, f.err
}

var testTokens = model.Tokens{
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

var (
	jpegData = []byte("\xff\xd8\xff\xe0 jpeg")
	pngData  = []byte("\x89PNG\r\n\x1a\n png")
)

type registerForm struct {
	fields map[string][]string
	photos [][]byte
}

func validRegisterForm() registerForm {
	return registerForm{
		fields: map[string][]string{
			"name":            {"alex"},
			"email":           {"  alex@example.com "},
			"password":        {"qwerty123"},
			"birth_date":      {"2000-01-01"},
			"sex":             {"male"},
			"search_sex":      {"female"},
			"dating_intent":   {"Ищу встречи"},
			"search_age_from": {"18"},
			"search_age_to":   {"30"},
			"about_me":        {"Люблю горы"},
			"tags":            {"sport", "music"},
		},
		photos: [][]byte{jpegData, pngData},
	}
}

func multipartBody(t *testing.T, form registerForm) (*bytes.Buffer, string) {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	for name, values := range form.fields {
		for _, v := range values {
			if err := mw.WriteField(name, v); err != nil {
				t.Fatal(err)
			}
		}
	}
	for i, data := range form.photos {
		fw, err := mw.CreateFormFile("photos", fmt.Sprintf("photo%d", i))
		if err != nil {
			t.Fatal(err)
		}
		fw.Write(data)
	}
	if err := mw.Close(); err != nil {
		t.Fatal(err)
	}
	return &buf, mw.FormDataContentType()
}

func doRegisterRaw(t *testing.T, svc *fakeAuth, body io.Reader, contentType string) (*httptest.ResponseRecorder, map[string]any) {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", body)
	req.Header.Set("Content-Type", contentType)
	rec := httptest.NewRecorder()
	NewAuthHandler(svc, true).Register(rec, req)

	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("response is not JSON: %q", rec.Body.String())
	}
	return rec, resp
}

func doRegister(t *testing.T, svc *fakeAuth, form registerForm) (*httptest.ResponseRecorder, map[string]any) {
	t.Helper()
	body, contentType := multipartBody(t, form)
	return doRegisterRaw(t, svc, body, contentType)
}

func errFields(resp map[string]any) map[string]any {
	e, _ := resp["error"].(map[string]any)
	f, _ := e["fields"].(map[string]any)
	return f
}

func TestRegisterHandler_Created(t *testing.T) {
	svc := &fakeAuth{res: model.AuthResult{UserID: 12, Tokens: testTokens}}
	rec, resp := doRegister(t, svc, validRegisterForm())

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
	if svc.got.BirthDate.Year() != 2000 || svc.got.AboutMe != "Люблю горы" || svc.got.SearchAgeTo != 30 {
		t.Errorf("input = %+v", svc.got)
	}
	if !reflect.DeepEqual(svc.got.Tags, []string{"sport", "music"}) {
		t.Errorf("tags = %v", svc.got.Tags)
	}
	wantPhotos := []model.PhotoUpload{{Data: jpegData, Ext: ".jpg"}, {Data: pngData, Ext: ".png"}}
	if !reflect.DeepEqual(svc.got.Photos, wantPhotos) {
		t.Errorf("photos = %+v", svc.got.Photos)
	}
	checkSessionCookies(t, rec)
}

func TestRegisterHandler_OptionalFields(t *testing.T) {
	form := validRegisterForm()
	delete(form.fields, "about_me")
	delete(form.fields, "tags")
	svc := &fakeAuth{res: model.AuthResult{UserID: 1, Tokens: testTokens}}
	rec, _ := doRegister(t, svc, form)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body)
	}
	if svc.got.AboutMe != "" || len(svc.got.Tags) != 0 {
		t.Errorf("input = %+v", svc.got)
	}
}

func TestRegisterHandler_PasswordTooLongForHash(t *testing.T) {
	rec, resp := doRegister(t, &fakeAuth{err: model.ErrPasswordTooLong}, validRegisterForm())
	if rec.Code != http.StatusBadRequest || errCode(resp) != "VALIDATION_ERROR" {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body)
	}
}

func TestRegisterHandler_ValidationError(t *testing.T) {
	svc := &fakeAuth{}
	rec, resp := doRegister(t, svc, registerForm{fields: map[string][]string{
		"email":           {"nope"},
		"password":        {"123"},
		"search_age_from": {"abc"},
	}})

	if rec.Code != http.StatusBadRequest || errCode(resp) != "VALIDATION_ERROR" {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body)
	}
	if svc.called {
		t.Error("service must not be called on invalid input")
	}
	fields := errFields(resp)
	for _, f := range []string{"email", "password", "name", "birth_date", "photos"} {
		if _, ok := fields[f]; !ok {
			t.Errorf("fields has no %q: %v", f, fields)
		}
	}
	if fields["search_age_from"] != "должно быть целым числом" || fields["search_age_to"] != "обязательное поле" {
		t.Errorf("search ages: %v", fields)
	}
}

func TestRegisterHandler_InvalidPhotos(t *testing.T) {
	tooMany := validRegisterForm()
	tooMany.photos = [][]byte{jpegData, jpegData, jpegData, jpegData, jpegData, jpegData, jpegData}

	notImage := validRegisterForm()
	notImage.photos = [][]byte{jpegData, []byte("GIF89a not supported")}

	tooBig := validRegisterForm()
	tooBig.photos = [][]byte{append(append([]byte{}, jpegData...), make([]byte, 5<<20)...)}

	noPhotos := validRegisterForm()
	noPhotos.photos = nil
	noPhotos.fields["photos"] = []string{"https://example.com/a.jpg"}

	for name, form := range map[string]registerForm{"too many": tooMany, "not image": notImage, "too big": tooBig, "as text": noPhotos} {
		svc := &fakeAuth{}
		rec, resp := doRegister(t, svc, form)
		if rec.Code != http.StatusBadRequest || svc.called {
			t.Errorf("%s: status = %d, called = %v", name, rec.Code, svc.called)
		}
		if _, ok := errFields(resp)["photos"]; !ok {
			t.Errorf("%s: fields = %v", name, errFields(resp))
		}
	}
}

func TestRegisterHandler_NotMultipart(t *testing.T) {
	for _, contentType := range []string{"application/json", "multipart/form-data"} {
		svc := &fakeAuth{}
		rec, resp := doRegisterRaw(t, svc, strings.NewReader(`{"name":"alex"}`), contentType)
		if rec.Code != http.StatusBadRequest || errCode(resp) != "VALIDATION_ERROR" || svc.called {
			t.Errorf("%s: status = %d, resp = %v", contentType, rec.Code, resp)
		}
	}
}

func TestRegisterHandler_TooLarge(t *testing.T) {
	form := validRegisterForm()
	big := append(append([]byte{}, jpegData...), make([]byte, 5<<20-100)...)
	form.photos = [][]byte{big, big, big, big, big, big, big}

	svc := &fakeAuth{}
	rec, resp := doRegister(t, svc, form)
	if rec.Code != http.StatusRequestEntityTooLarge || errCode(resp) != "PAYLOAD_TOO_LARGE" || svc.called {
		t.Errorf("status = %d, resp = %v", rec.Code, resp)
	}
}

func TestRegisterHandler_EmailTaken(t *testing.T) {
	svc := &fakeAuth{err: model.ErrEmailAlreadyExists}
	rec, resp := doRegister(t, svc, validRegisterForm())

	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d", rec.Code)
	}
	if resp["error"].(map[string]any)["code"] != "EMAIL_ALREADY_EXISTS" {
		t.Errorf("resp = %v", resp)
	}
}

func TestRegisterHandler_SessionNotOpened(t *testing.T) {
	svc := &fakeAuth{
		res: model.AuthResult{UserID: 12},
		err: fmt.Errorf("%w: redis down", model.ErrSessionNotOpened),
	}
	rec, resp := doRegister(t, svc, validRegisterForm())

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body)
	}
	if resp["user_id"] != float64(12) {
		t.Errorf("body = %v", resp)
	}
	if cookies := rec.Result().Cookies(); len(cookies) != 0 {
		t.Errorf("no session cookies expected, got %v", cookies)
	}
}

func TestRegisterHandler_InternalError(t *testing.T) {
	svc := &fakeAuth{err: errors.New("db is down")}
	rec, resp := doRegister(t, svc, validRegisterForm())

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d", rec.Code)
	}
	e := resp["error"].(map[string]any)
	if e["code"] != "INTERNAL_SERVER_ERROR" || strings.Contains(rec.Body.String(), "db is down") {
		t.Errorf("internal details must not leak: %s", rec.Body)
	}
}

func validRegister() dto.RegisterRequest {
	return dto.RegisterRequest{
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
		Photos:        []model.PhotoUpload{{Data: []byte("x"), Ext: ".jpg"}},
	}
}

func TestRegisterRequestValidate_OK(t *testing.T) {
	if errs := validRegister().Validate(); len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
}

func TestRegisterRequestValidate_CollectsAllFields(t *testing.T) {
	var r dto.RegisterRequest
	errs := r.Validate()

	want := []string{"birth_date", "dating_intent", "email", "name", "password", "photos", "search_age_from", "search_age_to", "search_sex", "sex"}
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
	r.Name = "  Анна   Мария "
	r.Email = "  Alex@Example.COM \n"
	r.Password = " qwerty123 "
	r.Normalize()

	if r.Name != "Анна Мария" {
		t.Errorf("name = %q", r.Name)
	}

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
	svc := &fakeAuth{res: model.AuthResult{UserID: 7, ProfileCompleted: true, Tokens: testTokens}}
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

func TestCheckEmailHandler(t *testing.T) {
	for _, taken := range []bool{false, true} {
		svc := &fakeAuth{taken: taken}
		rec, resp := do(t, NewAuthHandler(svc, true).CheckEmail, "/api/v1/auth/email/check", `{"email": " Alex@Example.com "}`)
		if rec.Code != http.StatusOK || resp["available"] != !taken {
			t.Errorf("taken=%v: status = %d, body = %s", taken, rec.Code, rec.Body)
		}
		if svc.email != "alex@example.com" {
			t.Errorf("email must be normalized, got %q", svc.email)
		}
	}
}

func TestCheckEmailHandler_Errors(t *testing.T) {
	tests := []struct {
		name   string
		body   string
		err    error
		status int
		code   string
	}{
		{"битый JSON", `{`, nil, http.StatusBadRequest, "VALIDATION_ERROR"},
		{"пустой email", `{}`, nil, http.StatusBadRequest, "VALIDATION_ERROR"},
		{"кривой email", `{"email": "nope"}`, nil, http.StatusBadRequest, "VALIDATION_ERROR"},
		{"ошибка сервиса", `{"email": "a@b.ru"}`, errors.New("db down"), http.StatusInternalServerError, "INTERNAL_SERVER_ERROR"},
	}
	for _, tt := range tests {
		svc := &fakeAuth{err: tt.err}
		rec, resp := do(t, NewAuthHandler(svc, true).CheckEmail, "/api/v1/auth/email/check", tt.body)
		if rec.Code != tt.status || errCode(resp) != tt.code {
			t.Errorf("%s: status = %d, body = %s", tt.name, rec.Code, rec.Body)
		}
		if tt.status == http.StatusBadRequest && svc.called {
			t.Errorf("%s: service must not be called", tt.name)
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

	// Без cookie - всё равно 204, сервис не вызывается
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
