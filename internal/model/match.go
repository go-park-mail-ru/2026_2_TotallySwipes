package model

import "time"

const (
	MatchesDefaultLimit = 12
	MatchesMaxLimit     = 48
)

type MatchSummary struct {
	Total int
	New   int
}

type MatchPage struct {
	Items      []Match
	Total      int
	NextCursor *int64
}

type Match struct {
	ID            int64
	UserID        int64
	Name          *string
	BirthDate     *time.Time
	Age           *int
	PhotoKey      *string
	MainPhotoURL  *string
	Psycho        BigFive
	Compatibility *float64
	IsNew         bool
	CreatedAt     time.Time
}
