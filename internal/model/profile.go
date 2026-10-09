package model

import "time"

type DatingGoal string

const (
	DatingGoalRelationship DatingGoal = "relationship"
	DatingGoalFriendship   DatingGoal = "friendship"
	DatingGoalCasual       DatingGoal = "casual"
)

var DatingGoals = []DatingGoal{DatingGoalRelationship, DatingGoalFriendship, DatingGoalCasual}

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

const MaxPhotos = 6

type PhotoUpload struct {
	Data []byte
	Ext  string
}

// Photo - фото анкеты; Position с 1, первая - главная, URL заполняет сервис
type Photo struct {
	ID         int64
	StorageKey string
	Position   int
	URL        string
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

// ProfileVersion - снимок анкеты; до первого изменения версии нет (ID == 0)
type ProfileVersion struct {
	ID         int64
	ProfileID  int64
	RecordedAt time.Time
	ProfileFields
}

type ProfileFields struct {
	Name          *string
	BirthDate     *time.Time
	Sex           *Sex
	DatingGoal    *DatingGoal
	AboutMe       *string
	SearchSex     *SearchSex
	SearchAgeFrom *int
	SearchAgeTo   *int
}

type ProfileVersionInput struct {
	ProfileFields
	Tags []Tag
}

// ProfilePatch - частичное изменение анкеты: nil - не менять, AboutMeSet отличает очистку about_me
type ProfilePatch struct {
	Name          *string
	BirthDate     *time.Time
	Sex           *Sex
	DatingGoal    *DatingGoal
	AboutMeSet    bool
	AboutMe       *string
	SearchSex     *SearchSex
	SearchAgeFrom *int
	SearchAgeTo   *int
	Tags          *[]string
}

func (p ProfilePatch) IsEmpty() bool {
	return p.Name == nil && p.BirthDate == nil && p.Sex == nil && p.DatingGoal == nil &&
		!p.AboutMeSet && p.SearchSex == nil && p.SearchAgeFrom == nil && p.SearchAgeTo == nil && p.Tags == nil
}

// Apply накладывает патч на поля анкеты. Теги патча обрабатывает вызывающий код
func (p ProfilePatch) Apply(f ProfileFields) ProfileFields {
	if p.Name != nil {
		f.Name = p.Name
	}
	if p.BirthDate != nil {
		f.BirthDate = p.BirthDate
	}
	if p.Sex != nil {
		f.Sex = p.Sex
	}
	if p.DatingGoal != nil {
		f.DatingGoal = p.DatingGoal
	}
	if p.AboutMeSet {
		f.AboutMe = p.AboutMe
	}
	if p.SearchSex != nil {
		f.SearchSex = p.SearchSex
	}
	if p.SearchAgeFrom != nil {
		f.SearchAgeFrom = p.SearchAgeFrom
	}
	if p.SearchAgeTo != nil {
		f.SearchAgeTo = p.SearchAgeTo
	}
	return f
}

// Названия обязательных полей анкеты в API
const (
	MissingName       = "name"
	MissingBirthDate  = "birth_date"
	MissingSex        = "sex"
	MissingDatingGoal = "dating_goal"
	MissingSearchSex  = "search_sex"
	MissingSearchAge  = "search_age"
	MissingPhotos     = "photos"
)

// Missing возвращает незаполненные обязательные поля в порядке онбординга; пусто - анкета попадает в ленту
func (p *Profile) Missing() []string {
	missing := missingFields(p.CurrentVersion.ProfileFields)
	if len(p.Photos) == 0 {
		missing = append(missing, MissingPhotos)
	}
	return missing
}

func missingFields(f ProfileFields) []string {
	missing := make([]string, 0)
	if f.Name == nil {
		missing = append(missing, MissingName)
	}
	if f.BirthDate == nil {
		missing = append(missing, MissingBirthDate)
	}
	if f.Sex == nil {
		missing = append(missing, MissingSex)
	}
	if f.DatingGoal == nil {
		missing = append(missing, MissingDatingGoal)
	}
	if f.SearchSex == nil {
		missing = append(missing, MissingSearchSex)
	}
	if f.SearchAgeFrom == nil || f.SearchAgeTo == nil {
		missing = append(missing, MissingSearchAge)
	}
	return missing
}

// AgeAt - число полных лет на дату now
func AgeAt(birthDate, now time.Time) int {
	age := now.Year() - birthDate.Year()
	if now.Month() < birthDate.Month() || (now.Month() == birthDate.Month() && now.Day() < birthDate.Day()) {
		age--
	}
	return age
}

type ProfilePsycho struct {
	ID                int64
	ProfileID         int64
	RecordedAt        time.Time
	Openness          *float64
	Conscientiousness *float64
	Extraversion      *float64
	Agreeableness     *float64
	Neuroticism       *float64
}

type BigFive struct {
	Openness          *float64
	Conscientiousness *float64
	Extraversion      *float64
	Agreeableness     *float64
	Neuroticism       *float64
}

type ProfilePsychoInput struct {
	PersonalityType   PersonalityType
	Openness          *float64
	Conscientiousness *float64
	Extraversion      *float64
	Agreeableness     *float64
	Neuroticism       *float64
}

type ProfileShort struct {
	UserID       int64
	Name         *string
	Age          *int
	MainPhotoURL *string
	Missing      []string
}
