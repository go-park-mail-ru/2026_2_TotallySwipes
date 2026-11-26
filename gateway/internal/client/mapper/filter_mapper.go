package mapper

import (
	profilepb "dating-app/gen/go/profile/v1"
	"dating-app/internal/model"
	"fmt"
)

func SearchFilterFromProto(filter *profilepb.SearchFilter) (*model.SearchFilter, error) {

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

func SearchFilterToProto(filter model.SearchFilter) *profilepb.SearchFilter {

	sex, ageFrom, ageTo := string(filter.Sex), int32(filter.AgeFrom), int32(filter.AgeTo)

	return &profilepb.SearchFilter{Sex: &sex, AgeFrom: &ageFrom, AgeTo: &ageTo}
}
