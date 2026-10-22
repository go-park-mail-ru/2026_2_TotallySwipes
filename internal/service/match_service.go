package service

import (
	"context"
	"dating-app/internal/model"
	"errors"
	"fmt"
	"time"
)

type MatchRepository interface {
	GetMatches(ctx context.Context, userID int64, limit int, cursor *int64) ([]model.Match, int, *int64, error)
	MarkSeen(ctx context.Context, userID, matchID int64) error
	GetSummary(ctx context.Context, userID int64) (*model.MatchSummary, error)
}

type MatchViewerRepository interface {
	GetByUserIDCurrentProfile(ctx context.Context, userID int64) (*model.Profile, error)
}

type MatchService struct {
	repo             MatchRepository
	profileRepo      MatchViewerRepository
	compatibilitySvc CompatibilityService
	media            URLProvider
}

func NewMatchService(repo MatchRepository, profileRepo MatchViewerRepository, compatibilitySvc CompatibilityService, media URLProvider) *MatchService {
	return &MatchService{repo: repo, profileRepo: profileRepo, compatibilitySvc: compatibilitySvc, media: media}
}

// GetMatches формирует страницу мэтчей с возрастом, главным фото и совместимостью при наличии обоих тестов.
func (s *MatchService) GetMatches(ctx context.Context, userID int64, limit int, cursor *int64) (*model.MatchPage, error) {
	if userID <= 0 || limit < 1 || limit > model.MatchesMaxLimit || (cursor != nil && *cursor <= 0) {
		return nil, fmt.Errorf("%w: invalid user ID, limit or cursor", model.ErrInvalidMatchesRequest)
	}

	viewer, err := s.profileRepo.GetByUserIDCurrentProfile(ctx, userID)
	if err != nil && !errors.Is(err, model.ErrNotFound) {
		return nil, fmt.Errorf("get matches viewer profile: %w", err)
	}
	var viewerVector *model.BigFive
	if viewer != nil {
		viewerVector, _ = feedBigFive(viewer.CurrentPsycho)
	}

	matches, total, nextCursor, err := s.repo.GetMatches(ctx, userID, limit, cursor)
	if err != nil {
		return nil, fmt.Errorf("get matches user id=%d: %w", userID, err)
	}

	now := time.Now()
	for i := range matches {
		m := &matches[i]
		if m.BirthDate != nil {
			age := model.AgeAt(*m.BirthDate, now)
			m.Age = &age
		}
		if m.PhotoKey != nil {
			url, err := s.media.GetURL(ctx, *m.PhotoKey)
			if err != nil {
				return nil, fmt.Errorf("get matches photo match id=%d: %w", m.ID, err)
			}
			m.MainPhotoURL = &url
		}
		if viewerVector != nil && isCompleteBigFive(m.Psycho) {
			compatibility, err := s.compatibilitySvc.CalculateDistance(*viewerVector, m.Psycho)
			if err != nil {
				return nil, fmt.Errorf("get matches compatibility match id=%d: %w", m.ID, err)
			}
			m.Compatibility = &compatibility
		}
	}

	return &model.MatchPage{Items: matches, Total: total, NextCursor: nextCursor}, nil
}

// MarkSeen снимает с мэтча признак новизны для пользователя; model.ErrMatchNotFound, если он не участник пары
func (s *MatchService) MarkSeen(ctx context.Context, userID, matchID int64) error {
	if userID <= 0 || matchID <= 0 {
		return fmt.Errorf("%w: invalid user ID or match ID", model.ErrInvalidMatchesRequest)
	}
	if err := s.repo.MarkSeen(ctx, userID, matchID); err != nil {
		if errors.Is(err, model.ErrNotFound) {
			return fmt.Errorf("mark match id=%d seen: %w", matchID, model.ErrMatchNotFound)
		}
		return fmt.Errorf("mark match id=%d seen: %w", matchID, err)
	}
	return nil
}

// GetSummary возвращает число мэтчей пользователя и сколько из них он ещё не открывал
func (s *MatchService) GetSummary(ctx context.Context, userID int64) (*model.MatchSummary, error) {
	if userID <= 0 {
		return nil, fmt.Errorf("%w: invalid user ID", model.ErrInvalidMatchesRequest)
	}
	summary, err := s.repo.GetSummary(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("get matches summary: %w", err)
	}
	return summary, nil
}

func isCompleteBigFive(v model.BigFive) bool {
	return v.Openness != nil && v.Conscientiousness != nil && v.Extraversion != nil &&
		v.Agreeableness != nil && v.Neuroticism != nil
}
