package service

import (
	"context"
	"fmt"

	"dating-app/internal/model"
)

type UserRepository interface {
	// GetUserByID находит пользователя по идентификатору.
	// Принимает: контекст ctx и ID пользователя id.
	// Возвращает: пользователя или ошибку, включая model.ErrNotFound.
	GetUserByID(ctx context.Context, id int64) (*model.User, error)
	// GetUserByEmail находит пользователя по email.
	// Принимает: контекст ctx и адрес email.
	// Возвращает: пользователя или ошибку; при отсутствии — model.ErrNotFound.
	GetUserByEmail(ctx context.Context, email string) (*model.User, error)

	// CreateUser передаёт создание пользователя репозиторию.
	// Принимает: контекст ctx и данные пользователя input.
	// Возвращает: nil при успехе или ошибку репозитория.
	CreateUser(ctx context.Context, input *model.UserInput) error
	// UpdateUser изменяет переданные поля пользователя, сохраняя поля с nil без изменений.
	// Принимает: контекст ctx, ID пользователя id и данные input.
	// Возвращает: обновлённого пользователя или ошибку, включая model.ErrNotFound и model.ErrEmailAlreadyExists.
	UpdateUser(ctx context.Context, id int64, input *model.UpdateUserInput) (*model.User, error)
	// DeleteUser удаляет пользователя из хранилища.
	// Принимает: контекст ctx и ID пользователя id.
	// Возвращает: nil при успехе или ошибку, включая model.ErrNotFound.
	DeleteUser(ctx context.Context, id int64) error
}

type UserService interface {
	// CreateUser передаёт создание пользователя репозиторию.
	// Принимает: контекст ctx и данные пользователя input.
	// Возвращает: nil при успехе или ошибку репозитория.
	CreateUser(ctx context.Context, input *model.UserInput) error

	// GetUserByEmail находит пользователя по email.
	// Принимает: контекст ctx и адрес email.
	// Возвращает: пользователя или ошибку; при отсутствии — model.ErrNotFound.
	GetUserByEmail(ctx context.Context, email string) (*model.User, error)
	// GetUserByID находит пользователя по идентификатору.
	// Принимает: контекст ctx и ID пользователя id.
	// Возвращает: пользователя или ошибку, включая model.ErrNotFound.
	GetUserByID(ctx context.Context, id int64) (*model.User, error)

	// UpdateUser изменяет переданные поля пользователя, сохраняя поля с nil без изменений.
	// Принимает: контекст ctx, ID пользователя id и данные input.
	// Возвращает: обновлённого пользователя или ошибку, включая model.ErrNotFound и model.ErrEmailAlreadyExists.
	UpdateUser(ctx context.Context, id int64, input *model.UpdateUserInput) (*model.User, error)
	// DeleteUserByID удаляет пользователя через репозиторий.
	// Принимает: контекст ctx и ID пользователя id.
	// Возвращает: nil при успехе или ошибку, включая model.ErrNotFound.
	DeleteUserByID(ctx context.Context, id int64) error
}

type UserServiceImpl struct {
	userRepo UserRepository
}

// NewUserService создаёт сервис операций с пользователями.
// Принимает: репозиторий пользователей repo.
// Возвращает: экземпляр UserServiceImpl.
func NewUserService(repo UserRepository) *UserServiceImpl {
	return &UserServiceImpl{userRepo: repo}
}

// CreateUser передаёт создание пользователя репозиторию.
// Принимает: контекст ctx и данные пользователя input.
// Возвращает: nil при успехе или ошибку репозитория.
func (s *UserServiceImpl) CreateUser(ctx context.Context, input *model.UserInput) error {
	if err := s.userRepo.CreateUser(ctx, input); err != nil {
		return fmt.Errorf("create user: %w", err)
	}
	return nil
}

// GetUserByEmail находит пользователя по email.
// Принимает: контекст ctx и адрес email.
// Возвращает: пользователя или ошибку; при отсутствии — model.ErrNotFound.
func (s *UserServiceImpl) GetUserByEmail(ctx context.Context, email string) (*model.User, error) {
	user, err := s.userRepo.GetUserByEmail(ctx, email)
	if err != nil {
		return nil, fmt.Errorf("get user by email: %w", err)
	}
	return user, nil
}

// GetUserByID находит пользователя по идентификатору.
// Принимает: контекст ctx и ID пользователя id.
// Возвращает: пользователя или ошибку, включая model.ErrNotFound.
func (s *UserServiceImpl) GetUserByID(ctx context.Context, id int64) (*model.User, error) {
	user, err := s.userRepo.GetUserByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("get user by id: %w", err)
	}
	return user, nil
}

// UpdateUser изменяет переданные поля пользователя, сохраняя поля с nil без изменений.
// Принимает: контекст ctx, ID пользователя id и данные input.
// Возвращает: обновлённого пользователя или ошибку, включая model.ErrNotFound и model.ErrEmailAlreadyExists.
func (s *UserServiceImpl) UpdateUser(ctx context.Context, id int64, input *model.UpdateUserInput) (*model.User, error) {
	user, err := s.userRepo.UpdateUser(ctx, id, input)
	if err != nil {
		return nil, fmt.Errorf("update user: %w", err)
	}
	return user, nil
}

// DeleteUserByID удаляет пользователя через репозиторий.
// Принимает: контекст ctx и ID пользователя id.
// Возвращает: nil при успехе или ошибку, включая model.ErrNotFound.
func (s *UserServiceImpl) DeleteUserByID(ctx context.Context, id int64) error {
	if err := s.userRepo.DeleteUser(ctx, id); err != nil {
		return fmt.Errorf("delete user: %w", err)
	}
	return nil
}
