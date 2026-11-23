package dto

import (
	"dating-app/internal/model"
	"time"
)

type CreateSearchRequest struct {
	City string `json:"city"`
}

type SearchLikedEventsRequest struct {
	LikedEventsIDs []int64 `json:"liked_event_ids"`
}

type SearchStateResponse struct {
	SearchID                 string             `json:"search_id"`
	ExpiresAt                time.Time          `json:"expires_at"`
	City                     string             `json:"city"`
	Status                   model.SearchStatus `json:"status"`
	Events                   []EventResponse    `json:"events"`
	CurrentUserLikedEventIDs []int64            `json:"current_user_liked_event_ids"`
	CurrentUserFinished      bool               `json:"current_user_finished"`
	PartnerFinished          bool               `json:"partner_finished"`
	Matches                  []EventResponse    `json:"matches"`
}

type EventResponse struct {
	ID         int64                   `json:"id"`
	Name       string                  `json:"name"`
	StartsAt   time.Time               `json:"starts_at"`
	ImageURL   *string                 `json:"image_url"`
	URL        string                  `json:"url"`
	Location   EventLocationResponse   `json:"location"`
	Categories []EventCategoryResponse `json:"categories"`
}

type EventLocationResponse struct {
	City    string  `json:"city"`
	Address *string `json:"address"`
}

type EventCategoryResponse struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

func NewSearchStateResponse(state *model.SearchEventsState) SearchStateResponse {
	var likes []int64
	if state.CurrentUserFinished {
		likes = append([]int64{}, state.CurrentUserLikedEventIDs...)
	}
	return SearchStateResponse{
		SearchID:                 state.SearchID.String(),
		ExpiresAt:                state.ExpiresAt,
		City:                     state.City,
		Status:                   state.Status,
		Events:                   eventResponses(state.Events),
		CurrentUserLikedEventIDs: likes,
		CurrentUserFinished:      state.CurrentUserFinished,
		PartnerFinished:          state.PartnerFinished,
		Matches:                  eventResponses(state.Matches),
	}
}

func eventResponses(events []model.Event) []EventResponse {
	result := make([]EventResponse, 0, len(events))
	for _, event := range events {
		categories := make([]EventCategoryResponse, 0, len(event.Categories))
		for _, category := range event.Categories {
			categories = append(categories, EventCategoryResponse{ID: category.ID, Name: category.Name})
		}
		result = append(result, EventResponse{
			ID:         event.ID,
			Name:       event.Name,
			StartsAt:   event.StartsAt,
			ImageURL:   event.ImageURL,
			URL:        event.URL,
			Location:   EventLocationResponse{City: event.Location.City, Address: event.Location.Address},
			Categories: categories,
		})
	}
	return result
}
