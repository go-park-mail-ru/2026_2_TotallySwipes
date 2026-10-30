package service

import (
	"context"
	"dating-app/internal/model"
	"fmt"
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
	GetActive(ctx context.Context, matchID int64) (*model.SearchEventsState, error)
	GetBySearchID(ctx context.Context, searchID uuid.UUID) (*model.SearchEventsState, error)
	CreateSearch(ctx context.Context, state *model.SearchEventsState) (actual *model.SearchEventsState, created bool, err error)
}

type EventService struct {
	provider  EventProvider
	matchRepo MatchRepository
	eventRepo EventRepository
}

var skip int = 0
var limit int = 10

func (s *EventService) SearchEventsByCity(ctx context.Context, userID int64, matchID int64, city string) (*model.SearchEventsState, bool, error) {

	if city == "" {
		return nil, false, model.ErrInvalidCity
	}

	_, err := s.matchRepo.GetParticipants(ctx, userID, matchID)
	if err != nil {
		return nil, false, fmt.Errorf("get participants: %w", err)

	}

	state, err := s.eventRepo.GetActive(ctx, matchID)

	if err != nil {
		return nil, false, fmt.Errorf("get active: %w", err)
	}

	if state != nil {

		if state.City != city {
			return nil, false, model.ErrSearchAlreadyActive
		}

		return state, false, nil
	}

	events := make([]model.Event, 0, 0)
	state = &model.SearchEventsState{
		SearchID: uuid.New(),
		MatchID:  matchID,
		City:     city,
		Events:   events,
		Status:   model.SearchStatusVoting,
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

	if page.Events == nil {
		return nil, false, model.ErrNoEvents
	}

	state.Events = page.Events

	actual, created, err := s.eventRepo.CreateSearch(ctx, state)

	if err != nil {
		return nil, false, fmt.Errorf("create search: %w", err)
	}

	if actual.City != city {
		return nil, false, model.ErrSearchAlreadyActive
	}

	return actual, created, nil
}

func (s *EventService) GetSearhEvents(ctx context.Context, userID, matchID int64, searchID uuid.UUID) (*model.SearchEventsState, error) {

	_, err := s.matchRepo.GetParticipants(ctx, userID, matchID)

	if err != nil {
		return nil, fmt.Errorf("get participants: %w", err)
	}

	state, err := s.eventRepo.GetBySearchID(ctx, searchID)

	if err != nil {
		return nil, fmt.Errorf("get search state: %w", err)
	}

	if state == nil || state.MatchID != matchID {
		return nil, model.ErrNotFound
	}

	return state, nil
}
