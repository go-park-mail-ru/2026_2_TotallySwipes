package client

import (
	"context"
	matchpb "dating-app/gen/go/match/v1"
	"dating-app/internal/model"
	"fmt"
)

type SwipeAdapter struct {
	client matchpb.MatchServiceClient
}

func NewSwipeAdapter(client matchpb.MatchServiceClient) *SwipeAdapter {
	return &SwipeAdapter{client: client}
}

func (a *SwipeAdapter) Swipe(ctx context.Context, swipe *model.Swipe) error {

	if swipe == nil {
		return fmt.Errorf("missing swipe")
	}

	var action matchpb.SwipeAction
	switch swipe.Action {
	case model.SwipeLike:
		action = matchpb.SwipeAction_SWIPE_ACTION_LIKE
	case model.SwipeDislike:
		action = matchpb.SwipeAction_SWIPE_ACTION_DISLIKE
	case model.SwipeSuperLike:
		action = matchpb.SwipeAction_SWIPE_ACTION_SUPER_LIKE
	default:
		return fmt.Errorf("unknown swipe action: %q", swipe.Action)
	}

	_, err := a.client.Swipe(ctx, &matchpb.SwipeRequest{UserId: swipe.AuthorID, TargetUserId: swipe.TargetID, Action: action})

	return err
}
