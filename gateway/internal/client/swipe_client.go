package client

import (
	"context"
	"dating-app/gateway/internal/client/mapper"
	"dating-app/gateway/internal/model"
	matchpb "dating-app/services/matching/api/gen/go/match/v1"
)

type SwipeAdapter struct {
	client matchpb.MatchServiceClient
}

func NewSwipeAdapter(client matchpb.MatchServiceClient) *SwipeAdapter {
	return &SwipeAdapter{client: client}
}

func (a *SwipeAdapter) Swipe(ctx context.Context, swipe *model.Swipe) error {

	request, err := mapper.SwipeToProto(swipe)

	if err != nil {
		return err
	}

	_, err = a.client.Swipe(ctx, request)

	return err
}
