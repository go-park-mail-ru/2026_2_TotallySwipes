package service

import (
	"context"
	"errors"
	"fmt"

	"dating-app/internal/model"
)

type FilterRepository interface {
	// Get возвращает фильтр пользователя или model.ErrNotFound.
	Get(ctx context.Context, userID int64) (*model.SearchFilter, error)
	Upsert(ctx context.Context, userID int64, f *model.SearchFilter) error
}

type FilterService struct {
	repo FilterRepository
}

func NewFilterService(repo FilterRepository) *FilterService {
	return &FilterService{repo: repo}
}

// Get возвращает фильтр пользователя или model.DefaultSearchFilter, если он не задан
func (s *FilterService) Get(ctx context.Context, userID int64) (*model.SearchFilter, error) {
	f, err := s.repo.Get(ctx, userID)
	if errors.Is(err, model.ErrNotFound) {
		def := model.DefaultSearchFilter
		return &def, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get filter user id=%d: %w", userID, err)
	}
	return f, nil
}

// Set заменяет фильтр пользователя целиком
func (s *FilterService) Set(ctx context.Context, userID int64, f model.SearchFilter) (*model.SearchFilter, error) {
	if err := s.repo.Upsert(ctx, userID, &f); err != nil {
		return nil, fmt.Errorf("set filter user id=%d: %w", userID, err)
	}
	return &f, nil
}
