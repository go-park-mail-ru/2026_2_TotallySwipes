package service

import (
	"context"
	"fmt"
	"log/slog"

	"dating-app/services/matching/internal/model"
)

type FeedRepository interface {
	GetSwipeStates(ctx context.Context, userID, profileID int64, candidates []model.Profile) (map[int64]model.SwipeState, error)
}

type FeedService struct {
	profiles      ProfileProvider
	swipes        FeedRepository
	compatibility CompatibilityService
}

func NewFeedService(profiles ProfileProvider, swipes FeedRepository, compatibility CompatibilityService) *FeedService {

	return &FeedService{profiles: profiles, swipes: swipes, compatibility: compatibility}
}

func (s *FeedService) GetNextFeed(ctx context.Context, userID int64, limit int, cursor *int64) (*model.FeedPage, error) {

	if userID <= 0 || limit < 1 || limit > 10 || cursor != nil && *cursor <= 0 {
		return nil, model.ErrInvalidFeedRequest
	}

	profiles, err := s.profiles.GetProfilesBatch(ctx, []int64{userID}, model.MatchFilter{})

	if err != nil {
		return nil, fmt.Errorf("get feed viewer: %w", err)
	}

	viewer, exists := profiles[userID]

	if !exists || len(viewer.Missing) > 0 {
		return nil, model.ErrProfileRequired
	}

	if viewer.City == nil {
		return nil, model.ErrLocationRequired
	}

	if viewer.ID <= 0 {
		return nil, fmt.Errorf("proto: missing viewer profile_id")
	}

	page := &model.FeedPage{Items: make([]model.FeedItem, 0, limit)}
	next := cursor
	seen := make(map[int64]bool)

	for batchNumber := 0; batchNumber < maxSelectionBatches; batchNumber++ {
		candidates, batchNext, err := s.profiles.SearchCandidates(ctx, userID, limit-len(page.Items), next)

		if err != nil {
			return nil, fmt.Errorf("get feed candidates: %w", err)
		}

		states, err := s.swipes.GetSwipeStates(ctx, userID, viewer.ID, candidates)

		if err != nil {
			return nil, fmt.Errorf("get feed swipes: %w", err)
		}

		for _, candidate := range candidates {
			if candidate.UserID == userID || seen[candidate.UserID] || states[candidate.UserID].Swiped {
				continue
			}

			seen[candidate.UserID] = true

			if len(candidate.Missing) > 0 {
				slog.WarnContext(ctx, "feed: incomplete candidate returned by Profile", "user_id", candidate.UserID)
				continue
			}

			item, err := feedItem(candidate, viewer.Psycho, s.compatibility)

			if err != nil {
				return nil, err
			}

			item.LikedMe = states[candidate.UserID].LikedMe
			page.Items = append(page.Items, item)
		}

		page.NextCursor = batchNext

		if batchNext == nil || len(page.Items) == limit {
			return page, nil
		}

		if next != nil && *batchNext == *next {
			return nil, fmt.Errorf("feed cursor did not advance")
		}

		next = batchNext
	}

	return page, nil
}

func feedItem(profile model.Profile, viewerVector model.BigFive, compatibility CompatibilityService) (model.FeedItem, error) {

	age := profileAge(profile)

	if profile.Name == nil || profile.DatingGoal == nil || age == nil {
		return model.FeedItem{}, fmt.Errorf("proto: missing required candidate fields")
	}

	item := model.FeedItem{UserID: profile.UserID, Name: *profile.Name, Age: *age, DatingGoal: *profile.DatingGoal,
		City: profile.City, AboutMe: profile.AboutMe, Education: profile.Education, Work: profile.Work,
		Smoking: profile.Smoking, Alcohol: profile.Alcohol, Height: profile.Height,
		Tags: append([]string{}, profile.Tags...), Photos: append([]model.FeedPhoto{}, profile.Photos...),
	}

	if isCompleteBigFive(viewerVector) && isCompleteBigFive(profile.Psycho) {
		value, err := compatibility.CalculateDistance(viewerVector, profile.Psycho)

		if err != nil {
			return model.FeedItem{}, fmt.Errorf("compatibility user id=%d: %w", profile.UserID, err)
		}

		item.Compatibility = &value
	}

	return item, nil
}
