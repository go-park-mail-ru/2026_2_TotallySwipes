package client

import (
	"context"
	"fmt"
	"strconv"
	"time"

	profilepb "dating-app/gen/go/profile/v1"
	"dating-app/services/matching/internal/model"
)

type ProfileAdapter struct {
	client profilepb.ProfileServiceClient
}

func NewProfileAdapter(client profilepb.ProfileServiceClient) *ProfileAdapter {

	return &ProfileAdapter{client: client}
}

func (a *ProfileAdapter) GetProfilesBatch(ctx context.Context, ids []int64, filter model.MatchFilter) (map[int64]model.Profile, error) {

	request := &profilepb.GetProfilesBatchRequest{UserIds: ids}

	if !filter.IsEmpty() {
		request.Filter = &profilepb.MatchFilter{Name: filter.Name, Sex: filter.Sex}

		if filter.AgeFrom != nil {
			value := int32(*filter.AgeFrom)
			request.Filter.AgeFrom = &value
		}

		if filter.AgeTo != nil {
			value := int32(*filter.AgeTo)
			request.Filter.AgeTo = &value
		}
	}

	response, err := a.client.GetProfilesBatch(ctx, request)

	if err != nil {
		return nil, err
	}

	if response == nil {
		return nil, fmt.Errorf("proto: missing profiles batch response")
	}

	result := make(map[int64]model.Profile, len(response.GetProfiles()))
	allowed := make(map[int64]bool, len(ids))

	for _, id := range ids {
		allowed[id] = true
	}

	for _, item := range response.GetProfiles() {
		profile, err := profileFromProto(item)

		if err != nil {
			return nil, err
		}

		if !allowed[profile.UserID] {
			return nil, fmt.Errorf("proto: unexpected profile user_id")
		}

		if _, exists := result[profile.UserID]; exists {
			return nil, fmt.Errorf("proto: duplicate profile user_id")
		}

		result[profile.UserID] = profile
	}

	return result, nil
}

func (a *ProfileAdapter) SearchCandidates(ctx context.Context, userID int64, limit int, cursor *int64) ([]model.Profile, *int64, error) {

	request := &profilepb.SearchCandidatesRequest{UserId: userID, Limit: int32(limit)}

	if cursor != nil {
		value := strconv.FormatInt(*cursor, 10)
		request.Cursor = &value
	}

	response, err := a.client.SearchCandidates(ctx, request)

	if err != nil {
		return nil, nil, err
	}

	if response == nil {
		return nil, nil, fmt.Errorf("proto: missing search candidates response")
	}

	result := make([]model.Profile, 0, len(response.GetCandidates()))

	if len(response.GetCandidates()) > limit {
		return nil, nil, fmt.Errorf("proto: candidates exceed requested limit")
	}

	for _, item := range response.GetCandidates() {
		profile, err := profileFromProto(item)

		if err != nil {
			return nil, nil, err
		}

		result = append(result, profile)
	}

	var next *int64

	if response.NextCursor != nil {
		value, err := strconv.ParseInt(response.GetNextCursor(), 10, 64)

		if err != nil || value <= 0 {
			return nil, nil, fmt.Errorf("proto: invalid candidates next_cursor")
		}

		next = &value
	}

	return result, next, nil
}

func profileFromProto(value *profilepb.ProfileWithBigFive) (model.Profile, error) {

	if value == nil || value.GetProfile() == nil {
		return model.Profile{}, fmt.Errorf("proto: missing profile")
	}

	p := value.GetProfile()

	if p.GetUserId() <= 0 {
		return model.Profile{}, fmt.Errorf("proto: invalid profile.user_id")
	}

	result := model.Profile{ID: p.GetProfileId(), UserID: p.GetUserId(), Name: p.Name, Sex: p.Sex, AboutMe: p.AboutMe, Work: p.Work, City: p.City, Tags: p.GetTags(), Missing: p.GetMissing(), Photos: make([]model.FeedPhoto, 0, len(p.GetPhotos()))}

	if p.BirthDate != nil {
		date := p.GetBirthDate()
		birth := time.Date(int(date.Year), time.Month(date.Month), int(date.Day), 0, 0, 0, 0, time.UTC)

		if birth.Year() != int(date.Year) || int(birth.Month()) != int(date.Month) || birth.Day() != int(date.Day) {
			return model.Profile{}, fmt.Errorf("proto: invalid birth_date")
		}

		result.BirthDate = &birth
	}

	if p.Age != nil {
		value := int(p.GetAge())
		result.Age = &value
	}

	if p.DatingGoal != nil {
		value := model.DatingGoal(p.GetDatingGoal())
		result.DatingGoal = &value
	}

	if p.Education != nil {
		value := model.Education(p.GetEducation())
		result.Education = &value
	}

	if p.Smoking != nil {
		value := model.Attitude(p.GetSmoking())
		result.Smoking = &value
	}

	if p.Alcohol != nil {
		value := model.Attitude(p.GetAlcohol())
		result.Alcohol = &value
	}

	if p.Height != nil {
		value := int(p.GetHeight())
		result.Height = &value
	}

	for _, photo := range p.GetPhotos() {
		if photo == nil || photo.GetId() <= 0 || photo.GetUrl() == "" {
			return model.Profile{}, fmt.Errorf("proto: invalid profile photo")
		}

		result.Photos = append(result.Photos, model.FeedPhoto{ID: photo.GetId(), URL: photo.GetUrl()})
	}

	if b := value.GetBigFive(); b != nil {
		result.Psycho = model.BigFive{Openness: b.Openness, Conscientiousness: b.Conscientiousness, Extraversion: b.Extraversion, Agreeableness: b.Agreeableness, Neuroticism: b.Neuroticism}
	}

	return result, nil
}
