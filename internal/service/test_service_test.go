package service

import (
	"context"
	"errors"
	"math"
	"reflect"
	"testing"

	"dating-app/internal/model"
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
			kind, about, err := (&TestServiceImpl{}).classifyPersonality(personalityTestVector(tc.vector))
			if err != nil || kind != tc.want || about == "" {
				t.Fatalf("got %s %q %v", kind, about, err)
			}
		})
	}
	for _, bad := range []float64{-0.1, 1.1, math.NaN(), math.Inf(1), math.Inf(-1)} {
		for field := 0; field < 5; field++ {
			values := [5]float64{.5, .5, .5, .5, .5}
			values[field] = bad
			_, _, err := (&TestServiceImpl{}).classifyPersonality(personalityTestVector(values))
			if !errors.Is(err, model.ErrInvalidBigFive) {
				t.Fatalf("invalid coordinate accepted: %v", values)
			}
		}
	}
	_, _, err := (&TestServiceImpl{}).classifyPersonality(model.BigFive{})
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
	definition model.Test
	saved      *model.ProfilePsychoInput
}

func (s *resultRepositoryStub) GetCurrentTest(context.Context) (*model.Test, error) {
	return &s.definition, nil
}
func (s *resultRepositoryStub) SaveTestResult(_ context.Context, _ int64, _ *model.TestAnswers, psycho *model.ProfilePsychoInput) (*model.TestResult, error) {
	s.saved = psycho
	return &model.TestResult{ID: 9, TestID: 42, Revision: 2}, nil
}

func TestSubmissionReturnsAttemptVectorBeforeProfileBlending(t *testing.T) {
	repo := &resultRepositoryStub{definition: model.NewTIPITest(42)}
	answers := &model.TestAnswers{TestID: 42}
	values := [10]int{5, 2, 3, 4, 7, 2, 6, 7, 6, 1}
	for i, value := range values {
		id := int64(i + 1)
		repo.definition.Questions = append(repo.definition.Questions, model.Question{ID: id, OperationID: id})
		answers.Answers = append(answers.Answers, model.Answer{QuestionID: id, Value: value})
	}
	zero := 0.0
	profiles := resultProfileStub{profile: &model.Profile{ID: 7, CurrentPsycho: &model.ProfilePsycho{
		Openness: &zero, Conscientiousness: &zero, Extraversion: &zero, Agreeableness: &zero, Neuroticism: &zero,
	}}}
	calc := NewCompatibilityService(.5)
	want, err := calc.CalculateBigFive(&repo.definition, answers)
	if err != nil {
		t.Fatal(err)
	}
	kind, about, err := (&TestServiceImpl{}).classifyPersonality(*want)
	if err != nil {
		t.Fatal(err)
	}
	got, err := NewTestService(repo, profiles, calc).SubmitTestAnswers(context.Background(), 1, answers)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got.BigFive, *want) || got.PersonalityType != kind || got.AboutPersonalityType != about {
		t.Fatalf("response does not describe the current attempt: %+v", got)
	}
	if repo.saved == nil || *repo.saved.Openness != .5 || *got.BigFive.Openness != 1 {
		t.Fatal("saved blended profile and attempt result must remain separate")
	}
}

func personalityTestVector(values [5]float64) model.BigFive {
	return model.BigFive{
		Openness: &values[0], Conscientiousness: &values[1], Extraversion: &values[2],
		Agreeableness: &values[3], Neuroticism: &values[4],
	}
}
