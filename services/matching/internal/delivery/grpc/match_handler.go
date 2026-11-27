package grpc

import (
	"context"
	"fmt"

	matchpb "dating-app/services/matching/api/gen/go/match/v1"
	"dating-app/services/matching/internal/model"
	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type MatchService interface {
	GetMatches(ctx context.Context, userID int64, limit int, cursor *int64, filter model.MatchFilter) (*model.MatchPage, error)
	GetSummary(ctx context.Context, userID int64) (*model.MatchSummary, error)
	MarkSeen(ctx context.Context, userID, matchID int64) error
	GetMatchedProfile(ctx context.Context, viewerID, targetID int64) (*model.MatchedProfile, error)
}

type FeedService interface {
	GetNextFeed(ctx context.Context, userID int64, limit int, cursor *int64) (*model.FeedPage, error)
}
type SwipeService interface {
	Swipe(ctx context.Context, swipe *model.Swipe) error
}
type EventService interface {
	SearchEventsByCity(ctx context.Context, userID, matchID int64, city string) (*model.SearchEventsState, bool, error)
	GetSearchEvents(ctx context.Context, userID, matchID int64, searchID uuid.UUID) (*model.SearchEventsState, error)
	SubmitResults(ctx context.Context, userID, matchID int64, searchID uuid.UUID, likedEventIDs []int64) (*model.SearchEventsState, error)
}

type MatchHandler struct {
	matchpb.UnimplementedMatchServiceServer
	matches MatchService
	feed    FeedService
	swipes  SwipeService
	events  EventService
}

var _ matchpb.MatchServiceServer = (*MatchHandler)(nil)

func NewMatchHandler(matches MatchService, feed FeedService, swipes SwipeService, events EventService) *MatchHandler {

	return &MatchHandler{matches: matches, feed: feed, swipes: swipes, events: events}
}

func missingRequest() error {

	return status.Error(codes.InvalidArgument, "missing request")
}

func (h *MatchHandler) GetMatches(ctx context.Context, request *matchpb.GetMatchesRequest) (*matchpb.GetMatchesResponse, error) {

	if request == nil {
		return nil, missingRequest()
	}

	limit := model.MatchesDefaultLimit

	if request.Limit != nil {
		limit = int(request.GetLimit())
	}

	filter := model.MatchFilter{}

	if value := request.GetFilter(); value != nil {
		filter.Name, filter.Sex = value.Name, value.Sex

		if value.AgeFrom != nil {
			age := int(value.GetAgeFrom())
			filter.AgeFrom = &age
		}

		if value.AgeTo != nil {
			age := int(value.GetAgeTo())
			filter.AgeTo = &age
		}
	}

	page, err := h.matches.GetMatches(ctx, request.GetUserId(), limit, request.Cursor, filter)

	if err != nil {
		return nil, matchError(ctx, err)
	}

	result := &matchpb.GetMatchesResponse{Items: make([]*matchpb.MatchItem, 0, len(page.Items)), Total: int64(page.Total), NextCursor: page.NextCursor}

	for _, item := range page.Items {
		result.Items = append(result.Items, matchToProto(item))
	}

	return result, nil
}

func (h *MatchHandler) GetSummary(ctx context.Context, request *matchpb.GetSummaryRequest) (*matchpb.GetSummaryResponse, error) {

	if request == nil {
		return nil, missingRequest()
	}

	result, err := h.matches.GetSummary(ctx, request.GetUserId())

	if err != nil {
		return nil, matchError(ctx, err)
	}

	return &matchpb.GetSummaryResponse{Total: int64(result.Total), NewCount: int64(result.New), HasNew: result.New > 0}, nil
}

func (h *MatchHandler) MarkSeen(ctx context.Context, request *matchpb.MarkSeenRequest) (*matchpb.MarkSeenResponse, error) {

	if request == nil {
		return nil, missingRequest()
	}

	if err := h.matches.MarkSeen(ctx, request.GetUserId(), request.GetMatchId()); err != nil {
		return nil, matchError(ctx, err)
	}

	return &matchpb.MarkSeenResponse{}, nil
}

func (h *MatchHandler) GetNextFeed(ctx context.Context, request *matchpb.GetNextFeedRequest) (*matchpb.GetNextFeedResponse, error) {

	if request == nil {
		return nil, missingRequest()
	}

	limit := 10

	if request.Limit != nil {
		limit = int(request.GetLimit())
	}

	page, err := h.feed.GetNextFeed(ctx, request.GetUserId(), limit, request.Cursor)

	if err != nil {
		return nil, matchError(ctx, err)
	}

	result := &matchpb.GetNextFeedResponse{Items: make([]*matchpb.FeedItem, 0, len(page.Items)), NextCursor: page.NextCursor}

	for _, item := range page.Items {
		result.Items = append(result.Items, feedItemToProto(item))
	}

	return result, nil
}

func (h *MatchHandler) GetMatchedProfile(ctx context.Context, request *matchpb.GetMatchedProfileRequest) (*matchpb.GetMatchedProfileResponse, error) {

	if request == nil {
		return nil, missingRequest()
	}

	result, err := h.matches.GetMatchedProfile(ctx, request.GetUserId(), request.GetTargetUserId())

	if err != nil {
		return nil, matchError(ctx, err)
	}

	return &matchpb.GetMatchedProfileResponse{MatchId: result.MatchID, Item: feedItemToProto(result.Item)}, nil
}

func (h *MatchHandler) Swipe(ctx context.Context, request *matchpb.SwipeRequest) (*matchpb.SwipeResponse, error) {

	if request == nil {
		return nil, missingRequest()
	}

	action := model.SwipeAction("")

	switch request.GetAction() {
	case matchpb.SwipeAction_SWIPE_ACTION_LIKE:

		action = model.SwipeLike

	case matchpb.SwipeAction_SWIPE_ACTION_DISLIKE:

		action = model.SwipeDislike

	case matchpb.SwipeAction_SWIPE_ACTION_SUPER_LIKE:

		action = model.SwipeSuperLike

	default:

		return nil, matchError(ctx, model.ErrInvalidSwipe)
	}

	err := h.swipes.Swipe(ctx, &model.Swipe{AuthorID: request.GetUserId(), TargetID: request.GetTargetUserId(), Action: action})

	if err != nil {
		return nil, matchError(ctx, err)
	}

	return &matchpb.SwipeResponse{}, nil
}

func (h *MatchHandler) SearchEventsByCity(ctx context.Context, request *matchpb.SearchEventsByCityRequest) (*matchpb.SearchEventsByCityResponse, error) {

	if request == nil {
		return nil, missingRequest()
	}

	result, created, err := h.events.SearchEventsByCity(ctx, request.GetUserId(), request.GetMatchId(), request.GetCity())

	if err != nil {
		return nil, matchError(ctx, err)
	}

	return &matchpb.SearchEventsByCityResponse{State: searchStateToProto(result), Created: created}, nil
}

func parseSearchID(value string) (uuid.UUID, error) {

	id, err := uuid.Parse(value)

	if err != nil || id == uuid.Nil {
		return uuid.Nil, status.Error(codes.InvalidArgument, "invalid search_id")
	}

	return id, nil
}

func (h *MatchHandler) GetSearchEvents(ctx context.Context, request *matchpb.GetSearchEventsRequest) (*matchpb.GetSearchEventsResponse, error) {

	if request == nil {
		return nil, missingRequest()
	}

	searchID, err := parseSearchID(request.GetSearchId())

	if err != nil {
		return nil, err
	}

	result, err := h.events.GetSearchEvents(ctx, request.GetUserId(), request.GetMatchId(), searchID)

	if err != nil {
		return nil, matchError(ctx, err)
	}

	return &matchpb.GetSearchEventsResponse{State: searchStateToProto(result)}, nil
}

func (h *MatchHandler) SubmitResults(ctx context.Context, request *matchpb.SubmitResultsRequest) (*matchpb.SubmitResultsResponse, error) {

	if request == nil {
		return nil, missingRequest()
	}

	searchID, err := parseSearchID(request.GetSearchId())

	if err != nil {
		return nil, err
	}

	result, err := h.events.SubmitResults(ctx, request.GetUserId(), request.GetMatchId(), searchID, request.GetLikedEventIds())

	if err != nil {
		return nil, matchError(ctx, err)
	}

	if result == nil {
		return nil, matchError(ctx, fmt.Errorf("missing search result"))
	}

	return &matchpb.SubmitResultsResponse{State: searchStateToProto(result)}, nil
}
