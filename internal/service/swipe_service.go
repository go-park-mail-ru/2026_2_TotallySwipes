package service

import (
	"context"
	"dating-app/internal/model"
	"errors"
	"fmt"
)

type SwipeRepository interface {
	SaveSwipeAndMatch(ctx context.Context, swipe *model.Swipe) error
}

type SwipeService struct {
	swipeRepo   SwipeRepository
	profileRepo ProfileRepository
}

func NewSwipeService(swipeRepo SwipeRepository, profileRepo ProfileRepository) *SwipeService {
	return &SwipeService{swipeRepo: swipeRepo, profileRepo: profileRepo}
}

func (s *SwipeService) Swipe(ctx context.Context, swipe *model.Swipe) error {

	if swipe.TargetID == swipe.AuthorID {
		return fmt.Errorf("swipe: self swipe userID=%d: %w", swipe.AuthorID, model.ErrInvalidSwipe)
	}

	profile, err := s.profileRepo.GetByUserIDCurrentProfile(ctx, swipe.AuthorID)

	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			return fmt.Errorf("swipe: author profile: %w", model.ErrProfileIncomplete)
		}
		return fmt.Errorf("swipe: get author profile: %w", err)
	}

	if len(profile.Missing()) > 0 {
		return fmt.Errorf("swipe and match :%w", model.ErrProfileIncomplete)
	}

	if err := s.swipeRepo.SaveSwipeAndMatch(ctx, swipe); err != nil {
		if errors.Is(err, model.ErrNotFound) {
			return fmt.Errorf("swipe: target profile: %w", model.ErrProfileNotFound)
		}
		return fmt.Errorf("swipe: save swipe and match: %w", err)
	}

	return nil
}
