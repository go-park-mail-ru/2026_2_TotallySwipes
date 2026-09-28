package service

import (
	"context"
	"dating-app/internal/model"
	"errors"
	"fmt"
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

func (s *TestServiceImpl) GetCurrentTest(ctx context.Context) (*model.Test, error) {
	test, err := s.testRepo.GetCurrentTest(ctx)
	if errors.Is(err, model.ErrNotFound) {
		return nil, ErrActiveTestNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get current test: %w", err)
	}
	return test, nil
}

func (s *TestServiceImpl) SubmitTestAnswers(ctx context.Context, userID int64, answers *model.TestAnswers) (*model.TestResult, error) {

	if answers == nil || answers.TestID <= 0 {
		return nil, ErrInvalidTestRequest
	}

	profile, err := s.profileSvc.GetByUserIDCurrentProfile(ctx, userID)
	if errors.Is(err, model.ErrNotFound) {
		return nil, ErrProfileRequired
	}

	if err != nil {
		return nil, fmt.Errorf("SubmitTestAnswers : %w", err)
	}

	if profile == nil {
		return nil, fmt.Errorf("submit test answers: profile service returned nil")
	}

	profileID := profile.ID
	test, err := s.testRepo.GetCurrentTest(ctx)
	if errors.Is(err, model.ErrNotFound) {
		return nil, ErrTestNotFound
	}

	if err != nil {
		return nil, fmt.Errorf("SubmitTestAnswers : %w", err)
	}

	if test == nil {
		return nil, fmt.Errorf("submit test answers: repository returned nil test")
	}

	if test.ID != answers.TestID {
		return nil, ErrTestNotFound
	}

	bigFive, err := s.compatibilitySvc.CalculateBigFive(test, answers)
	if err != nil {
		return nil, fmt.Errorf("submit test answers: calculate: %w", err)
	}

	prevBigFive, exists := feedBigFive(profile.CurrentPsycho)
	if exists {
		bigFive, err = s.compatibilitySvc.UpdateBigFive(prevBigFive, bigFive)

		if err != nil {
			return nil, fmt.Errorf("submit test answers: update indicators: %w", err)
		}
	}

	if err != nil {
		return nil, fmt.Errorf("SubmitTestAnswers : %w", err)
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

	return result, nil
}
