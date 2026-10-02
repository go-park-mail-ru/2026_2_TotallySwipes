package model

import "errors"

var (
	ErrNotFound           = errors.New("not found")
	ErrEmailAlreadyExists = errors.New("email already exists")
	ErrPasswordTooLong    = errors.New("password too long")
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrInvalidSession     = errors.New("invalid session")
	// ErrSessionNotOpened означает, что аккаунт создан, но сессия не открыта.
	ErrSessionNotOpened       = errors.New("account created, session not opened")
	ErrActiveTestNotFound     = errors.New("active test not found")
	ErrInvalidFeedRequest     = errors.New("invalid feed request")
	ErrInvalidTestRequest     = errors.New("invalid test request")
	ErrInvalidAnswers         = errors.New("invalid test answers")
	ErrTestNotFound           = errors.New("test not found")
	ErrProfileRequired        = errors.New("profile required")
	ErrInvalidBigFive         = errors.New("invalid Big Five")
	ErrInvalidTestDefinition  = errors.New("invalid test definition")
	ErrInvalidPhotoStorageKey = errors.New("invalid photo storage key")
)
