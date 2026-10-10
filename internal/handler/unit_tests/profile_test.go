package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	. "dating-app/internal/handler"
	"dating-app/internal/model"

	"github.com/gorilla/mux"
)

type fakeProfiles struct {
	patch    *model.ProfilePatch
	upload   *model.PhotoUpload
	deleted  int64
	order    []int64
	profile  model.Profile
	photos   []model.Photo
	err      error
	patchErr error
}

func (f *fakeProfiles) GetShortProfile(context.Context, int64) (*model.ProfileShort, error) {
	return &model.ProfileShort{UserID: 7, Missing: []string{"photos"}}, f.err
}
func (f *fakeProfiles) GetMyProfile(context.Context, int64) (*model.Profile, error) {
	return &f.profile, f.err
}
func (f *fakeProfiles) UpdateProfile(_ context.Context, _ int64, patch *model.ProfilePatch) (*model.Profile, error) {
	f.patch = patch
	if f.patchErr != nil {
		return nil, f.patchErr
	}
	return &f.profile, nil
}
func (f *fakeProfiles) AddPhoto(_ context.Context, _ int64, upload model.PhotoUpload) ([]model.Photo, error) {
	f.upload = &upload
	return f.photos, f.err
}
func (f *fakeProfiles) DeletePhoto(_ context.Context, _, photoID int64) ([]model.Photo, error) {
	f.deleted = photoID
	return f.photos, f.err
}

func (f *fakeProfiles) ReorderPhotos(_ context.Context, _ int64, photoIDs []int64) ([]model.Photo, error) {
	f.order = photoIDs
	return f.photos, f.err
}

func profileRouter(svc *fakeProfiles) http.Handler {
	h := NewProfileHandler(svc)
	withUser := func(fn UserHandlerFunc) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fn(w, r, 7) })
	}
	r := mux.NewRouter()
	r.Handle("/profile/me", withUser(h.Get)).Methods(http.MethodGet)
	r.Handle("/profile/me", withUser(h.Update)).Methods(http.MethodPatch)
	r.Handle("/profile/me/short", withUser(h.ShortProfile)).Methods(http.MethodGet)
	r.Handle("/profile/me/photos", withUser(h.AddPhoto)).Methods(http.MethodPost)
	r.Handle("/profile/me/photos/order", withUser(h.ReorderPhotos)).Methods(http.MethodPut)
	r.Handle("/profile/me/photos/{photo_id}", withUser(h.DeletePhoto)).Methods(http.MethodDelete)
	return r
}

func serve(t *testing.T, h http.Handler, req *http.Request) (*httptest.ResponseRecorder, map[string]any) {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("response is not JSON: %q", rec.Body.String())
	}
	return rec, resp
}

func patchProfile(t *testing.T, svc *fakeProfiles, body string) (*httptest.ResponseRecorder, map[string]any) {
	t.Helper()
	req := httptest.NewRequest(http.MethodPatch, "/profile/me", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	return serve(t, profileRouter(svc), req)
}

func TestProfileHandler_GetEmpty(t *testing.T) {
	svc := &fakeProfiles{profile: model.Profile{UserID: 7}}
	rec, resp := serve(t, profileRouter(svc), httptest.NewRequest(http.MethodGet, "/profile/me", nil))
	if rec.Code != http.StatusOK || resp["name"] != nil || len(resp["missing"].([]any)) != 6 {
		t.Fatalf("status = %d, body = %v", rec.Code, resp)
	}
	if !reflect.DeepEqual(resp["photos"], []any{}) || !reflect.DeepEqual(resp["tags"], []any{}) {
		t.Errorf("lists must be empty arrays: %v", resp)
	}
}

func TestProfileHandler_Short(t *testing.T) {
	rec, resp := serve(t, profileRouter(&fakeProfiles{}), httptest.NewRequest(http.MethodGet, "/profile/me/short", nil))
	if rec.Code != http.StatusOK || !reflect.DeepEqual(resp["missing"], []any{"photos"}) {
		t.Fatalf("status = %d, body = %v", rec.Code, resp)
	}
}

func TestProfileHandler_Patch(t *testing.T) {
	svc := &fakeProfiles{profile: model.Profile{UserID: 7}}
	rec, _ := patchProfile(t, svc, `{"name": "  Анна  ", "birth_date": "2000-05-01", "dating_goal": "casual", "about_me": null, "tags": ["coffee"]}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body)
	}
	p := svc.patch
	if *p.Name != "Анна" || p.BirthDate.Year() != 2000 || *p.DatingGoal != model.DatingGoalCasual ||
		!p.AboutMe.Set || p.AboutMe.Value != nil || !reflect.DeepEqual(*p.Tags, []string{"coffee"}) || p.Sex != nil {
		t.Errorf("patch = %+v", p)
	}

	svc = &fakeProfiles{profile: model.Profile{UserID: 7}}
	if rec, _ := patchProfile(t, svc, `{"sex": "female"}`); rec.Code != http.StatusOK || svc.patch.AboutMe.Set || svc.patch.Height.Set {
		t.Errorf("absent about_me must not be touched: %d %+v", rec.Code, svc.patch)
	}
}

func TestProfileHandler_PatchDetails(t *testing.T) {
	svc := &fakeProfiles{profile: model.Profile{UserID: 7}}
	rec, _ := patchProfile(t, svc, `{"education": "higher", "work": "  Инженер  ", "smoking": "negative", "alcohol": null, "height": 180}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body)
	}
	p := svc.patch
	if *p.Education.Value != model.EducationHigher || *p.Work.Value != "Инженер" || *p.Smoking.Value != model.AttitudeNegative ||
		!p.Alcohol.Set || p.Alcohol.Value != nil || *p.Height.Value != 180 {
		t.Errorf("patch = %+v", p)
	}

	svc = &fakeProfiles{profile: model.Profile{UserID: 7}}
	if rec, _ := patchProfile(t, svc, `{"work": "   ", "height": null}`); rec.Code != http.StatusOK ||
		!svc.patch.Work.Set || svc.patch.Work.Value != nil || !svc.patch.Height.Set || svc.patch.Height.Value != nil {
		t.Errorf("blank work and null height must clear: %d %+v", rec.Code, svc.patch)
	}
}

func TestProfileHandler_PatchValidation(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		fields     []string
	}{
		{"пустой патч", `{}`, []string{"profile"}},
		{"цель знакомства подписью", `{"dating_goal": "Ищу половинку"}`, []string{"dating_goal"}},
		{"несовершеннолетний", `{"birth_date": "2099-01-01"}`, []string{"birth_date"}},
		{"образование подписью", `{"education": "Высшее"}`, []string{"education"}},
		{"неизвестное отношение", `{"smoking": "sometimes", "alcohol": ""}`, []string{"alcohol", "smoking"}},
		{"рост вне диапазона", `{"height": 99}`, []string{"height"}},
		{"длинная работа", `{"work": "` + strings.Repeat("я", 101) + `"}`, []string{"work"}},
		{"несколько ошибок", `{"sex": "x", "dating_goal": "y", "tags": ["a", "a"]}`, []string{"dating_goal", "sex", "tags"}},
	} {
		svc := &fakeProfiles{}
		rec, resp := patchProfile(t, svc, tc.body)
		if rec.Code != http.StatusBadRequest || svc.patch != nil {
			t.Errorf("%s: status = %d, called = %v", tc.name, rec.Code, svc.patch != nil)
			continue
		}
		got := make(map[string]string)
		for k, v := range errFields(resp) {
			got[k] = fmt.Sprint(v)
		}
		if !reflect.DeepEqual(keys(got), tc.fields) {
			t.Errorf("%s: fields = %v, want %v", tc.name, keys(got), tc.fields)
		}
	}

	rec, resp := patchProfile(t, &fakeProfiles{patchErr: fmt.Errorf("x: %w", model.ErrUnknownTag)}, `{"tags": ["nope"]}`)
	if rec.Code != http.StatusBadRequest || errFields(resp)["tags"] == nil {
		t.Errorf("unknown tag: status = %d, body = %v", rec.Code, resp)
	}
}

func uploadPhoto(t *testing.T, svc *fakeProfiles, files ...[]byte) (*httptest.ResponseRecorder, map[string]any) {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	for i, data := range files {
		fw, err := mw.CreateFormFile("photo", fmt.Sprintf("p%d", i))
		if err != nil {
			t.Fatal(err)
		}
		fw.Write(data)
	}
	mw.Close()
	req := httptest.NewRequest(http.MethodPost, "/profile/me/photos", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	return serve(t, profileRouter(svc), req)
}

var jpegData = []byte("\xff\xd8\xff\xe0 jpeg")

func TestProfileHandler_AddPhoto(t *testing.T) {
	svc := &fakeProfiles{photos: []model.Photo{{ID: 3, URL: "u3"}, {ID: 9, URL: "u9"}}}
	rec, resp := uploadPhoto(t, svc, jpegData)
	if rec.Code != http.StatusCreated || svc.upload == nil || svc.upload.Ext != ".jpg" {
		t.Fatalf("status = %d, upload = %+v", rec.Code, svc.upload)
	}
	want := []any{map[string]any{"id": float64(3), "url": "u3"}, map[string]any{"id": float64(9), "url": "u9"}}
	if !reflect.DeepEqual(resp["photos"], want) {
		t.Errorf("photos = %v", resp["photos"])
	}

	for name, files := range map[string][][]byte{
		"без файла":       nil,
		"два файла":       {jpegData, jpegData},
		"не картинка":     {[]byte("GIF89a")},
		"слишком большой": {append(append([]byte{}, jpegData...), make([]byte, 5<<20)...)},
	} {
		svc := &fakeProfiles{}
		rec, resp := uploadPhoto(t, svc, files...)
		if rec.Code != http.StatusBadRequest || svc.upload != nil || errFields(resp)["photo"] == nil {
			t.Errorf("%s: status = %d, body = %v", name, rec.Code, resp)
		}
	}

	rec, resp = uploadPhoto(t, &fakeProfiles{err: model.ErrPhotoLimit}, jpegData)
	if rec.Code != http.StatusConflict || errCode(resp) != "PHOTO_LIMIT" {
		t.Errorf("limit: status = %d, body = %v", rec.Code, resp)
	}
}

func TestProfileHandler_DeletePhoto(t *testing.T) {
	svc := &fakeProfiles{photos: []model.Photo{}}
	rec, resp := serve(t, profileRouter(svc), httptest.NewRequest(http.MethodDelete, "/profile/me/photos/5", nil))
	if rec.Code != http.StatusOK || svc.deleted != 5 || !reflect.DeepEqual(resp["photos"], []any{}) {
		t.Fatalf("status = %d, deleted = %d, body = %v", rec.Code, svc.deleted, resp)
	}

	for _, tc := range []struct {
		path   string
		err    error
		status int
		code   string
	}{
		{"/profile/me/photos/abc", nil, http.StatusBadRequest, "VALIDATION_ERROR"},
		{"/profile/me/photos/5", model.ErrLastPhoto, http.StatusConflict, "LAST_PHOTO"},
		{"/profile/me/photos/5", model.ErrPhotoNotFound, http.StatusNotFound, "PHOTO_NOT_FOUND"},
	} {
		rec, resp := serve(t, profileRouter(&fakeProfiles{err: tc.err}), httptest.NewRequest(http.MethodDelete, tc.path, nil))
		if rec.Code != tc.status || errCode(resp) != tc.code {
			t.Errorf("%s %v: status = %d, body = %v", tc.path, tc.err, rec.Code, resp)
		}
	}
}

func TestProfileHandler_ReorderPhotos(t *testing.T) {
	put := func(svc *fakeProfiles, body string) (*httptest.ResponseRecorder, map[string]any) {
		req := httptest.NewRequest(http.MethodPut, "/profile/me/photos/order", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		return serve(t, profileRouter(svc), req)
	}

	svc := &fakeProfiles{photos: []model.Photo{{ID: 9, URL: "u9"}, {ID: 7, URL: "u7"}}}
	rec, resp := put(svc, `{"photo_ids": [9, 7]}`)
	want := []any{map[string]any{"id": float64(9), "url": "u9"}, map[string]any{"id": float64(7), "url": "u7"}}
	if rec.Code != http.StatusOK || !reflect.DeepEqual(svc.order, []int64{9, 7}) || !reflect.DeepEqual(resp["photos"], want) {
		t.Fatalf("status = %d, order = %v, body = %v", rec.Code, svc.order, resp)
	}

	for _, body := range []string{`{}`, `{"photo_ids": []}`, `{"photo_ids": [1, 1]}`, `{"photo_ids": [0]}`, `{"photo_ids": [1, 2, 3, 4, 5, 6, 7]}`} {
		svc := &fakeProfiles{}
		rec, resp := put(svc, body)
		if rec.Code != http.StatusBadRequest || svc.order != nil || errFields(resp)["photo_ids"] == nil {
			t.Errorf("%s: status = %d, called = %v, body = %v", body, rec.Code, svc.order != nil, resp)
		}
	}

	rec, resp = put(&fakeProfiles{err: model.ErrPhotoOrderMismatch}, `{"photo_ids": [1, 2]}`)
	if rec.Code != http.StatusBadRequest || errCode(resp) != "VALIDATION_ERROR" || errFields(resp)["photo_ids"] == nil {
		t.Errorf("mismatch: status = %d, body = %v", rec.Code, resp)
	}
}
