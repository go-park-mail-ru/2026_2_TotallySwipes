package model

import "time"

type DatingGoal string
type Education string
type Attitude string

// Profile — снимок данных Profile Service для чтения; Matching не изменяет анкеты.
type Profile struct {
	ID         int64
	UserID     int64
	Name       *string
	BirthDate  *time.Time
	Age        *int
	Sex        *string
	DatingGoal *DatingGoal
	AboutMe    *string
	Education  *Education
	Work       *string
	Smoking    *Attitude
	Alcohol    *Attitude
	Height     *int
	Tags       []string
	Photos     []FeedPhoto
	City       *string
	Missing    []string
	Psycho     BigFive
}

type BigFive struct {
	Openness          *float64
	Conscientiousness *float64
	Extraversion      *float64
	Agreeableness     *float64
	Neuroticism       *float64
}

type MatchFilter struct {
	Name    *string
	AgeFrom *int
	AgeTo   *int
	Sex     *string
}

func (f MatchFilter) IsEmpty() bool {

	return f.Name == nil && f.AgeFrom == nil && f.AgeTo == nil && (f.Sex == nil || *f.Sex == "all")
}

type SwipeState struct {
	Swiped  bool
	LikedMe bool
}

func AgeAt(birthDate, now time.Time) int {

	age := now.Year() - birthDate.Year()

	if now.Month() < birthDate.Month() || (now.Month() == birthDate.Month() && now.Day() < birthDate.Day()) {
		age--
	}

	return age
}
