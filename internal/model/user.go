package model

type User struct {
	ID           int64
	Name         string
	Email        string
	PasswordHash string
}

type UserInput struct {
	Name         string
	Email        string
	PasswordHash string
}

// UpdateUserInput - частичное обновление: nil значит "не менять поле".
// Email ожидается уже нормализованным (нижний регистр, без пробелов по краям)
type UpdateUserInput struct {
	Name         *string
	Email        *string
	PasswordHash *string
}
