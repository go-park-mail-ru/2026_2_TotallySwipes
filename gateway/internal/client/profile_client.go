package client

import (
	"context"
	"dating-app/gateway/internal/client/mapper"
	profilepb "dating-app/gen/go/profile/v1"
	"dating-app/internal/model"
	"fmt"
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

	return mapper.ShortProfileFromProto(response)
}

func (a *ProfileAdapter) GetMyProfile(ctx context.Context, userID int64) (*model.Profile, error) {

	response, err := a.client.GetMyProfile(ctx, &profilepb.GetMyProfileRequest{UserId: userID})

	if err != nil {
		return nil, err
	}

	return mapper.ProfileFromProto(response.GetProfile())
}

func (a *ProfileAdapter) UpdateProfile(ctx context.Context, userID int64, patch *model.ProfilePatch) (*model.Profile, error) {

	request, err := mapper.ProfilePatchToProto(userID, patch)

	if err != nil {
		return nil, err
	}

	response, err := a.client.UpdateProfile(ctx, request)

	if err != nil {
		return nil, err
	}

	return mapper.ProfileFromProto(response.GetProfile())
}

func (a *ProfileAdapter) AddPhoto(ctx context.Context, userID int64, upload model.PhotoUpload) ([]model.Photo, error) {

	response, err := a.client.AddPhoto(ctx, mapper.PhotoUploadToProto(userID, upload))

	if err != nil {
		return nil, err
	}

	if response == nil {
		return nil, fmt.Errorf("proto: missing add photo response")
	}

	return mapper.PhotosFromProto(response.GetPhotos())
}

func (a *ProfileAdapter) DeletePhoto(ctx context.Context, userID, photoID int64) ([]model.Photo, error) {

	response, err := a.client.DeletePhoto(ctx, &profilepb.DeletePhotoRequest{UserId: userID, PhotoId: photoID})

	if err != nil {
		return nil, err
	}

	if response == nil {
		return nil, fmt.Errorf("proto: missing delete photo response")
	}

	return mapper.PhotosFromProto(response.GetPhotos())
}

func (a *ProfileAdapter) ReorderPhotos(ctx context.Context, userID int64, photoIDs []int64) ([]model.Photo, error) {

	response, err := a.client.ReorderPhotos(ctx, &profilepb.ReorderPhotosRequest{UserId: userID, PhotoIds: photoIDs})

	if err != nil {
		return nil, err
	}

	if response == nil {
		return nil, fmt.Errorf("proto: missing reorder photos response")
	}

	return mapper.PhotosFromProto(response.GetPhotos())
}
