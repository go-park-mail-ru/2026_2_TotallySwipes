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

type UserService interface {
	CreateUser(ctx context.Context, input *model.UserInput) error

	GetUserByEmail(ctx context.Context, email string) (*model.User, error)
	GetUserByID(ctx context.Context, id int64) (*model.User, error)

	UpdateUser(ctx context.Context, id int64, input *model.UpdateUserInput) (*model.User, error)
	DeleteUserByID(ctx context.Context, id int64) error
}

type UserServiceImpl struct {
	userRepo UserRepository
}

func NewUserService(repo UserRepository) *UserServiceImpl {
	return &UserServiceImpl{userRepo: repo}
}

func (s *UserServiceImpl) CreateUser(ctx context.Context, input *model.UserInput) error {
	return s.userRepo.CreateUser(ctx, input)
}

func (s *UserServiceImpl) GetUserByEmail(ctx context.Context, email string) (*model.User, error) {
	return s.userRepo.GetUserByEmail(ctx, email)
}

func (s *UserServiceImpl) GetUserByID(ctx context.Context, id int64) (*model.User, error) {
	return s.userRepo.GetUserByID(ctx, id)
}

func (s *UserServiceImpl) UpdateUser(ctx context.Context, id int64, input *model.UpdateUserInput) (*model.User, error) {
	return s.userRepo.UpdateUser(ctx, id, input)
}

func (s *UserServiceImpl) DeleteUserByID(ctx context.Context, id int64) error {
	return s.userRepo.DeleteUserByID(ctx, id)
}
