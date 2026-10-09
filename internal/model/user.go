package model

type User struct {
	ID           int64
	Email        string
	PasswordHash string
}

type UserInput struct {
	Email        string
	PasswordHash string
}

type UpdateUserInput struct {
	Email        *string
	PasswordHash *string
}
