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

// UpdateUserInput - частичное обновление: nil значит "не менять поле".
// Email ожидается уже нормализованным (нижний регистр, без пробелов по краям)
type UpdateUserInput struct {
	Name         *string
	Email        *string
	PasswordHash *string
	BirthDate    *time.Time
}
