package client

import (
	"context"
	"dating-app/gateway/internal/client/mapper"
	"dating-app/internal/model"
	matchpb "dating-app/services/matching/api/gen/go/match/v1"
)

type MatchAdapter struct {
	client matchpb.MatchServiceClient
}

func NewMatchAdapter(client matchpb.MatchServiceClient) *MatchAdapter {
	return &MatchAdapter{client: client}
}

func (a *MatchAdapter) GetMatches(ctx context.Context, userID int64, limit int, cursor *int64) (*model.MatchPage, error) {
	l := int32(limit)
	response, err := a.client.GetMatches(ctx, &matchpb.GetMatchesRequest{UserId: userID, Limit: &l, Cursor: cursor})

	if err != nil {
		return nil, err
	}

	return mapper.MatchPageFromProto(response)
}

func (a *MatchAdapter) MarkSeen(ctx context.Context, userID, matchID int64) error {
	_, err := a.client.MarkSeen(ctx, &matchpb.MarkSeenRequest{UserId: userID, MatchId: matchID})

	return err
}

func (a *MatchAdapter) GetSummary(ctx context.Context, userID int64) (*model.MatchSummary, error) {
	response, err := a.client.GetSummary(ctx, &matchpb.GetSummaryRequest{UserId: userID})

	if err != nil {
		return nil, err
	}

	return mapper.MatchSummaryFromProto(response)
}

func (a *MatchAdapter) GetMatchedProfile(ctx context.Context, viewerID, targetID int64) (*model.MatchedProfile, error) {

	response, err := a.client.GetMatchedProfile(ctx, &matchpb.GetMatchedProfileRequest{
		UserId:       viewerID,
		TargetUserId: targetID,
	})

	if err != nil {
		return nil, err
	}

	return mapper.MatchedProfileFromProto(response)
}
