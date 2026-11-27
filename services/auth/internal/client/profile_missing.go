// Package client - обращения сервиса авторизации к другим сервисам.
package client

import (
	"context"
	"fmt"

	profilepb "dating-app/gen/go/profile/v1"
)

// ProfileMissing узнаёт у сервиса анкет, каких обязательных полей не хватает анкете;
// реализует service.ProfileMissingChecker.
type ProfileMissing struct {
	client profilepb.ProfileServiceClient
}

func NewProfileMissing(client profilepb.ProfileServiceClient) *ProfileMissing {
	return &ProfileMissing{client: client}
}

func (p *ProfileMissing) Missing(ctx context.Context, userID int64) ([]string, error) {
	resp, err := p.client.GetShortProfile(ctx, &profilepb.GetShortProfileRequest{UserId: userID})
	if err != nil {
		return nil, fmt.Errorf("profile missing user id=%d: %w", userID, err)
	}
	return append([]string{}, resp.GetMissing()...), nil
}
