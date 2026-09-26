package model

import "errors"

var (
	ErrNotFound           = errors.New("not found")
	ErrEmailAlreadyExists = errors.New("email already exists")
	ErrPasswordTooLong    = errors.New("password too long")
)
