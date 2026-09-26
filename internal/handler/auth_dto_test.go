package handler

import (
	"reflect"
	"sort"
	"testing"
)

func validRegister() RegisterRequest {
	return RegisterRequest{
		Name:         "alex",
		Email:        "alex@example.com",
		Password:     "qwerty123",
		BirthDate:    "2000-01-01",
		Sex:          "male",
		SearchSex:    "female",
		DatingIntent: "Ищу половинку",
		Tags:         []string{"sport"},
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

	want := []string{"birth_date", "dating_intent", "email", "name", "password", "search_sex", "sex"}
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
