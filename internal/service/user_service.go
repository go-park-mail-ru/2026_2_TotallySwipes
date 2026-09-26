package service

import (
	"context"
	"dating-app/internal/model"
)

type UserRepository interface {
	CreateUser(ctx context.Context, input *model.UserInput) error
	GetUserByEmail(ctx context.Context, email string) (*model.User, error)
	GetUserByID(ctx context.Context, id int64) (*model.User, error)

	UpdateUser(ctx context.Context, id int64, input *model.UpdateUserInput) (*model.User, error)
	DeleteUserByID(ctx context.Context, id int64) error
}
