package client

import (
	"context"
	matchpb "dating-app/gen/go/match/v1"
	"dating-app/internal/model"
	"fmt"

	"github.com/google/uuid"
)

type EventAdapter struct {
	client matchpb.MatchServiceClient
}

func NewEventAdapter(client matchpb.MatchServiceClient) *EventAdapter {
	return &EventAdapter{client: client}
}

func (a *EventAdapter) SearchEventsByCity(ctx context.Context, userID int64, matchID int64, city string) (*model.SearchEventsState, bool, error) {
	response, err := a.client.SearchEventsByCity(ctx, &matchpb.SearchEventsByCityRequest{UserId: userID, MatchId: matchID, City: city})

	if err != nil {
		return nil, false, err
	}

	state, err := searchEventsStateFromProto(response.GetState())
	if err != nil {
		return nil, false, err
	}

	state.MatchID = matchID

	return state, response.GetCreated(), nil
}

func (a *EventAdapter) GetSearchEvents(ctx context.Context, userID, matchID int64, searchID uuid.UUID) (*model.SearchEventsState, error) {
	response, err := a.client.GetSearchEvents(ctx, &matchpb.GetSearchEventsRequest{UserId: userID, MatchId: matchID, SearchId: searchID.String()})

	if err != nil {
		return nil, err
	}

	state, err := searchEventsStateFromProto(response.GetState())
	if err != nil {
		return nil, err
	}

	state.MatchID = matchID

	return state, nil
}

func (a *EventAdapter) SubmitResults(ctx context.Context, userID, matchID int64, searchID uuid.UUID, likedEventsIDs []int64) (*model.SearchEventsState, error) {
	response, err := a.client.SubmitResults(ctx, &matchpb.SubmitResultsRequest{UserId: userID, MatchId: matchID, SearchId: searchID.String(), LikedEventIds: likedEventsIDs})

	if err != nil {
		return nil, err
	}

	state, err := searchEventsStateFromProto(response.GetState())
	if err != nil {
		return nil, err
	}

	state.MatchID = matchID

	return state, nil
}

func searchEventsStateFromProto(state *matchpb.SearchEventsState) (*model.SearchEventsState, error) {

	if state == nil {
		return nil, fmt.Errorf("proto: missing state")
	}

	searchID, err := uuid.Parse(state.GetSearchId())
	if err != nil {
		return nil, fmt.Errorf("proto: search_id: %w", err)
	}

	if err := state.GetExpiresAt().CheckValid(); err != nil {
		return nil, fmt.Errorf("proto: expires_at: %w", err)
	}

	var status model.SearchStatus
	switch state.GetStatus() {
	case matchpb.SearchStatus_SEARCH_STATUS_VOTING:
		status = model.SearchStatusVoting
	case matchpb.SearchStatus_SEARCH_STATUS_WAITING_PARTNER:
		status = model.SearchStatusWaitingPartner
	case matchpb.SearchStatus_SEARCH_STATUS_COMPLETED:
		status = model.SearchStatusCompleted
	case matchpb.SearchStatus_SEARCH_STATUS_CANCELLED:
		status = model.SearchStatusCancelled
	default:
		return nil, fmt.Errorf("proto: unknown status %d", state.GetStatus())
	}

	events, err := eventsFromProto(state.GetEvents())

	if err != nil {
		return nil, err
	}

	matches, err := eventsFromProto(state.GetMatches())
	if err != nil {
		return nil, err
	}

	return &model.SearchEventsState{
		SearchID:                 searchID,
		City:                     state.GetCity(),
		Events:                   events,
		Status:                   status,
		ExpiresAt:                state.GetExpiresAt().AsTime(),
		CurrentUserLikedEventIDs: state.GetCurrentUserLikedEventIds(),
		CurrentUserFinished:      state.GetCurrentUserFinished(),
		PartnerFinished:          state.GetPartnerFinished(),
		Matches:                  matches,
	}, nil
}

func eventsFromProto(events []*matchpb.Event) ([]model.Event, error) {
	result := make([]model.Event, 0, len(events))

	for _, event := range events {

		if event == nil {
			return nil, fmt.Errorf("proto: missing event")
		}

		if err := event.GetStartsAt().CheckValid(); err != nil {
			return nil, fmt.Errorf("proto: event %d starts_at: %w", event.GetId(), err)
		}

		location := event.GetLocation()
		if location == nil {
			return nil, fmt.Errorf("proto: event %d: missing location", event.GetId())
		}

		categories := make([]model.EventCategory, 0, len(event.GetCategories()))
		for _, category := range event.GetCategories() {
			if category == nil {
				return nil, fmt.Errorf("proto: event %d: missing category", event.GetId())
			}
			categories = append(categories, model.EventCategory{ID: category.GetId(), Name: category.GetName()})
		}

		result = append(result, model.Event{
			ID:         event.GetId(),
			Name:       event.GetName(),
			ImageURL:   event.ImageUrl,
			URL:        event.GetUrl(),
			StartsAt:   event.GetStartsAt().AsTime(),
			Location:   model.EventLocation{City: location.GetCity(), Address: location.Address},
			Categories: categories,
		})
	}
	return result, nil
}
