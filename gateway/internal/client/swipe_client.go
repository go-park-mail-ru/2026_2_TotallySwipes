package client

import (
	"context"
	"dating-app/gateway/internal/client/mapper"
	matchpb "dating-app/gen/go/match/v1"
	"dating-app/internal/model"
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
