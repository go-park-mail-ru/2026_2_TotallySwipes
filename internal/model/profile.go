package model

import "time"

type DatingGoal string

const (
	DatingGoalRelationship DatingGoal = "relationship"
	DatingGoalFriendship   DatingGoal = "friendship"
	DatingGoalCasual       DatingGoal = "casual"
)

type Sex string

const (
	SexMale   Sex = "male"
	SexFemale Sex = "female"
)

type SearchSex string

const (
	SearchSexMale   SearchSex = "male"
	SearchSexFemale SearchSex = "female"
	SearchSexAll    SearchSex = "all"
)

type Tag struct {
	ID   int64
	Name string
}

type PhotoInput struct {
	StorageKey string
	Position   int
}

type Photo struct {
	ID         int64
	StorageKey string
	Position   int
}

type Profile struct {
	ID             int64
	UserID         int64
	CreatedAt      time.Time
	UpdatedAt      time.Time
	CurrentVersion ProfileVersion
	CurrentPsycho  *ProfilePsycho

	Tags   []Tag
	Photos []Photo
}

type ProfileInput struct {
	UserID         int64
	CurrentVersion ProfileVersionInput
	CurrentPsycho  *ProfilePsychoInput
}

type ProfileVersion struct {
	ID            int64
	ProfileID     int64
	BirthDate     time.Time
	DatingGoal    DatingGoal
	AboutMe       string
	RecordedAt    time.Time
	Sex           Sex
	SearchSex     SearchSex
	SearchAgeFrom int
	SearchAgeTo   int
}

type ProfileVersionInput struct {
	BirthDate     time.Time
	DatingGoal    DatingGoal
	AboutMe       string
	Sex           Sex
	SearchSex     SearchSex
	SearchAgeFrom int
	SearchAgeTo   int
}

type ProfilePsycho struct {
	ID                int64
	ProfileID         int64
	TestID            int64
	RecordedAt        time.Time
	Openness          *float64
	Conscientiousness *float64
	Extraversion      *float64
	Agreeableness     *float64
	Neuroticism       *float64
}

type ProfilePsychoInput struct {
	TestID            int64
	Openness          *float64
	Conscientiousness *float64
	Extraversion      *float64
	Agreeableness     *float64
	Neuroticism       *float64
}
