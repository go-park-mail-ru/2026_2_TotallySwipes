package client

import (
	"context"
	matchpb "dating-app/gen/go/match/v1"
	"dating-app/internal/model"
	"fmt"
)

type FeedAdapter struct {
	client matchpb.MatchServiceClient
}

func NewFeedAdapter(client matchpb.MatchServiceClient) *FeedAdapter {
	return &FeedAdapter{client: client}
}

func (a *FeedAdapter) GetNextFeed(ctx context.Context, userID int64, limit int, cursor *int64) (*model.FeedPage, error) {

	l := int32(limit)
	response, err := a.client.GetNextFeed(ctx, &matchpb.GetNextFeedRequest{UserId: userID, Limit: &l, Cursor: cursor})

	if err != nil {
		return nil, err
	}

	feedPage, err := feedPageFromProto(response)

	if err != nil {
		return nil, err
	}

	return feedPage, nil
}

func feedPageFromProto(response *matchpb.GetNextFeedResponse) (*model.FeedPage, error) {

	if response == nil {
		return nil, fmt.Errorf("proto: response missing")
	}

	page := &model.FeedPage{
		Items:      make([]model.FeedItem, 0, len(response.GetItems())),
		NextCursor: response.NextCursor,
	}

	for _, i := range response.GetItems() {

		if i == nil {
			return nil, fmt.Errorf("proto: item missing")
		}

		item, err := feedItemFromProto(i)

		if err != nil {
			return nil, err
		}

		page.Items = append(page.Items, item)
	}

	return page, nil
}

func feedItemFromProto(i *matchpb.FeedItem) (model.FeedItem, error) {

	if i == nil {
		return model.FeedItem{}, fmt.Errorf("proto: missing feed item")
	}

	item := model.FeedItem{
		UserID:        i.GetUserId(),
		Name:          i.GetName(),
		Age:           int(i.GetAge()),
		DatingGoal:    model.DatingGoal(i.GetDatingGoal()),
		Compatibility: i.Compatibility,
		AboutMe:       i.AboutMe,
		Work:          i.Work,
		Tags:          append([]string{}, i.GetTags()...),
		Photos:        make([]model.FeedPhoto, 0, len(i.GetPhotos())),
		LikedMe:       i.GetLikedMe(),
		City:          i.City,
	}

	if i.Education != nil {
		education := model.Education(i.GetEducation())
		item.Education = &education
	}

	if i.Smoking != nil {
		smoking := model.Attitude(i.GetSmoking())
		item.Smoking = &smoking
	}

	if i.Alcohol != nil {
		alcohol := model.Attitude(i.GetAlcohol())
		item.Alcohol = &alcohol
	}

	if i.Height != nil {
		height := int(i.GetHeight())
		item.Height = &height
	}

	for _, photo := range i.GetPhotos() {

		if photo == nil {
			return model.FeedItem{}, fmt.Errorf("proto: no photo %d", i.GetUserId())
		}

		item.Photos = append(item.Photos, model.FeedPhoto{
			ID:  photo.GetId(),
			URL: photo.GetUrl(),
		})
	}

	return item, nil
}
