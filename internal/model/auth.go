package model

import (
	"time"
)

type RegisterInput struct {
	Email    string
	Password string
}

type Tokens struct {
	Access           string
	AccessExpiresAt  time.Time
	Refresh          string
	RefreshExpiresAt time.Time
}

type AuthResult struct {
	UserID  int64
	Missing []string
	Tokens  Tokens
}
