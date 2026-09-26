package model

import "time"

type User struct {
	ID           int64
	Name         string
	Email        string
	PasswordHash string
	BirthDate    time.Time
}

type UserInput struct {
	Name         string
	Email        string
	PasswordHash string
	BirthDate    time.Time
}

type UpdateUserInput struct {
	Name         string
	Email        string
	PasswordHash string
	BirthDate    time.Time
}
