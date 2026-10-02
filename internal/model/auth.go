package model

import (
	"time"
)

type RegisterInput struct {
	Name          string
	Email         string
	Password      string
	BirthDate     time.Time
	Sex           Sex
	SearchSex     SearchSex
	DatingGoal    DatingGoal
	AboutMe       string
	SearchAgeFrom int
	SearchAgeTo   int
	Tags          []string
	Photos        []PhotoUpload
}

// Tokens - пара токенов новой сессии
type Tokens struct {
	Access           string
	AccessExpiresAt  time.Time
	Refresh          string
	RefreshExpiresAt time.Time
}

type AuthResult struct {
	UserID           int64
	ProfileCompleted bool
	Tokens           Tokens
}
