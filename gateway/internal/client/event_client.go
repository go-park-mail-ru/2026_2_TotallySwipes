package client

import (
	"context"
	"dating-app/gateway/internal/client/mapper"
	"dating-app/internal/model"
	matchpb "dating-app/services/matching/api/gen/go/match/v1"

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

	state, err := mapper.SearchEventsStateFromProto(response.GetState())
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

	state, err := mapper.SearchEventsStateFromProto(response.GetState())
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

	state, err := mapper.SearchEventsStateFromProto(response.GetState())
	if err != nil {
		return nil, err
	}

	state.MatchID = matchID

	return state, nil
}
