package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sort"
	"strings"
	"testing"

	. "dating-app/internal/handler"
	"dating-app/internal/model"

	"github.com/gorilla/mux"
)

type fakeFilters struct {
	filter model.SearchFilter
	set    *model.SearchFilter
	err    error
}

func (f *fakeFilters) Get(context.Context, int64) (*model.SearchFilter, error) {
	return &f.filter, f.err
}
func (f *fakeFilters) Set(_ context.Context, _ int64, filter model.SearchFilter) (*model.SearchFilter, error) {
	f.set = &filter
	return &filter, f.err
}

func filterRouter(svc *fakeFilters) http.Handler {
	h := NewFilterHandler(svc)
	withUser := func(fn UserHandlerFunc) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fn(w, r, 7) })
	}
	r := mux.NewRouter()
	r.Handle("/filters/me", withUser(h.Get)).Methods(http.MethodGet)
	r.Handle("/filters/me", withUser(h.Set)).Methods(http.MethodPut)
	return r
}

func putFilter(t *testing.T, svc *fakeFilters, body string) (*httptest.ResponseRecorder, map[string]any) {
	t.Helper()
	req := httptest.NewRequest(http.MethodPut, "/filters/me", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	return serve(t, filterRouter(svc), req)
}

func TestFilterHandler_Get(t *testing.T) {
	svc := &fakeFilters{filter: model.DefaultSearchFilter}
	rec, resp := serve(t, filterRouter(svc), httptest.NewRequest(http.MethodGet, "/filters/me", nil))
	want := map[string]any{"sex": "all", "age_from": float64(18), "age_to": float64(100)}
	if rec.Code != http.StatusOK || !reflect.DeepEqual(resp, want) {
		t.Fatalf("status = %d, body = %v", rec.Code, resp)
	}
}

func TestFilterHandler_Set(t *testing.T) {
	svc := &fakeFilters{}
	rec, resp := putFilter(t, svc, `{"sex": "female", "age_from": 20, "age_to": 30}`)
	want := model.SearchFilter{Sex: model.SearchSexFemale, AgeFrom: 20, AgeTo: 30}
	if rec.Code != http.StatusOK || svc.set == nil || *svc.set != want || resp["sex"] != "female" {
		t.Fatalf("status = %d, set = %+v, body = %v", rec.Code, svc.set, resp)
	}
}

func TestFilterHandler_SetValidation(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		fields     []string
	}{
		{"пустой фильтр", `{}`, []string{"age_from", "age_to", "sex"}},
		{"неизвестный пол", `{"sex": "x", "age_from": 18, "age_to": 30}`, []string{"sex"}},
		{"младше 18", `{"sex": "all", "age_from": 17, "age_to": 30}`, []string{"age_from"}},
		{"старше 100", `{"sex": "all", "age_from": 18, "age_to": 101}`, []string{"age_to"}},
		{"перепутаны границы", `{"sex": "all", "age_from": 40, "age_to": 30}`, []string{"age_to"}},
	} {
		svc := &fakeFilters{}
		rec, resp := putFilter(t, svc, tc.body)
		if rec.Code != http.StatusBadRequest || svc.set != nil {
			t.Errorf("%s: status = %d, called = %v", tc.name, rec.Code, svc.set != nil)
			continue
		}
		var got []string
		for k := range errFields(resp) {
			got = append(got, k)
		}
		sort.Strings(got)
		if !reflect.DeepEqual(got, tc.fields) {
			t.Errorf("%s: fields = %v, want %v", tc.name, got, tc.fields)
		}
	}
}
