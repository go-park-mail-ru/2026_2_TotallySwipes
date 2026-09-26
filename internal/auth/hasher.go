package auth

import (
	"errors"

	"golang.org/x/crypto/bcrypt"

	"dating-app/internal/model"
)

// BcryptHasher - обёртка над HashPassword/ComparePassword
type BcryptHasher struct{}

func (BcryptHasher) Hash(plain string) (string, error) {
	hash, err := HashPassword(plain)
	if errors.Is(err, bcrypt.ErrPasswordTooLong) {
		return "", model.ErrPasswordTooLong
	}
	return hash, err
}

// Matches возвращает false без ошибки, если пароль просто не подошёл
func (BcryptHasher) Matches(hash, plain string) (bool, error) {
	err := ComparePassword(hash, plain)
	switch {
	case err == nil:
		return true, nil
	case errors.Is(err, bcrypt.ErrMismatchedHashAndPassword), errors.Is(err, bcrypt.ErrPasswordTooLong):
		return false, nil
	default:
		return false, err
	}
}
