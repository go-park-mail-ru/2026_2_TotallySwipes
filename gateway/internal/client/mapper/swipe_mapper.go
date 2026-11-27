package mapper

import (
	"dating-app/internal/model"
	matchpb "dating-app/services/matching/api/gen/go/match/v1"
	"fmt"
)

func SwipeToProto(swipe *model.Swipe) (*matchpb.SwipeRequest, error) {
	if swipe == nil {
		return nil, fmt.Errorf("missing swipe")
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
		return nil, fmt.Errorf("unknown swipe action: %q", swipe.Action)
	}

	return &matchpb.SwipeRequest{UserId: swipe.AuthorID, TargetUserId: swipe.TargetID, Action: action}, nil
}
