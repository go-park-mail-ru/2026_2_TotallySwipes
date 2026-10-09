package auth

import (
	"errors"

	"golang.org/x/crypto/bcrypt"

	"dating-app/internal/model"
)

// BcryptHasher - хеширование и проверка паролей через bcrypt
type BcryptHasher struct{}

func (BcryptHasher) Hash(plain string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(plain), bcrypt.DefaultCost)
	if errors.Is(err, bcrypt.ErrPasswordTooLong) {
		return "", model.ErrPasswordTooLong
	}
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

// Matches возвращает false без ошибки, если пароль просто не подошёл
func (BcryptHasher) Matches(hash, plain string) (bool, error) {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(plain))
	switch {
	case err == nil:
		return true, nil
	case errors.Is(err, bcrypt.ErrMismatchedHashAndPassword), errors.Is(err, bcrypt.ErrPasswordTooLong):
		return false, nil
	default:
		return false, err
	}
}
