package model

import "time"

// Session - refresh-сессия авторизации
type Session struct {
	ID        string
	UserID    int64
	TokenHash string
	ExpiresAt time.Time
	Revoked   bool
}
