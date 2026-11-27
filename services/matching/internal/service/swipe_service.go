package service

import (
	"context"
	"fmt"

	"dating-app/services/matching/internal/model"
)

type SwipeRepository interface {
	SaveSwipeAndMatch(ctx context.Context, swipe *model.Swipe, authorProfileID, targetProfileID int64) (int64, error)
}

type SwipeService struct {
	repo     SwipeRepository
	profiles ProfileProvider
	chat     ChatProvider
}

func NewSwipeService(repo SwipeRepository, profiles ProfileProvider, chat ChatProvider) *SwipeService {

	return &SwipeService{repo: repo, profiles: profiles, chat: chat}
}

func (s *SwipeService) Swipe(ctx context.Context, swipe *model.Swipe) error {

	if swipe == nil || swipe.AuthorID <= 0 || swipe.TargetID <= 0 || swipe.AuthorID == swipe.TargetID {
		return model.ErrInvalidSwipe
	}

	if swipe.Action != model.SwipeLike && swipe.Action != model.SwipeDislike && swipe.Action != model.SwipeSuperLike {
		return model.ErrInvalidSwipe
	}

	profiles, err := s.profiles.GetProfilesBatch(ctx, []int64{swipe.AuthorID, swipe.TargetID}, model.MatchFilter{})

	if err != nil {
		return fmt.Errorf("swipe profiles: %w", err)
	}

	author, exists := profiles[swipe.AuthorID]

	if !exists || len(author.Missing) > 0 {
		return model.ErrProfileIncomplete
	}

	target, exists := profiles[swipe.TargetID]

	if !exists {
		return model.ErrProfileNotFound
	}

	if author.ID <= 0 || target.ID <= 0 {
		return fmt.Errorf("proto: missing profile_id for swipe")
	}

	matchID, err := s.repo.SaveSwipeAndMatch(ctx, swipe, author.ID, target.ID)

	if err != nil {
		return err
	}

	if matchID == 0 {
		return nil
	}

	_, _, err = s.chat.EnsureChat(ctx, matchID, swipe.AuthorID, swipe.TargetID)

	if err != nil {
		return fmt.Errorf("ensure chat for match %d: %w", matchID, err)
	}

	return nil
}
