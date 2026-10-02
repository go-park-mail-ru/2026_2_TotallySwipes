package service

import (
	"context"
	"dating-app/internal/model"
	"errors"
	"fmt"
	"math"
)

type TestService interface {
	GetCurrentTest(ctx context.Context) (*model.Test, error)
	SubmitTestAnswers(ctx context.Context, userID int64, answers *model.TestAnswers) (*model.TestResult, error)
}

type TestRepository interface {
	GetCurrentTest(ctx context.Context) (*model.Test, error)
	SaveTestResult(ctx context.Context, profileID int64, answers *model.TestAnswers, psycho *model.ProfilePsychoInput) (*model.TestResult, error)
}

type TestServiceImpl struct {
	compatibilitySvc CompatibilityService
	profileSvc       ProfileService

	testRepo TestRepository
}

func NewTestService(repo TestRepository, profiles ProfileService, compatibility CompatibilityService) *TestServiceImpl {
	return &TestServiceImpl{testRepo: repo, profileSvc: profiles, compatibilitySvc: compatibility}
}

func (s *TestServiceImpl) GetCurrentTest(ctx context.Context) (*model.Test, error) {
	test, err := s.testRepo.GetCurrentTest(ctx)
	if errors.Is(err, model.ErrNotFound) {
		return nil, fmt.Errorf("get current test: %w", model.ErrActiveTestNotFound)
	}

	if err != nil {
		return nil, fmt.Errorf("get current test: %w", err)

	}
	return test, nil
}

func (s *TestServiceImpl) SubmitTestAnswers(ctx context.Context, userID int64, answers *model.TestAnswers) (*model.TestResult, error) {

	if answers == nil || answers.TestID <= 0 {
		return nil, fmt.Errorf("submit test answers: invalid input: %w", model.ErrInvalidTestRequest)
	}

	profile, err := s.profileSvc.GetByUserIDCurrentProfile(ctx, userID)
	if errors.Is(err, model.ErrNotFound) {
		return nil, fmt.Errorf("submit test answers: get profile: %w", model.ErrProfileRequired)
	}

	if err != nil {
		return nil, fmt.Errorf("submit test answers: get profile: %w", err)
	}

	if profile == nil {
		return nil, fmt.Errorf("submit test answers: profile service returned nil")
	}

	profileID := profile.ID
	test, err := s.testRepo.GetCurrentTest(ctx)
	if errors.Is(err, model.ErrNotFound) {
		return nil, fmt.Errorf("submit test answers: find requested test: %w", model.ErrTestNotFound)
	}

	if err != nil {
		return nil, fmt.Errorf("submit test answers: get current test: %w", err)
	}

	if test == nil {
		return nil, fmt.Errorf("submit test answers: repository returned nil test")
	}

	if test.ID != answers.TestID {
		return nil, fmt.Errorf("submit test answers: find requested test: %w", model.ErrTestNotFound)
	}

	bigFive, err := s.compatibilitySvc.CalculateBigFive(test, answers)
	if err != nil {
		return nil, fmt.Errorf("submit test answers: calculate: %w", err)
	}

	// Keep the current attempt's normalized scores separate from the blended profile.
	if bigFive == nil {
		return nil, fmt.Errorf("submit test answers: missing calculated vector: %w", model.ErrInvalidBigFive)
	}
	personalityType, aboutPersonalityType, err := s.classifyPersonality(*bigFive)
	if err != nil {
		return nil, err
	}
	coordinates := [5]float64{*bigFive.Openness, *bigFive.Conscientiousness, *bigFive.Extraversion, *bigFive.Agreeableness, *bigFive.Neuroticism}
	attemptBigFive := model.BigFive{
		Openness: &coordinates[0], Conscientiousness: &coordinates[1],
		Extraversion: &coordinates[2], Agreeableness: &coordinates[3], Neuroticism: &coordinates[4],
	}

	prevBigFive, exists := feedBigFive(profile.CurrentPsycho)
	if exists {
		bigFive, err = s.compatibilitySvc.UpdateBigFive(prevBigFive, bigFive)

		if err != nil {
			return nil, fmt.Errorf("submit test answers: update indicators: %w", err)
		}
	}

	psychoInput := &model.ProfilePsychoInput{
		TestID:            answers.TestID,
		Openness:          bigFive.Openness,
		Conscientiousness: bigFive.Conscientiousness,
		Extraversion:      bigFive.Extraversion,
		Agreeableness:     bigFive.Agreeableness,
		Neuroticism:       bigFive.Neuroticism,
	}

	result, err := s.testRepo.SaveTestResult(ctx, profileID, answers, psychoInput)

	if err != nil {
		return nil, fmt.Errorf("submit test answers: save result: %w", err)
	}

	if result == nil {
		return nil, fmt.Errorf("submit test answers: repository returned nil result")
	}
	result.BigFive = attemptBigFive
	result.PersonalityType = personalityType
	result.AboutPersonalityType = aboutPersonalityType
	return result, nil
}

type personalityPrototype struct {
	kind        model.PersonalityType
	coordinates [5]float64
}

var personalityPrototypes = [...]personalityPrototype{
	{model.PersonalityExplorer, [5]float64{0.8, 0.2, 0.8, 0.5, 0.2}},
	{model.PersonalityVisionary, [5]float64{0.8, 0.5, 0.8, 0.5, 0.8}},
	{model.PersonalityStrategist, [5]float64{0.8, 0.8, 0.2, 0.5, 0.2}},
	{model.PersonalityInventor, [5]float64{0.8, 0.2, 0.2, 0.5, 0.2}},
	{model.PersonalityDreamer, [5]float64{0.8, 0.2, 0.2, 0.8, 0.8}},
	{model.PersonalityCurator, [5]float64{0.8, 0.8, 0.2, 0.8, 0.8}},
	{model.PersonalityInspirer, [5]float64{0.8, 0.5, 0.8, 0.8, 0.2}},
	{model.PersonalityDebater, [5]float64{0.8, 0.8, 0.8, 0.2, 0.5}},
	{model.PersonalityOrganizer, [5]float64{0.2, 0.8, 0.8, 0.5, 0.2}},
	{model.PersonalityConnector, [5]float64{0.5, 0.5, 0.8, 0.8, 0.8}},
	{model.PersonalityCompanion, [5]float64{0.2, 0.2, 0.8, 0.8, 0.2}},
	{model.PersonalityDriver, [5]float64{0.5, 0.2, 0.8, 0.2, 0.8}},
	{model.PersonalityAnchor, [5]float64{0.2, 0.8, 0.2, 0.8, 0.2}},
	{model.PersonalityCraftsperson, [5]float64{0.2, 0.8, 0.2, 0.2, 0.2}},
	{model.PersonalityObserver, [5]float64{0.5, 0.5, 0.2, 0.2, 0.8}},
	{model.PersonalityKeeper, [5]float64{0.2, 0.8, 0.2, 0.8, 0.8}},
}

func (s *TestServiceImpl) classifyPersonality(vector model.BigFive) (model.PersonalityType, string, error) {

	fields := [5]*float64{vector.Openness, vector.Conscientiousness, vector.Extraversion, vector.Agreeableness, vector.Neuroticism}
	var values [5]float64

	for i, field := range fields {
		if field == nil || !isValid(*field) {
			return "", "", fmt.Errorf("classify personality: %w", model.ErrInvalidBigFive)
		}

		values[i] = *field
	}

	closest := 0
	minimum := math.Inf(1)
	for i, prototype := range personalityPrototypes {
		distance := 0.0

		for j, value := range values {
			delta := value - prototype.coordinates[j]
			distance += delta * delta
		}

		if distance < minimum {
			closest, minimum = i, distance
		}
	}

	prototype := personalityPrototypes[closest]
	return prototype.kind, prototype.kind.Description(), nil
}
