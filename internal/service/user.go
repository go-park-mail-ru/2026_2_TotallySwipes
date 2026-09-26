package service

import (
	"context"

	"dating-app/internal/model"
)

type UserRepository interface {
	GetUserByID(ctx context.Context, id int64) (*model.User, error)
	GetUserByEmail(ctx context.Context, email string) (*model.User, error)

	CreateUser(ctx context.Context, input *model.UserInput) error
	UpdateUser(ctx context.Context, id int64, input *model.UpdateUserInput) (*model.User, error)
	DeleteUser(ctx context.Context, id int64) error
}
