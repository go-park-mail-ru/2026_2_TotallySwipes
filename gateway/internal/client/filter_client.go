package client

import (
	"context"
	"dating-app/gateway/internal/client/mapper"
	profilepb "dating-app/gen/go/profile/v1"
	"dating-app/internal/model"
)

type FilterAdapter struct {
	client profilepb.ProfileServiceClient
}

func NewFilterAdapter(client profilepb.ProfileServiceClient) *FilterAdapter {
	return &FilterAdapter{client: client}
}

func (a *FilterAdapter) Get(ctx context.Context, userID int64) (*model.SearchFilter, error) {

	response, err := a.client.GetSearchFilter(ctx, &profilepb.GetSearchFilterRequest{UserId: userID})

	if err != nil {
		return nil, err
	}

	filter, err := mapper.SearchFilterFromProto(response.GetFilter())

	if err != nil {
		return nil, err
	}

	return filter, nil
}

func (a *FilterAdapter) Set(ctx context.Context, userID int64, f model.SearchFilter) (*model.SearchFilter, error) {

	response, err := a.client.SetSearchFilter(ctx, &profilepb.SetSearchFilterRequest{
		UserId: userID, Filter: mapper.SearchFilterToProto(f),
	})

	if err != nil {
		return nil, err
	}

	filter, err := mapper.SearchFilterFromProto(response.GetFilter())

	if err != nil {
		return nil, err
	}

	return filter, nil
}
