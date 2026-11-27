package client

import (
	"context"
	profilepb "dating-app/gen/go/profile/v1"
	"dating-app/services/chat/internal/model"
	"fmt"
)

type ProfileAdapter struct {
	client profilepb.ProfileServiceClient
}

func NewProfileAdapter(client profilepb.ProfileServiceClient) *ProfileAdapter {
	return &ProfileAdapter{client: client}
}

func (a *ProfileAdapter) GetChatProfiles(ctx context.Context, userIDs []int64) (map[int64]model.ChatProfile, error) {

	if len(userIDs) == 0 {
		return nil, nil
	}

	result := make(map[int64]model.ChatProfile, len(userIDs))

	response, err := a.client.GetProfileChatSummaries(ctx, &profilepb.GetProfileChatSummariesRequest{UserIds: userIDs})

	if err != nil {
		return nil, err
	}

	if response == nil {
		return nil, fmt.Errorf("proto: missing profile chat summaries response")
	}

	for _, profile := range response.GetProfiles() {

		if profile == nil {
			return nil, fmt.Errorf("proto: missing profile chat summary")
		}

		item := model.ChatProfile{
			UserID:    profile.GetUserId(),
			Name:      profile.Name,
			AvatarURL: profile.PhotoUrl,
			Tags:      profile.GetTags(),
		}

		if profile.Age != nil {

			age := int(profile.GetAge())
			item.Age = &age
		}

		result[profile.GetUserId()] = item
	}

	return result, nil
}
