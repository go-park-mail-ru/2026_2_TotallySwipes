package dto

import (
	"dating-app/internal/model"
	"dating-app/internal/validate"
	"strings"
)

// AuthResponse - ответ логина и регистрации. Missing - обязательные поля анкеты,
// которых не хватает; если список не пуст, клиент ведёт пользователя на онбординг
type AuthResponse struct {
	UserID  int64    `json:"user_id"`
	Missing []string `json:"missing"`
}

func NewAuthResponse(res model.AuthResult) AuthResponse {
	missing := res.Missing
	if missing == nil {
		missing = []string{}
	}
	return AuthResponse{UserID: res.UserID, Missing: missing}
}

type RegisterRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// Normalize обрезает email и приводит его к нижнему регистру, пароль не трогает
func (r *RegisterRequest) Normalize() {
	r.Email = normalizeEmail(r.Email)
}

// Validate собирает ошибки по всем полям сразу. Вызывать после Normalize
func (r RegisterRequest) Validate() map[string]string {
	errs := make(map[string]string)
	addErr(errs, "email", validate.ValidateEmail(r.Email))
	addErr(errs, "password", validate.ValidatePassword(r.Password))
	return errs
}

func (r RegisterRequest) ToModel() model.RegisterInput {
	return model.RegisterInput{Email: r.Email, Password: r.Password}
}

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (r *LoginRequest) Normalize() {
	r.Email = normalizeEmail(r.Email)
}

func (r LoginRequest) Validate() map[string]string {
	errs := make(map[string]string)
	addErr(errs, "email", validate.ValidateEmail(r.Email))
	addErr(errs, "password", validate.ValidateLoginPassword(r.Password))
	return errs
}

// normalizeEmail приводит строку к lower case и тримит ее
func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

func addErr(errs map[string]string, field string, err error) {
	if err != nil {
		errs[field] = err.Error()
	}
}
