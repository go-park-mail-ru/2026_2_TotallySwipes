package handler

import (
	"strings"
	"unicode/utf8"

	"dating-app/internal/validate"
)

// Как в контракте
const loginPasswordMaxLen = 128

type RegisterRequest struct {
	Name         string   `json:"name"`
	Email        string   `json:"email"`
	Password     string   `json:"password"`
	BirthDate    string   `json:"birth_date"`
	Sex          string   `json:"sex"`
	SearchSex    string   `json:"search_sex"`
	DatingIntent string   `json:"dating_intent"`
	Tags         []string `json:"tags"`
}

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// Normalize приводит поля к виду, в котором их проверяют и сохраняют:
// email обрезается по краям, пароль не трогается
func (r *RegisterRequest) Normalize() {
	r.Email = strings.TrimSpace(r.Email)
}

func (r *LoginRequest) Normalize() {
	r.Email = strings.TrimSpace(r.Email)
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
	addErr(errs, "tags", validate.ValidateTags(r.Tags))

	return errs
}

func (r LoginRequest) Validate() map[string]string {
	errs := make(map[string]string)

	addErr(errs, "email", validate.ValidateEmail(r.Email))

	switch {
	case r.Password == "":
		addErr(errs, "password", validate.ErrRequired)
	case utf8.RuneCountInString(r.Password) > loginPasswordMaxLen:
		addErr(errs, "password", validate.ErrPasswordTooLong)
	}

	return errs
}

func addErr(errs map[string]string, field string, err error) {
	if err != nil {
		errs[field] = err.Error()
	}
}
