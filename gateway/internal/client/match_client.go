package client

import (
	"context"
	matchpb "dating-app/gen/go/match/v1"
	"dating-app/internal/model"
	"fmt"
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

	if response == nil {
		return nil, fmt.Errorf("proto: missing matches response")
	}

	page := &model.MatchPage{Items: make([]model.Match, 0, len(response.GetItems())), Total: int(response.GetTotal()), NextCursor: response.NextCursor}

	for _, item := range response.GetItems() {

		if item == nil {
			return nil, fmt.Errorf("proto: missing match item")
		}

		if err := item.GetCreatedAt().CheckValid(); err != nil {
			return nil, fmt.Errorf("proto: invalid match.created_at: %w", err)
		}

		m := model.Match{
			ID:              item.GetMatchId(),
			UserID:          item.GetUserId(),
			Name:            item.Name,
			MainPhotoURL:    item.PhotoUrl,
			Compatibility:   item.Compatibility,
			IsNew:           item.GetIsNew(),
			PartnerFinished: item.GetPartnerFinished(),
			CanMeetup:       item.GetCanMeetup(),
			CreatedAt:       item.GetCreatedAt().AsTime()}

		if item.Age != nil {
			age := int(item.GetAge())
			m.Age = &age
		}

		page.Items = append(page.Items, m)
	}

	return page, nil
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

	if response == nil {
		return nil, fmt.Errorf("proto: missing match summary")
	}

	return &model.MatchSummary{Total: int(response.GetTotal()), New: int(response.GetNewCount())}, nil
}

func (a *MatchAdapter) GetMatchedProfile(ctx context.Context, viewerID, targetID int64) (*model.MatchedProfile, error) {

	response, err := a.client.GetMatchedProfile(ctx, &matchpb.GetMatchedProfileRequest{
		UserId:       viewerID,
		TargetUserId: targetID,
	})

	if err != nil {
		return nil, err
	}

	if response == nil {
		return nil, fmt.Errorf("proto: missing matched profile response")
	}

	item, err := feedItemFromProto(response.GetItem())

	if err != nil {
		return nil, err
	}

	return &model.MatchedProfile{MatchID: response.GetMatchId(), Item: item}, nil
}
