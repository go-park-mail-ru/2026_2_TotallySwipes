package client

import (
	"context"
	"dating-app/gateway/internal/client/mapper"
	"dating-app/gateway/internal/model"
	matchpb "dating-app/services/matching/api/gen/go/match/v1"
)

type FeedAdapter struct {
	client matchpb.MatchServiceClient
}

func NewFeedAdapter(client matchpb.MatchServiceClient) *FeedAdapter {
	return &FeedAdapter{client: client}
}

func (a *FeedAdapter) GetNextFeed(ctx context.Context, userID int64, limit int, cursor *int64) (*model.FeedPage, error) {

	l := int32(limit)
	response, err := a.client.GetNextFeed(ctx, &matchpb.GetNextFeedRequest{UserId: userID, Limit: &l, Cursor: cursor})

	if err != nil {
		return nil, err
	}

	feedPage, err := mapper.FeedPageFromProto(response)

	if err != nil {
		return nil, err
	}

	return feedPage, nil
}
