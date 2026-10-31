package service

import (
	"context"
	"dating-app/internal/model"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

type EventProvider interface {
	GetEvents(ctx context.Context, eventQuery *model.EventQuery) (*model.EventPage, error)
}

type MatchRepository interface {
	GetParticipants(ctx context.Context, userID, matchID int64) ([2]int64, error)
}

type EventRepository interface {
	GetActive(ctx context.Context, matchID int64) (*model.PlaceSearch, error)
	GetBySearchID(ctx context.Context, searchID uuid.UUID) (*model.PlaceSearch, error)
	SubmitResults(ctx context.Context, userID, matchID int64, searchID uuid.UUID, likedEvents []int64) (*model.PlaceSearch, error)
	CreateSearch(ctx context.Context, state *model.PlaceSearch) (actual *model.PlaceSearch, created bool, err error)
}

type EventService struct {
	provider  EventProvider
	matchRepo MatchRepository
	eventRepo EventRepository
}

func NewEvenService(provider EventProvider, mathRepo MatchRepository, eventRepo EventRepository) *EventService {
	return &EventService{provider: provider, matchRepo: mathRepo, eventRepo: eventRepo}
}

func NewEventService(provider EventProvider, matchRepo MatchRepository, eventRepo EventRepository) *EventService {
	return &EventService{provider: provider, matchRepo: matchRepo, eventRepo: eventRepo}
}

func (s *EventService) SearchEventsByCity(ctx context.Context, userID int64, matchID int64, city string) (*model.SearchEventsState, bool, error) {

	city = strings.TrimSpace(city)
	if city == "" {
		return nil, false, model.ErrInvalidCity
	}

	participants, err := s.matchRepo.GetParticipants(ctx, userID, matchID)
	if err != nil {
		return nil, false, fmt.Errorf("get participants: %w", err)

	}

	activeSearch, err := s.eventRepo.GetActive(ctx, matchID)

	if err != nil {
		return nil, false, fmt.Errorf("get active: %w", err)
	}

	if activeSearch != nil {

		state, err := toSearchEventsState(activeSearch, userID)

		if err != nil {
			return nil, false, fmt.Errorf("to search events state: %w", err)
		}

		if state.City != city {
			return nil, false, model.ErrSearchAlreadyActive
		}

		return state, false, nil
	}

	now := time.Now()
	categoryIDs := []int64{
		374,  // Кино
		376,  // Спорт
		456,  // Еда
		457,  // Вечеринки
		458,  // Выставки
		459,  // Театры
		460,  // Концерты
		461,  // Экскурсии и путешествия
		463,  // Другие развлечения
		524,  // Хобби и творчество
		525,  // Искусство и культура
		2335, // Интеллектуальные игры
	}

	var skip int = 0
	var limit int = 10
	var expiries time.Duration = 24 * time.Hour

	eventQuery := model.EventQuery{
		City:        &city,
		Skip:        &skip,
		Limit:       &limit,
		CategoryIDs: &categoryIDs,
		StartsAtMin: &now,
	}

	page, err := s.provider.GetEvents(ctx, &eventQuery)

	if err != nil {
		return nil, false, fmt.Errorf("get events: %w", err)
	}

	if len(page.Events) == 0 {
		return nil, false, model.ErrNoEvents
	}

	search := &model.PlaceSearch{
		SearchID: uuid.New(),
		MatchID:  matchID,
		City:     city,
		Events:   page.Events,
		Participants: [2]model.ParticipantResult{
			{UserID: participants[0]},
			{UserID: participants[1]},
		},

		Status:    model.SearchStatusVoting,
		ExpiresAt: time.Now().Add(expiries),
	}

	actual, created, err := s.eventRepo.CreateSearch(ctx, search)

	if err != nil {
		return nil, false, fmt.Errorf("create search: %w", err)
	}

	if actual.City != city {
		return nil, false, model.ErrSearchAlreadyActive
	}

	state, err := toSearchEventsState(actual, userID)

	if err != nil {
		return nil, false, fmt.Errorf("to search events state: %w", err)
	}

	return state, created, nil
}

func (s *EventService) GetSearchEvents(ctx context.Context, userID, matchID int64, searchID uuid.UUID) (*model.SearchEventsState, error) {

	_, err := s.matchRepo.GetParticipants(ctx, userID, matchID)

	if err != nil {
		return nil, fmt.Errorf("get participants: %w", err)
	}

	search, err := s.eventRepo.GetBySearchID(ctx, searchID)

	if err != nil {
		return nil, fmt.Errorf("get search state: %w", err)
	}

	if search == nil || search.MatchID != matchID {
		return nil, model.ErrNotFound
	}

	state, err := toSearchEventsState(search, userID)

	if err != nil {
		return nil, fmt.Errorf("to search events state: %w", err)
	}

	return state, nil
}

func (s *EventService) SubmitResults(ctx context.Context, userID, matchID int64, searchID uuid.UUID, likedEventsIDs []int64) (*model.SearchEventsState, error) {

	searchState, err := s.GetSearchEvents(ctx, userID, matchID, searchID)

	if err != nil {
		return nil, fmt.Errorf("get search events: %w", err)
	}

	err = validateEvents(searchState.Events, likedEventsIDs)

	if err != nil {
		return nil, fmt.Errorf("validate events: %w", err)
	}

	placeSearch, err := s.eventRepo.SubmitResults(ctx, userID, matchID, searchID, likedEventsIDs)

	if err != nil {
		return nil, fmt.Errorf("submit results: %w", err)
	}

	return toSearchEventsState(placeSearch, userID)
}

func validateEvents(events []model.Event, likedEventsIDs []int64) error {

	if len(likedEventsIDs) == 0 {
		return nil
	}

	seen := make(map[int64]bool)

	for _, id := range likedEventsIDs {
		_, exists := seen[id]
		if exists {
			return model.ErrInvalidEventResults
		}
		seen[id] = true
	}

	eventIDs := make(map[int64]bool, len(events))
	for _, event := range events {
		eventIDs[event.ID] = true
	}

	for _, id := range likedEventsIDs {
		_, exists := eventIDs[id]

		if !exists {
			return model.ErrInvalidEventResults
		}
	}

	return nil
}

func toSearchEventsState(search *model.PlaceSearch, userID int64) (*model.SearchEventsState, error) {

	currentIndex := -1
	for i, participant := range search.Participants {
		if participant.UserID == userID {
			currentIndex = i
			break
		}
	}

	if currentIndex == -1 {
		return nil, model.ErrNotFound
	}

	current := search.Participants[currentIndex]
	partner := search.Participants[1-currentIndex]

	state := &model.SearchEventsState{
		SearchID:            search.SearchID,
		MatchID:             search.MatchID,
		City:                search.City,
		Events:              search.Events,
		Status:              search.Status,
		ExpiresAt:           search.ExpiresAt,
		CurrentUserFinished: current.Submitted,
		PartnerFinished:     partner.Submitted,
		Matches:             make([]model.Event, 0),
	}

	if current.Submitted {
		state.CurrentUserLikedEventIDs = make([]int64, len(current.LikedEventsIDs))
		copy(state.CurrentUserLikedEventIDs, current.LikedEventsIDs)
	}

	if search.Status == model.SearchStatusVoting && current.Submitted && !partner.Submitted {
		state.Status = model.SearchStatusWaitingPartner
	}

	if search.Status == model.SearchStatusCompleted {
		state.Matches = intersectEvents(search.Events, current.LikedEventsIDs, partner.LikedEventsIDs)
	}

	return state, nil
}

func intersectEvents(events []model.Event, firstIDs, secondIDs []int64) []model.Event {

	first := make(map[int64]bool, len(firstIDs))
	second := make(map[int64]bool, len(secondIDs))

	for _, id := range firstIDs {
		first[id] = true
	}
	for _, id := range secondIDs {
		second[id] = true
	}

	matches := make([]model.Event, 0)
	for _, event := range events {
		if first[event.ID] && second[event.ID] {
			matches = append(matches, event)
		}
	}

	return matches
}

// тут по userID надо participants заполнить поля?

// func (s *EventService) checkParticipants(ctx context.Context, userID, matchID int64) error {

// 	_, err := s.matchRepo.GetParticipants(ctx, userID, matchID)

// 	if
// }
