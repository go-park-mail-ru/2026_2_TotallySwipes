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

type Education string

const (
	EducationSecondary        Education = "secondary"
	EducationVocational       Education = "vocational"
	EducationIncompleteHigher Education = "incomplete_higher"
	EducationHigher           Education = "higher"
	EducationDegree           Education = "degree"
)

var Educations = []Education{EducationSecondary, EducationVocational, EducationIncompleteHigher, EducationHigher, EducationDegree}

// Attitude - отношение к курению или алкоголю
type Attitude string

const (
	AttitudeNegative Attitude = "negative"
	AttitudeNeutral  Attitude = "neutral"
	AttitudePositive Attitude = "positive"
)

var Attitudes = []Attitude{AttitudeNegative, AttitudeNeutral, AttitudePositive}

const (
	MinHeight = 100
	MaxHeight = 250
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
	ID              int64
	UserID          int64
	CreatedAt       time.Time
	UpdatedAt       time.Time
	CurrentVersion  ProfileVersion
	CurrentPsycho   *ProfilePsycho
	HasSearchFilter bool

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
	Name       *string
	BirthDate  *time.Time
	Sex        *Sex
	DatingGoal *DatingGoal
	AboutMe    *string
	Education  *Education
	Work       *string
	Smoking    *Attitude
	Alcohol    *Attitude
	Height     *int
}

type ProfileVersionInput struct {
	ProfileFields
	Tags []Tag
}

// Optional - необязательное поле патча: Set=false - не менять, Value=nil - очистить
type Optional[T any] struct {
	Set   bool
	Value *T
}

func (o Optional[T]) applyTo(dst **T) {
	if o.Set {
		*dst = o.Value
	}
}

// ProfilePatch - частичное изменение анкеты: nil или Set=false - не менять
type ProfilePatch struct {
	Name       *string
	BirthDate  *time.Time
	Sex        *Sex
	DatingGoal *DatingGoal
	AboutMe    Optional[string]
	Education  Optional[Education]
	Work       Optional[string]
	Smoking    Optional[Attitude]
	Alcohol    Optional[Attitude]
	Height     Optional[int]
	Tags       *[]string
}

func (p ProfilePatch) IsEmpty() bool {
	return p.Name == nil && p.BirthDate == nil && p.Sex == nil && p.DatingGoal == nil &&
		!p.AboutMe.Set && !p.Education.Set && !p.Work.Set && !p.Smoking.Set && !p.Alcohol.Set &&
		!p.Height.Set && p.Tags == nil
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
	p.AboutMe.applyTo(&f.AboutMe)
	p.Education.applyTo(&f.Education)
	p.Work.applyTo(&f.Work)
	p.Smoking.applyTo(&f.Smoking)
	p.Alcohol.applyTo(&f.Alcohol)
	p.Height.applyTo(&f.Height)
	return f
}

// Названия обязательных полей анкеты в API
const (
	MissingName         = "name"
	MissingBirthDate    = "birth_date"
	MissingSex          = "sex"
	MissingDatingGoal   = "dating_goal"
	MissingSearchFilter = "search_filter"
	MissingPhotos       = "photos"
)

// Missing возвращает незаполненные обязательные поля в порядке онбординга; пусто - анкета попадает в ленту
func (p *Profile) Missing() []string {
	f := p.CurrentVersion.ProfileFields
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
	if !p.HasSearchFilter {
		missing = append(missing, MissingSearchFilter)
	}
	if len(p.Photos) == 0 {
		missing = append(missing, MissingPhotos)
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
