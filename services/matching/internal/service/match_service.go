package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"dating-app/services/matching/internal/model"
)

const maxSelectionBatches = 32

type ChatProvider interface {
	EnsureChat(ctx context.Context, matchID, firstUserID, secondUserID int64) (chatID int64, created bool, err error)
}

type ProfileProvider interface {
	GetProfilesBatch(ctx context.Context, ids []int64, filter model.MatchFilter) (map[int64]model.Profile, error)
	SearchCandidates(ctx context.Context, userID int64, limit int, cursor *int64) ([]model.Profile, *int64, error)
}

type MatchRepository interface {
	GetMatches(ctx context.Context, userID int64, limit int, cursor *int64) ([]model.Match, int, *int64, error)
	GetSummary(ctx context.Context, userID int64) (*model.MatchSummary, error)
	MarkSeen(ctx context.Context, userID, matchID int64) error
	GetMatchID(ctx context.Context, viewerID, targetID int64) (int64, error)
}

type MatchSearchRepository interface {
	GetPartnerFinishedByMatchIDs(ctx context.Context, userID int64, matchIDs []int64) (map[int64]bool, error)
}

type MatchService struct {
	repo          MatchRepository
	profiles      ProfileProvider
	compatibility CompatibilityService
	events        MatchSearchRepository
}

func NewMatchService(repo MatchRepository, profiles ProfileProvider, compatibility CompatibilityService, events MatchSearchRepository) *MatchService {

	return &MatchService{repo: repo, profiles: profiles, compatibility: compatibility, events: events}
}

func (s *MatchService) GetMatches(ctx context.Context, userID int64, limit int, cursor *int64, filter model.MatchFilter) (*model.MatchPage, error) {

	if userID <= 0 || limit < 1 || limit > model.MatchesMaxLimit || cursor != nil && *cursor <= 0 {
		return nil, model.ErrInvalidMatchesRequest
	}

	if filter.AgeFrom != nil && (*filter.AgeFrom < 18 || *filter.AgeFrom > 100) ||
		filter.AgeTo != nil && (*filter.AgeTo < 18 || *filter.AgeTo > 100) ||
		filter.AgeFrom != nil && filter.AgeTo != nil && *filter.AgeFrom > *filter.AgeTo ||
		filter.Sex != nil && *filter.Sex != "all" && *filter.Sex != "male" && *filter.Sex != "female" {
		return nil, model.ErrInvalidMatchesRequest
	}

	viewer, err := s.profiles.GetProfilesBatch(ctx, []int64{userID}, model.MatchFilter{})

	if err != nil {
		return nil, fmt.Errorf("get matches viewer: %w", err)
	}

	page := &model.MatchPage{Items: make([]model.Match, 0, limit)}
	next := cursor

	for batchNumber := 0; batchNumber < maxSelectionBatches; batchNumber++ {
		matches, total, batchNext, err := s.repo.GetMatches(ctx, userID, limit-len(page.Items), next)

		if err != nil {
			return nil, fmt.Errorf("get matches: %w", err)
		}

		page.Total = total
		ids := make([]int64, 0, len(matches))
		matchIDs := make([]int64, 0, len(matches))

		for _, match := range matches {
			ids = append(ids, match.UserID)
			matchIDs = append(matchIDs, match.ID)
		}

		if len(matches) > 0 {
			profiles, err := s.profiles.GetProfilesBatch(ctx, ids, filter)

			if err != nil {
				return nil, fmt.Errorf("get matches profiles: %w", err)
			}

			finished, err := s.events.GetPartnerFinishedByMatchIDs(ctx, userID, matchIDs)

			if err != nil {
				return nil, fmt.Errorf("get matches event statuses: %w", err)
			}

			for _, match := range matches {
				profile, exists := profiles[match.UserID]

				if !exists {
					continue
				}

				match.Name, match.BirthDate, match.Psycho = profile.Name, profile.BirthDate, profile.Psycho
				match.Age = profileAge(profile)

				if len(profile.Photos) > 0 {
					photoURL := profile.Photos[0].URL
					match.MainPhotoURL = &photoURL
				}

				match.CanMeetup = sameCity(viewer[userID].City, profile.City)
				match.PartnerFinished = finished[match.ID]

				if isCompleteBigFive(viewer[userID].Psycho) && isCompleteBigFive(profile.Psycho) {
					value, err := s.compatibility.CalculateDistance(viewer[userID].Psycho, profile.Psycho)

					if err != nil {
						return nil, fmt.Errorf("get matches compatibility: %w", err)
					}

					match.Compatibility = &value
				}

				page.Items = append(page.Items, match)
			}
		}

		page.NextCursor = batchNext

		if batchNext == nil || len(page.Items) == limit {
			return page, nil
		}

		if next != nil && *batchNext >= *next {
			return nil, fmt.Errorf("matches cursor did not advance")
		}

		next = batchNext
	}

	return page, nil
}

func (s *MatchService) MarkSeen(ctx context.Context, userID, matchID int64) error {

	if userID <= 0 || matchID <= 0 {
		return model.ErrInvalidMatchesRequest
	}

	if err := s.repo.MarkSeen(ctx, userID, matchID); err != nil {
		if errors.Is(err, model.ErrNotFound) {
			return fmt.Errorf("mark match seen: %w", model.ErrMatchNotFound)
		}

		return fmt.Errorf("mark match seen: %w", err)
	}

	return nil
}

func (s *MatchService) GetSummary(ctx context.Context, userID int64) (*model.MatchSummary, error) {

	if userID <= 0 {
		return nil, model.ErrInvalidMatchesRequest
	}

	summary, err := s.repo.GetSummary(ctx, userID)

	if err != nil {
		return nil, fmt.Errorf("get matches summary: %w", err)
	}

	return summary, nil
}

func (s *MatchService) GetMatchedProfile(ctx context.Context, viewerID, targetID int64) (*model.MatchedProfile, error) {

	if viewerID <= 0 || targetID <= 0 || viewerID == targetID {
		return nil, model.ErrProfileNotFound
	}

	matchID, err := s.repo.GetMatchID(ctx, viewerID, targetID)

	if errors.Is(err, model.ErrNotFound) {
		return nil, model.ErrProfileNotFound
	}

	if err != nil {
		return nil, err
	}

	profiles, err := s.profiles.GetProfilesBatch(ctx, []int64{viewerID, targetID}, model.MatchFilter{})

	if err != nil {
		return nil, err
	}

	profile, exists := profiles[targetID]

	if !exists || len(profile.Missing) > 0 {
		return nil, model.ErrProfileNotFound
	}

	item, err := feedItem(profile, profiles[viewerID].Psycho, s.compatibility)

	if err != nil {
		return nil, err
	}

	item.LikedMe = true
	return &model.MatchedProfile{MatchID: matchID, Item: item}, nil
}

func isCompleteBigFive(v model.BigFive) bool {

	return v.Openness != nil && v.Conscientiousness != nil && v.Extraversion != nil && v.Agreeableness != nil && v.Neuroticism != nil
}

func profileAge(profile model.Profile) *int {

	if profile.BirthDate != nil {
		value := model.AgeAt(*profile.BirthDate, time.Now())
		return &value
	}

	return profile.Age
}

func sameCity(first, second *string) bool {

	return first != nil && second != nil && strings.TrimSpace(*first) != "" &&
		strings.EqualFold(strings.TrimSpace(*first), strings.TrimSpace(*second))
}
