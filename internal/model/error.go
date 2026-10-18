package model

import "errors"

var (
	ErrNotFound           = errors.New("not found")
	ErrEmailAlreadyExists = errors.New("email already exists")
	ErrPasswordTooLong    = errors.New("password too long")
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrInvalidSession     = errors.New("invalid session")
	ErrInvalidSwipe       = errors.New("invalid self-swipe")
	ErrProfileIncomplete  = errors.New("profile incomplete")
	// ErrSessionNotOpened означает, что аккаунт создан, но сессия не открыта.
	ErrSessionNotOpened       = errors.New("account created, session not opened")
	ErrInvalidFeedRequest     = errors.New("invalid feed request")
	ErrInvalidMatchesRequest  = errors.New("invalid matches request")
	ErrInvalidTestRequest     = errors.New("invalid test request")
	ErrInvalidAnswers         = errors.New("invalid test answers")
	ErrTestNotFound           = errors.New("test not found")
	ErrProfileNotFound        = errors.New("profile not found")
	ErrTestResultNotFound     = errors.New("test result not found")
	ErrProfileRequired        = errors.New("profile required")
	ErrInvalidBigFive         = errors.New("invalid Big Five")
	ErrInvalidTestDefinition  = errors.New("invalid test definition")
	ErrInvalidPhotoStorageKey = errors.New("invalid photo storage key")
	ErrUnknownTag             = errors.New("unknown tag")
	ErrPhotoLimit             = errors.New("photo limit reached")
	// ErrLastPhoto - нельзя удалить единственное фото заполненной анкеты
	ErrLastPhoto     = errors.New("last photo of completed profile")
	ErrPhotoNotFound = errors.New("photo not found")
	ErrMatchNotFound = errors.New("match not found")
	// ErrPhotoOrderMismatch - новый порядок не совпадает с текущим набором фото анкеты
	ErrPhotoOrderMismatch = errors.New("photo order mismatch")
)
