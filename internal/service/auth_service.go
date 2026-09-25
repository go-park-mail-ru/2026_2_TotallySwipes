package service

import (
	"context"
	"crypto/sha256"
	"dating-app/internal/auth"
	"dating-app/internal/model"
	"dating-app/internal/repository"
	"dating-app/internal/validate"
	"encoding/hex"
	"errors"
	"fmt"
)

var (
	ErrInvalidCredentials = errors.New("invalid credentials")
)

type AuthService interface {
	LoginWithEmail(ctx context.Context, email string, password string) (*model.User, error)
	Logout(ctx context.Context, refreshToken string) error

	CheckProfileByUserID(ctx context.Context, userId int64) error
}

type AuthServiceImpl struct {
	userRepo    repository.UserRepository
	profileRepo repository.ProfileRepository
	refreshRepo repository.RefreshTokenRepository
}

func (s *AuthServiceImpl) LoginWithEmail(ctx context.Context, email string, password string) (*model.User, error) {

	if err := validate.ValidateEmail(email); err != nil {
		return nil, err
	}

	if err := validate.ValidatePassword(password); err != nil {
		return nil, err
	}

	user, err := s.userRepo.GetUserByEmail(ctx, email)

	if errors.Is(err, repository.ErrNotFound) {
		return nil, ErrInvalidCredentials
	}

	if err != nil {
		return nil, err
	}

	if err := auth.ComparePassword(user.PasswordHash, password); err != nil {
		return nil, err
	}

	return user, nil
}

func (s *AuthService) Logout(ctx context.Context, refreshToken string) error {

	if refreshToken == "" {
		return nil
	}

	sum := sha256.Sum256([]byte(refreshToken))
	tokenHash := hex.EncodeToString(sum[:])

	token, err := s.refreshRepo.GetRefreshToken(ctx, tokenHash)
	if errors.Is(err, repository.ErrNotFound) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("logout: get refresh token: %w", err)
	}

	if token.Revoked {
		return nil
	}

	if err := s.refreshRepo.RevokeRefreshToken(ctx, token.ID); err != nil {
		return fmt.Errorf("logout: revoke refresh token: %w", err)
	}

	return nil
}

// заглушка (добавить метод для репозитория)
func (s *AuthServiceImpl) CheckProfileByUserID(ctx context.Context, userId int64) error {
	return nil
}
