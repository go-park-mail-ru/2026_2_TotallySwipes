package mapper

import (
	matchpb "dating-app/gen/go/match/v1"
	"dating-app/internal/model"
	"fmt"
)

func MatchPageFromProto(response *matchpb.GetMatchesResponse) (*model.MatchPage, error) {
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

func MatchSummaryFromProto(response *matchpb.GetSummaryResponse) (*model.MatchSummary, error) {
	if response == nil {
		return nil, fmt.Errorf("proto: missing match summary")
	}

	return &model.MatchSummary{Total: int(response.GetTotal()), New: int(response.GetNewCount())}, nil
}

func MatchedProfileFromProto(response *matchpb.GetMatchedProfileResponse) (*model.MatchedProfile, error) {
	if response == nil {
		return nil, fmt.Errorf("proto: missing matched profile response")
	}

	item, err := FeedItemFromProto(response.GetItem())

	if err != nil {
		return nil, err
	}

	return &model.MatchedProfile{MatchID: response.GetMatchId(), Item: item}, nil
}
