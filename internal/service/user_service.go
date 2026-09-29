package service

import (
	"context"
	"fmt"

	"dating-app/internal/model"
)

type UserRepository interface {
	GetUserByID(ctx context.Context, id int64) (*model.User, error)
	GetUserByEmail(ctx context.Context, email string) (*model.User, error)

	CreateUser(ctx context.Context, input *model.UserInput) error
	UpdateUser(ctx context.Context, id int64, input *model.UpdateUserInput) (*model.User, error)
	DeleteUser(ctx context.Context, id int64) error
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
	if err := s.userRepo.CreateUser(ctx, input); err != nil {
		return fmt.Errorf("create user: %w", err)
	}
	return nil
}

func (s *UserServiceImpl) GetUserByEmail(ctx context.Context, email string) (*model.User, error) {
	user, err := s.userRepo.GetUserByEmail(ctx, email)
	if err != nil {
		return nil, fmt.Errorf("get user by email: %w", err)
	}
	return user, nil
}

func (s *UserServiceImpl) GetUserByID(ctx context.Context, id int64) (*model.User, error) {
	user, err := s.userRepo.GetUserByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("get user by id: %w", err)
	}
	return user, nil
}

func (s *UserServiceImpl) UpdateUser(ctx context.Context, id int64, input *model.UpdateUserInput) (*model.User, error) {
	user, err := s.userRepo.UpdateUser(ctx, id, input)
	if err != nil {
		return nil, fmt.Errorf("update user: %w", err)
	}
	return user, nil
}

func (s *UserServiceImpl) DeleteUserByID(ctx context.Context, id int64) error {
	if err := s.userRepo.DeleteUser(ctx, id); err != nil {
		return fmt.Errorf("delete user: %w", err)
	}
	return nil
}
