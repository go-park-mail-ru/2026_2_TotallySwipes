package client

import (
	"context"
	profilepb "dating-app/gen/go/profile/v1"
	"dating-app/internal/model"
	"fmt"
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

	filter, err := searchFilterFromProto(response.GetFilter())

	if err != nil {
		return nil, err
	}

	return filter, nil
}

func (a *FilterAdapter) Set(ctx context.Context, userID int64, f model.SearchFilter) (*model.SearchFilter, error) {

	sex := string(f.Sex)
	ageFrom := int32(f.AgeFrom)
	ageTo := int32(f.AgeTo)

	response, err := a.client.SetSearchFilter(ctx, &profilepb.SetSearchFilterRequest{
		UserId: userID,
		Filter: &profilepb.SearchFilter{
			Sex:     &sex,
			AgeFrom: &ageFrom,
			AgeTo:   &ageTo,
		},
	})

	if err != nil {
		return nil, err
	}

	filter, err := searchFilterFromProto(response.GetFilter())

	if err != nil {
		return nil, err
	}

	return filter, nil
}

func searchFilterFromProto(filter *profilepb.SearchFilter) (*model.SearchFilter, error) {

	if filter == nil {
		return nil, fmt.Errorf("proto: missing filter")
	}

	if filter.Sex == nil {
		return nil, fmt.Errorf("proto: missing filter.sex")
	}

	if filter.AgeFrom == nil {
		return nil, fmt.Errorf("proto: missing filter.age_from")
	}

	if filter.AgeTo == nil {
		return nil, fmt.Errorf("proto: missing filter.age_to")
	}

	return &model.SearchFilter{
		Sex:     model.SearchSex(filter.GetSex()),
		AgeFrom: int(filter.GetAgeFrom()),
		AgeTo:   int(filter.GetAgeTo()),
	}, nil
}
