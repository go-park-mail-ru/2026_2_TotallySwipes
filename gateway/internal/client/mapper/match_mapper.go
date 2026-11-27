package mapper

import (
	"dating-app/gateway/internal/model"
	matchpb "dating-app/services/matching/api/gen/go/match/v1"
	"fmt"
	"strconv"
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

		if item.ChatId != nil {

			chatID, err := strconv.ParseInt(item.GetChatId(), 10, 64)

			if err != nil {
				return nil, fmt.Errorf("proto: invalid match.chat_id: %w", err)
			}

			if chatID <= 0 {
				return nil, fmt.Errorf("proto: match.chat_id must be positive")
			}

			m.ChatID = &chatID
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
