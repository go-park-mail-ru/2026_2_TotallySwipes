package client

import (
	"context"
	profilepb "dating-app/gen/go/profile/v1"
	"dating-app/internal/model"
	"fmt"
	"slices"
	"time"

	"google.golang.org/protobuf/types/known/fieldmaskpb"
)

type ProfileAdapter struct {
	client profilepb.ProfileServiceClient
}

func NewProfileAdapter(client profilepb.ProfileServiceClient) *ProfileAdapter {
	return &ProfileAdapter{client: client}
}

func (a *ProfileAdapter) GetShortProfile(ctx context.Context, userID int64) (*model.ProfileShort, error) {

	response, err := a.client.GetShortProfile(ctx, &profilepb.GetShortProfileRequest{UserId: userID})

	if err != nil {
		return nil, err
	}

	if response == nil {
		return nil, fmt.Errorf("proto: missing short profile response")
	}

	result := &model.ProfileShort{
		UserID:       response.GetUserId(),
		Name:         response.Name,
		MainPhotoURL: response.PhotoUrl,
		Missing:      append([]string{}, response.GetMissing()...),
		City:         response.City,
	}

	if response.Age != nil {
		age := int(response.GetAge())
		result.Age = &age
	}

	return result, nil
}

func (a *ProfileAdapter) GetMyProfile(ctx context.Context, userID int64) (*model.Profile, error) {

	response, err := a.client.GetMyProfile(ctx, &profilepb.GetMyProfileRequest{UserId: userID})

	if err != nil {
		return nil, err
	}

	return profileFromProto(response.GetProfile())
}

func (a *ProfileAdapter) UpdateProfile(ctx context.Context, userID int64, patch *model.ProfilePatch) (*model.Profile, error) {

	request, err := profilePatchToProto(userID, patch)

	if err != nil {
		return nil, err
	}

	response, err := a.client.UpdateProfile(ctx, request)

	if err != nil {
		return nil, err
	}

	return profileFromProto(response.GetProfile())
}

func (a *ProfileAdapter) AddPhoto(ctx context.Context, userID int64, upload model.PhotoUpload) ([]model.Photo, error) {

	response, err := a.client.AddPhoto(ctx, &profilepb.AddPhotoRequest{UserId: userID, Data: upload.Data, Ext: upload.Ext})

	if err != nil {
		return nil, err
	}

	if response == nil {
		return nil, fmt.Errorf("proto: missing add photo response")
	}

	return photosFromProto(response.GetPhotos())
}

func (a *ProfileAdapter) DeletePhoto(ctx context.Context, userID, photoID int64) ([]model.Photo, error) {

	response, err := a.client.DeletePhoto(ctx, &profilepb.DeletePhotoRequest{UserId: userID, PhotoId: photoID})

	if err != nil {
		return nil, err
	}

	if response == nil {
		return nil, fmt.Errorf("proto: missing delete photo response")
	}

	return photosFromProto(response.GetPhotos())
}

func (a *ProfileAdapter) ReorderPhotos(ctx context.Context, userID int64, photoIDs []int64) ([]model.Photo, error) {

	response, err := a.client.ReorderPhotos(ctx, &profilepb.ReorderPhotosRequest{UserId: userID, PhotoIds: photoIDs})

	if err != nil {
		return nil, err
	}

	if response == nil {
		return nil, fmt.Errorf("proto: missing reorder photos response")
	}

	return photosFromProto(response.GetPhotos())
}

func photosFromProto(photos []*profilepb.Photo) ([]model.Photo, error) {

	result := make([]model.Photo, 0, len(photos))

	for index, photo := range photos {

		if photo == nil {
			return nil, fmt.Errorf("proto: missing photo")
		}

		result = append(result, model.Photo{ID: photo.GetId(), URL: photo.GetUrl(), Position: index + 1})
	}

	return result, nil
}

func profileFromProto(profile *profilepb.Profile) (*model.Profile, error) {

	if profile == nil {
		return nil, fmt.Errorf("proto: missing profile")
	}

	photos, err := photosFromProto(profile.GetPhotos())

	if err != nil {
		return nil, err
	}

	result := &model.Profile{
		UserID:          profile.GetUserId(),
		City:            profile.City,
		Photos:          photos,
		Tags:            make([]model.Tag, 0, len(profile.GetTags())),
		HasSearchFilter: !slices.Contains(profile.GetMissing(), model.MissingSearchFilter),
	}

	fields := &result.CurrentVersion.ProfileFields
	fields.Name = profile.Name
	fields.AboutMe = profile.AboutMe
	fields.Work = profile.Work

	if profile.BirthDate != nil {
		date := profile.GetBirthDate()
		year, month, day := int(date.GetYear()), time.Month(date.GetMonth()), int(date.GetDay())
		birthDate := time.Date(year, month, day, 0, 0, 0, 0, time.UTC)

		if year < 1 || year > 9999 || birthDate.Year() != year || birthDate.Month() != month || birthDate.Day() != day {
			return nil, fmt.Errorf("proto: invalid profile.birth_date")
		}

		fields.BirthDate = &birthDate
	}

	if profile.Sex != nil {
		value := model.Sex(profile.GetSex())
		fields.Sex = &value
	}

	if profile.DatingGoal != nil {
		value := model.DatingGoal(profile.GetDatingGoal())
		fields.DatingGoal = &value
	}

	if profile.Education != nil {
		value := model.Education(profile.GetEducation())
		fields.Education = &value
	}

	if profile.Smoking != nil {
		value := model.Attitude(profile.GetSmoking())
		fields.Smoking = &value
	}

	if profile.Alcohol != nil {
		value := model.Attitude(profile.GetAlcohol())
		fields.Alcohol = &value
	}

	if profile.Height != nil {
		height := int(profile.GetHeight())
		fields.Height = &height
	}

	for _, tag := range profile.GetTags() {
		result.Tags = append(result.Tags, model.Tag{Name: tag})
	}

	return result, nil
}

func profilePatchToProto(userID int64, patch *model.ProfilePatch) (*profilepb.UpdateProfileRequest, error) {

	if patch == nil {
		return nil, fmt.Errorf("missing profile patch")
	}

	fields := &profilepb.ProfileFields{}
	paths := make([]string, 0)

	if patch.Name != nil {
		fields.Name = patch.Name
		paths = append(paths, "name")
	}

	if patch.BirthDate != nil {
		date := patch.BirthDate
		fields.BirthDate = &profilepb.Date{Year: int32(date.Year()), Month: int32(date.Month()), Day: int32(date.Day())}
		paths = append(paths, "birth_date")
	}

	if patch.Sex != nil {
		value := string(*patch.Sex)
		fields.Sex = &value
		paths = append(paths, "sex")
	}

	if patch.DatingGoal != nil {
		value := string(*patch.DatingGoal)
		fields.DatingGoal = &value
		paths = append(paths, "dating_goal")
	}

	if patch.AboutMe.Set {
		paths = append(paths, "about_me")

		if patch.AboutMe.Value != nil {
			value := string(*patch.AboutMe.Value)
			fields.AboutMe = &value
		}
	}

	if patch.Education.Set {
		paths = append(paths, "education")

		if patch.Education.Value != nil {
			value := string(*patch.Education.Value)
			fields.Education = &value
		}
	}

	if patch.Work.Set {
		paths = append(paths, "work")

		if patch.Work.Value != nil {
			value := string(*patch.Work.Value)
			fields.Work = &value
		}
	}

	if patch.Smoking.Set {
		paths = append(paths, "smoking")

		if patch.Smoking.Value != nil {
			value := string(*patch.Smoking.Value)
			fields.Smoking = &value
		}
	}

	if patch.Alcohol.Set {
		paths = append(paths, "alcohol")

		if patch.Alcohol.Value != nil {
			value := string(*patch.Alcohol.Value)
			fields.Alcohol = &value
		}
	}

	if patch.Height.Set {
		paths = append(paths, "height")

		if patch.Height.Value != nil {
			value := int32(*patch.Height.Value)
			fields.Height = &value
		}
	}

	if patch.Tags != nil {
		fields.Tags = append([]string{}, (*patch.Tags)...)
		paths = append(paths, "tags")
	}

	return &profilepb.UpdateProfileRequest{
		UserId:     userID,
		Fields:     fields,
		UpdateMask: &fieldmaskpb.FieldMask{Paths: paths},
	}, nil
}
