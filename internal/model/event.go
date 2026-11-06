package model

import (
	"time"

	"github.com/google/uuid"
)

type SearchStatus string

const (
	SearchStatusVoting         SearchStatus = "voting"
	SearchStatusWaitingPartner SearchStatus = "waiting_partner"
	SearchStatusCompleted      SearchStatus = "completed"
	SearchStatusCancelled      SearchStatus = "cancelled"
)

type ParticipantResult struct {
	UserID         int64
	Submitted      bool
	LikedEventsIDs []int64
}

type PlaceSearch struct {
	SearchID uuid.UUID
	MatchID  int64
	City     string

	Events       []Event
	Participants [2]ParticipantResult
	Status       SearchStatus
	ExpiresAt    time.Time
}

type EventCategory struct {
	ID   int64
	Name string
}

type EventLocation struct {
	City    string
	Address *string
}

type Event struct {
	ID         int64
	Name       string
	ImageURL   *string
	URL        string
	StartsAt   time.Time
	Location   EventLocation
	Categories []EventCategory
}

type SearchEventsState struct {
	SearchID  uuid.UUID
	MatchID   int64
	City      string
	Events    []Event
	Status    SearchStatus
	ExpiresAt time.Time

	// CurrentUserLikedEventIDs is nil until the user's result is accepted.
	CurrentUserLikedEventIDs []int64
	CurrentUserFinished      bool
	PartnerFinished          bool
	Matches                  []Event
}

type EventQuery struct {
	City        *string
	Skip        *int
	Limit       *int
	CategoryIDs *[]int64
	StartsAtMin *time.Time
}

type EventPage struct {
	Total  int
	Events []Event
}
