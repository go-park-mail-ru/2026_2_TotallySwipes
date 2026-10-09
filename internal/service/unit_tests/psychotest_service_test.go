package service

import (
	"context"
	"dating-app/internal/model"
	"dating-app/internal/psychotest"
	. "dating-app/internal/service"
	"errors"
	"math"
	"reflect"
	"testing"
)

func TestClassifyPersonality(t *testing.T) {
	for _, tc := range []struct {
		name   string
		vector [5]float64
		want   model.PersonalityType
	}{
		{"near strategist", [5]float64{.85, .8, .25, .55, .2}, model.PersonalityStrategist},
		{"low N", [5]float64{.2, .8, .2, .8, .2}, model.PersonalityAnchor},
		{"high N changes type", [5]float64{.2, .8, .2, .8, .8}, model.PersonalityKeeper},
		{"low A changes type", [5]float64{.2, .8, .2, .2, .2}, model.PersonalityCraftsperson},
	} {
		t.Run(tc.name, func(t *testing.T) {
			kind, about, err := (&PsychoTestService{}).ClassifyPersonality(personalityTestVector(tc.vector))
			if err != nil || kind != tc.want || about == "" {
				t.Fatalf("got %s %q %v", kind, about, err)
			}
		})
	}
	for _, bad := range []float64{-0.1, 1.1, math.NaN(), math.Inf(1), math.Inf(-1)} {
		for field := 0; field < 5; field++ {
			values := [5]float64{.5, .5, .5, .5, .5}
			values[field] = bad
			_, _, err := (&PsychoTestService{}).ClassifyPersonality(personalityTestVector(values))
			if !errors.Is(err, model.ErrInvalidBigFive) {
				t.Fatalf("invalid coordinate accepted: %v", values)
			}
		}
	}
	_, _, err := (&PsychoTestService{}).ClassifyPersonality(model.BigFive{})
	if !errors.Is(err, model.ErrInvalidBigFive) {
		t.Fatal("missing scores accepted")
	}
}

type resultProfileStub struct {
	ProfileService
	profile *model.Profile
}

func (s resultProfileStub) GetByUserIDCurrentProfile(context.Context, int64) (*model.Profile, error) {
	return s.profile, nil
}

type resultRepositoryStub struct {
	saved  *model.ProfilePsychoInput
	stored *model.TestResult
}

func (s *resultRepositoryStub) SaveTestResult(_ context.Context, _ int64, _ *model.TestAnswers, psycho *model.ProfilePsychoInput) (*model.TestResult, error) {
	s.saved = psycho
	return &model.TestResult{ID: 9, TestID: 42, Revision: 2}, nil
}
func (s *resultRepositoryStub) GetTestResult(context.Context, int64) (*model.TestResult, error) {
	if s.stored == nil {
		return nil, model.ErrNotFound
	}
	stored := *s.stored
	return &stored, nil
}

func TestSubmissionReplacesPreviousResult(t *testing.T) {
	repo := &resultRepositoryStub{}
	definition := tipiTest(t)
	answers := &model.TestAnswers{TestID: definition.ID}
	values := [10]int{5, 2, 3, 4, 7, 2, 6, 7, 6, 1}
	for i, value := range values {
		answers.Answers = append(answers.Answers, model.Answer{QuestionID: int64(i + 1), Value: value})
	}
	zero := 0.0
	profiles := resultProfileStub{profile: &model.Profile{ID: 7, CurrentPsycho: &model.ProfilePsycho{
		Openness: &zero, Conscientiousness: &zero, Extraversion: &zero, Agreeableness: &zero, Neuroticism: &zero,
	}}}
	calc := NewCompatibilityService()
	want, err := calc.CalculateBigFive(&definition, answers)
	if err != nil {
		t.Fatal(err)
	}
	kind, about, err := (&PsychoTestService{}).ClassifyPersonality(*want)
	if err != nil {
		t.Fatal(err)
	}
	got, err := NewPsychoTestService(repo, profiles, calc, definition).SubmitTestAnswers(context.Background(), 1, answers)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got.BigFive, *want) || got.PersonalityType != kind || got.AboutPersonalityType != about {
		t.Fatalf("response does not describe the current attempt: %+v", got)
	}
	saved := model.BigFive{Openness: repo.saved.Openness, Conscientiousness: repo.saved.Conscientiousness,
		Extraversion: repo.saved.Extraversion, Agreeableness: repo.saved.Agreeableness, Neuroticism: repo.saved.Neuroticism}
	if !reflect.DeepEqual(saved, *want) {
		t.Fatalf("saved profile must equal the attempt, got %+v", saved)
	}
	if got.TestID != definition.ID {
		t.Fatalf("test_id = %d, want %d", got.TestID, definition.ID)
	}
	if repo.saved.PersonalityType != kind {
		t.Fatalf("saved personality type = %q, want %q", repo.saved.PersonalityType, kind)
	}
}

func TestGetMyTestResultReturnsStoredResult(t *testing.T) {
	scores := personalityTestVector([5]float64{.2, .8, .2, .8, .8})
	repo := &resultRepositoryStub{stored: &model.TestResult{ID: 9, Revision: 3, BigFive: scores, PersonalityType: model.PersonalityKeeper}}
	profiles := resultProfileStub{profile: &model.Profile{ID: 7}}

	got, err := NewPsychoTestService(repo, profiles, nil, tipiTest(t)).GetMyTestResult(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}
	want := *repo.stored
	want.TestID = 1
	want.AboutPersonalityType = model.PersonalityKeeper.Description()
	if !reflect.DeepEqual(*got, want) {
		t.Fatalf("got %+v, want %+v", *got, want)
	}
}

func TestGetMyTestResultNotFound(t *testing.T) {
	profiles := resultProfileStub{profile: &model.Profile{ID: 7}}
	_, err := NewPsychoTestService(&resultRepositoryStub{}, profiles, nil, tipiTest(t)).GetMyTestResult(context.Background(), 1)
	if !errors.Is(err, model.ErrTestResultNotFound) {
		t.Fatalf("err = %v, want ErrTestResultNotFound", err)
	}
}

func personalityTestVector(values [5]float64) model.BigFive {
	return model.BigFive{
		Openness: &values[0], Conscientiousness: &values[1], Extraversion: &values[2],
		Agreeableness: &values[3], Neuroticism: &values[4],
	}
}

func tipiTest(t *testing.T) model.Test {
	t.Helper()
	test, err := psychotest.Load()
	if err != nil {
		t.Fatal(err)
	}
	return test
}
