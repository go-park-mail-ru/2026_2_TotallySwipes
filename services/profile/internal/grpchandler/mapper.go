package grpchandler

import (
	"fmt"
	"strconv"
	"time"

	profilepb "dating-app/gen/go/profile/v1"
	"dating-app/internal/model"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func invalidArgument(format string, args ...any) error {
	return status.Errorf(codes.InvalidArgument, format, args...)
}

func requireUserID(userID int64) error {
	if userID <= 0 {
		return invalidArgument("user_id must be positive")
	}
	return nil
}

func stringPtr[T ~string](v *T) *string {
	if v == nil {
		return nil
	}
	s := string(*v)
	return &s
}

func int32Ptr(v *int) *int32 {
	if v == nil {
		return nil
	}
	i := int32(*v)
	return &i
}

func dateToProto(t *time.Time) *profilepb.Date {
	if t == nil {
		return nil
	}
	return &profilepb.Date{Year: int32(t.Year()), Month: int32(t.Month()), Day: int32(t.Day())}
}

func dateFromProto(d *profilepb.Date) (time.Time, error) {
	year, month, day := int(d.GetYear()), time.Month(d.GetMonth()), int(d.GetDay())
	t := time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
	if year < 1 || year > 9999 || t.Year() != year || t.Month() != month || t.Day() != day {
		return time.Time{}, invalidArgument("invalid date %04d-%02d-%02d", year, month, day)
	}
	return t, nil
}

func photosToProto(photos []model.Photo) []*profilepb.Photo {
	result := make([]*profilepb.Photo, 0, len(photos))
	for _, p := range photos {
		result = append(result, &profilepb.Photo{Id: p.ID, Url: p.URL})
	}
	return result
}

func profileToProto(p *model.Profile, now time.Time) *profilepb.Profile {
	f := p.CurrentVersion.ProfileFields
	result := &profilepb.Profile{
		UserId:     p.UserID,
		ProfileId:  p.ID,
		Name:       f.Name,
		BirthDate:  dateToProto(f.BirthDate),
		Sex:        stringPtr(f.Sex),
		DatingGoal: stringPtr(f.DatingGoal),
		AboutMe:    f.AboutMe,
		Education:  stringPtr(f.Education),
		Work:       f.Work,
		Smoking:    stringPtr(f.Smoking),
		Alcohol:    stringPtr(f.Alcohol),
		Height:     int32Ptr(f.Height),
		Tags:       make([]string, 0, len(p.Tags)),
		Photos:     photosToProto(p.Photos),
		Missing:    p.Missing(),
		City:       p.City,
	}
	if f.BirthDate != nil {
		age := int32(model.AgeAt(*f.BirthDate, now))
		result.Age = &age
	}
	for _, tag := range p.Tags {
		result.Tags = append(result.Tags, tag.Name)
	}
	return result
}

func psychoToProto(p *model.ProfilePsycho) *profilepb.BigFive {
	if p == nil {
		return nil
	}
	return &profilepb.BigFive{
		Openness:          p.Openness,
		Conscientiousness: p.Conscientiousness,
		Extraversion:      p.Extraversion,
		Agreeableness:     p.Agreeableness,
		Neuroticism:       p.Neuroticism,
	}
}

func bigFiveToProto(b model.BigFive) *profilepb.BigFive {
	return &profilepb.BigFive{
		Openness:          b.Openness,
		Conscientiousness: b.Conscientiousness,
		Extraversion:      b.Extraversion,
		Agreeableness:     b.Agreeableness,
		Neuroticism:       b.Neuroticism,
	}
}

func profileWithBigFiveToProto(p *model.Profile, now time.Time) *profilepb.ProfileWithBigFive {
	return &profilepb.ProfileWithBigFive{Profile: profileToProto(p, now), BigFive: psychoToProto(p.CurrentPsycho)}
}

func shortProfileToProto(p *model.ProfileShort) *profilepb.GetShortProfileResponse {
	return &profilepb.GetShortProfileResponse{
		UserId:   p.UserID,
		Name:     p.Name,
		Age:      int32Ptr(p.Age),
		PhotoUrl: p.MainPhotoURL,
		Missing:  p.Missing,
		City:     p.City,
	}
}

func optionalFrom[T ~string](v *string) model.Optional[T] {
	if v == nil {
		return model.Optional[T]{Set: true}
	}
	value := T(*v)
	return model.Optional[T]{Set: true, Value: &value}
}

// patchFromProto собирает патч анкеты из полей и маски: путь в маске означает «изменить поле»,
// отсутствующее значение необязательного поля - «очистить»; обязательные поля очистить нельзя.
func patchFromProto(fields *profilepb.ProfileFields, mask *fieldmaskpb.FieldMask) (*model.ProfilePatch, error) {
	if fields == nil {
		fields = &profilepb.ProfileFields{}
	}
	patch := &model.ProfilePatch{}

	for _, path := range mask.GetPaths() {
		switch path {
		case "name":
			if fields.Name == nil {
				return nil, invalidArgument("name cannot be cleared")
			}
			name := fields.GetName()
			patch.Name = &name
		case "birth_date":
			if fields.BirthDate == nil {
				return nil, invalidArgument("birth_date cannot be cleared")
			}
			date, err := dateFromProto(fields.GetBirthDate())
			if err != nil {
				return nil, err
			}
			patch.BirthDate = &date
		case "sex":
			if fields.Sex == nil {
				return nil, invalidArgument("sex cannot be cleared")
			}
			sex := model.Sex(fields.GetSex())
			patch.Sex = &sex
		case "dating_goal":
			if fields.DatingGoal == nil {
				return nil, invalidArgument("dating_goal cannot be cleared")
			}
			goal := model.DatingGoal(fields.GetDatingGoal())
			patch.DatingGoal = &goal
		case "about_me":
			patch.AboutMe = optionalFrom[string](fields.AboutMe)
		case "education":
			patch.Education = optionalFrom[model.Education](fields.Education)
		case "work":
			patch.Work = optionalFrom[string](fields.Work)
		case "smoking":
			patch.Smoking = optionalFrom[model.Attitude](fields.Smoking)
		case "alcohol":
			patch.Alcohol = optionalFrom[model.Attitude](fields.Alcohol)
		case "height":
			patch.Height = model.Optional[int]{Set: true}
			if fields.Height != nil {
				height := int(fields.GetHeight())
				patch.Height.Value = &height
			}
		case "tags":
			tags := append([]string{}, fields.GetTags()...)
			patch.Tags = &tags
		default:
			return nil, invalidArgument("unknown update_mask path %q", path)
		}
	}
	return patch, nil
}

func testToProto(t *model.Test) *profilepb.Test {
	result := &profilepb.Test{
		TestId:        t.ID,
		Title:         t.Title,
		Instructions:  t.Instructions,
		AnswerOptions: make([]*profilepb.TestAnswerOption, 0, len(t.AnswerOptions)),
		Questions:     make([]*profilepb.TestQuestion, 0, len(t.Questions)),
	}
	for _, o := range t.AnswerOptions {
		result.AnswerOptions = append(result.AnswerOptions, &profilepb.TestAnswerOption{Value: int32(o.Value), Label: o.Label})
	}
	for _, q := range t.Questions {
		result.Questions = append(result.Questions, &profilepb.TestQuestion{Id: q.ID, Body: q.Body})
	}
	return result
}

func testAnswersFromProto(req *profilepb.SubmitTestAnswersRequest) (*model.TestAnswers, error) {
	answers := &model.TestAnswers{TestID: req.GetTestId(), Answers: make([]model.Answer, 0, len(req.GetAnswers()))}
	for _, a := range req.GetAnswers() {
		if a == nil || a.Value == nil {
			return nil, fmt.Errorf("answer without value: %w", model.ErrInvalidAnswers)
		}
		answers.Answers = append(answers.Answers, model.Answer{QuestionID: a.GetQuestionId(), Value: int(a.GetValue())})
	}
	return answers, nil
}

func testResultToProto(r *model.TestResult) *profilepb.TestResult {
	return &profilepb.TestResult{
		ResultId:             r.ID,
		TestId:               r.TestID,
		Revision:             int32(r.Revision),
		CompletedAt:          timestamppb.New(r.CompletedA),
		BigFive:              bigFiveToProto(r.BigFive),
		PersonalityType:      string(r.PersonalityType),
		AboutPersonalityType: r.AboutPersonalityType,
	}
}

func coordinatesFromProto(c *profilepb.Coordinates) (*model.Coordinates, error) {
	if c == nil || c.Latitude == nil || c.Longitude == nil {
		return nil, fmt.Errorf("missing coordinates: %w", model.ErrInvalidCoordinates)
	}
	return &model.Coordinates{Latitude: c.GetLatitude(), Longitude: c.GetLongitude()}, nil
}

func locationToProto(l *model.UserLocation) *profilepb.Location {
	if l == nil {
		return nil
	}
	lat, lng := l.Coordinates.Latitude, l.Coordinates.Longitude
	return &profilepb.Location{
		City:        l.City,
		District:    l.District,
		Coordinates: &profilepb.Coordinates{Latitude: &lat, Longitude: &lng},
	}
}

func searchFilterToProto(f *model.SearchFilter) *profilepb.SearchFilter {
	sex, ageFrom, ageTo := string(f.Sex), int32(f.AgeFrom), int32(f.AgeTo)
	return &profilepb.SearchFilter{Sex: &sex, AgeFrom: &ageFrom, AgeTo: &ageTo}
}

func searchFilterFromProto(f *profilepb.SearchFilter) (model.SearchFilter, error) {
	if f == nil || f.Sex == nil || f.AgeFrom == nil || f.AgeTo == nil {
		return model.SearchFilter{}, invalidArgument("filter.sex, filter.age_from and filter.age_to are required")
	}
	return model.SearchFilter{Sex: model.SearchSex(f.GetSex()), AgeFrom: int(f.GetAgeFrom()), AgeTo: int(f.GetAgeTo())}, nil
}

// profilesFilterFromProto переводит фильтр списка мэтчей; пол "all" и пустые строки не ограничивают выборку.
func profilesFilterFromProto(f *profilepb.MatchFilter) *model.ProfilesFilter {
	if f == nil {
		return nil
	}
	result := &model.ProfilesFilter{}
	if f.Name != nil && f.GetName() != "" {
		name := f.GetName()
		result.Name = &name
	}
	if f.AgeFrom != nil {
		v := int(f.GetAgeFrom())
		result.AgeFrom = &v
	}
	if f.AgeTo != nil {
		v := int(f.GetAgeTo())
		result.AgeTo = &v
	}
	if f.Sex != nil && f.GetSex() != "" && f.GetSex() != string(model.SearchSexAll) {
		sex := model.Sex(f.GetSex())
		result.Sex = &sex
	}
	return result
}

func cursorFromProto(cursor *string) (*int64, error) {
	if cursor == nil || *cursor == "" {
		return nil, nil
	}
	v, err := strconv.ParseInt(*cursor, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("cursor %q: %w", *cursor, model.ErrInvalidFeedRequest)
	}
	return &v, nil
}

func cursorToProto(cursor *int64) *string {
	if cursor == nil {
		return nil
	}
	s := strconv.FormatInt(*cursor, 10)
	return &s
}
