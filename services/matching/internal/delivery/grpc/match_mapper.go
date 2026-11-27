package grpc

import (
	profilepb "dating-app/gen/go/profile/v1"
	matchpb "dating-app/services/matching/api/gen/go/match/v1"
	"dating-app/services/matching/internal/model"
	"google.golang.org/protobuf/types/known/timestamppb"
	"strconv"
)

func optionalInt32(value *int) *int32 {

	if value == nil {
		return nil
	}

	result := int32(*value)
	return &result
}

func optionalString[T ~string](value *T) *string {

	if value == nil {
		return nil
	}

	result := string(*value)
	return &result
}

func matchToProto(item model.Match) *matchpb.MatchItem {

	result := &matchpb.MatchItem{
		MatchId:         item.ID,
		UserId:          item.UserID,
		Name:            item.Name,
		Age:             optionalInt32(item.Age),
		PhotoUrl:        item.MainPhotoURL,
		Compatibility:   item.Compatibility,
		IsNew:           item.IsNew,
		PartnerFinished: item.PartnerFinished,
		CanMeetup:       item.CanMeetup,
		CreatedAt:       timestamppb.New(item.CreatedAt),
	}

	if item.ChatID != nil {

		chatID := strconv.FormatInt(*item.ChatID, 10)
		result.ChatId = &chatID
	}

	return result
}

func feedItemToProto(item model.FeedItem) *matchpb.FeedItem {

	result := &matchpb.FeedItem{UserId: item.UserID, Name: item.Name, Age: int32(item.Age), DatingGoal: string(item.DatingGoal), Compatibility: item.Compatibility, AboutMe: item.AboutMe, Education: optionalString(item.Education), Work: item.Work, Smoking: optionalString(item.Smoking), Alcohol: optionalString(item.Alcohol), Height: optionalInt32(item.Height), Tags: item.Tags, Photos: make([]*profilepb.Photo, 0, len(item.Photos)), LikedMe: item.LikedMe, City: item.City}

	for _, photo := range item.Photos {
		result.Photos = append(result.Photos, &profilepb.Photo{Id: photo.ID, Url: photo.URL})
	}

	return result
}

func searchStateToProto(state *model.SearchEventsState) *matchpb.SearchEventsState {

	if state == nil {
		return nil
	}

	result := &matchpb.SearchEventsState{SearchId: state.SearchID.String(), ExpiresAt: timestamppb.New(state.ExpiresAt), City: state.City, CurrentUserLikedEventIds: state.CurrentUserLikedEventIDs, CurrentUserFinished: state.CurrentUserFinished, PartnerFinished: state.PartnerFinished, Events: eventsToProto(state.Events), Matches: eventsToProto(state.Matches)}

	switch state.Status {
	case model.SearchStatusVoting:

		result.Status = matchpb.SearchStatus_SEARCH_STATUS_VOTING

	case model.SearchStatusWaitingPartner:

		result.Status = matchpb.SearchStatus_SEARCH_STATUS_WAITING_PARTNER

	case model.SearchStatusCompleted:

		result.Status = matchpb.SearchStatus_SEARCH_STATUS_COMPLETED

	case model.SearchStatusCancelled:

		result.Status = matchpb.SearchStatus_SEARCH_STATUS_CANCELLED
	}

	return result
}

func eventsToProto(events []model.Event) []*matchpb.Event {

	result := make([]*matchpb.Event, 0, len(events))

	for _, event := range events {
		item := &matchpb.Event{Id: event.ID, Name: event.Name, StartsAt: timestamppb.New(event.StartsAt), ImageUrl: event.ImageURL, Url: event.URL, Location: &matchpb.EventLocation{City: event.Location.City, Address: event.Location.Address}, Categories: make([]*matchpb.EventCategory, 0, len(event.Categories))}

		for _, category := range event.Categories {
			item.Categories = append(item.Categories, &matchpb.EventCategory{Id: category.ID, Name: category.Name})
		}

		result = append(result, item)
	}

	return result
}
