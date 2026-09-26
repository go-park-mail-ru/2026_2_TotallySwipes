package auth

import "golang.org/x/crypto/bcrypt"

func HashPassword(plain string) (string, error) {

	bytes := []byte(plain)
	hash, err := bcrypt.GenerateFromPassword(bytes, bcrypt.DefaultCost)

	if err != nil {
		return "", err
	}

	return string(hash), nil
}

func ComparePassword(hash, plain string) error {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(plain))
}
