package dto

import (
	"dating-app/internal/model"
	"dating-app/internal/validate"
	"strings"
)

type AuthResponse struct {
	UserID           int64 `json:"user_id"`
	ProfileCompleted bool  `json:"profile_completed"`
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

	Photos []model.PhotoUpload `json:"-"`
}

// Normalize приводит поля к виду, в котором их проверяют и сохраняют:
// пробелы в имени схлопываются, email обрезается по краям и приводится к нижнему регистру, пароль не трогается
func (r *RegisterRequest) Normalize() {
	r.Name = validate.NormalizeName(r.Name)
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
	addErr(errs, "photos", validate.ValidatePhotos(r.Photos))

	return errs
}

func addErr(errs map[string]string, field string, err error) {
	if err != nil {
		errs[field] = err.Error()
	}
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

type CheckEmailRequest struct {
	Email string `json:"email"`
}

type CheckEmailResponse struct {
	Available bool `json:"available"`
}

func (r *CheckEmailRequest) Normalize() {
	r.Email = normalizeEmail(r.Email)
}

func (r CheckEmailRequest) Validate() map[string]string {
	errs := make(map[string]string)
	addErr(errs, "email", validate.ValidateEmail(r.Email))
	return errs
}

func (r LoginRequest) Validate() map[string]string {
	errs := make(map[string]string)

	addErr(errs, "email", validate.ValidateEmail(r.Email))
	addErr(errs, "password", validate.ValidateLoginPassword(r.Password))

	return errs
}
