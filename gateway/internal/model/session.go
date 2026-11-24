package model

import "time"

type Session struct {
	ID        string
	UserID    int64
	TokenHash string
	ExpiresAt time.Time
	Revoked   bool
}
