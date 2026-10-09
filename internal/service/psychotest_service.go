package service

import (
	"context"
	"dating-app/internal/model"
	"errors"
	"fmt"
	"math"
)

type PsychoTestRepository interface {
	// SaveTestResult сохраняет ответы и новую ревизию психопрофиля; ID теста и Big Five в результате не заполняет.
	SaveTestResult(ctx context.Context, profileID int64, answers *model.TestAnswers, psycho *model.ProfilePsychoInput) (*model.TestResult, error)
	// GetTestResult возвращает последний завершённый результат или model.ErrNotFound.
	GetTestResult(ctx context.Context, profileID int64) (*model.TestResult, error)
}

// ProfileReader - откуда сервис теста берёт анкету пользователя
type ProfileReader interface {
	// GetByUserIDCurrentProfile возвращает анкету или model.ErrNotFound.
	GetByUserIDCurrentProfile(ctx context.Context, userID int64) (*model.Profile, error)
}

type PsychoTestService struct {
	compatibilitySvc CompatibilityService
	profiles         ProfileReader

	repo PsychoTestRepository
	test model.Test
}

func NewPsychoTestService(repo PsychoTestRepository, profiles ProfileReader, compatibility CompatibilityService, test model.Test) *PsychoTestService {
	return &PsychoTestService{repo: repo, profiles: profiles, compatibilitySvc: compatibility, test: test}
}

// GetCurrentTest получает текущий тест с вопросами и вариантами ответов.
func (s *PsychoTestService) GetCurrentTest(_ context.Context) (*model.Test, error) {
	return s.currentTest(), nil
}

// currentTest возвращает копию теста, чтобы вызывающий код не мог изменить общее описание.
func (s *PsychoTestService) currentTest() *model.Test {
	test := s.test
	test.AnswerOptions = append([]model.AnswerOption(nil), s.test.AnswerOptions...)
	test.Questions = append([]model.Question(nil), s.test.Questions...)
	return &test
}

// SubmitTestAnswers рассчитывает результат попытки и сохраняет его вместе с ответами как новый психопрофиль.
func (s *PsychoTestService) SubmitTestAnswers(ctx context.Context, userID int64, answers *model.TestAnswers) (*model.TestResult, error) {

	if answers == nil || answers.TestID <= 0 {
		return nil, fmt.Errorf("submit test answers: invalid input: %w", model.ErrInvalidTestRequest)
	}

	profile, err := s.profiles.GetByUserIDCurrentProfile(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("submit test answers: get profile: %w", profileRequired(err))
	}

	if profile == nil {
		return nil, fmt.Errorf("submit test answers: profile service returned nil")
	}

	profileID := profile.ID
	test := s.currentTest()
	if test.ID != answers.TestID {
		return nil, fmt.Errorf("submit test answers: find requested test: %w", model.ErrTestNotFound)
	}

	bigFive, err := s.compatibilitySvc.CalculateBigFive(test, answers)
	if err != nil {
		return nil, fmt.Errorf("submit test answers: calculate: %w", err)
	}

	if bigFive == nil {
		return nil, fmt.Errorf("submit test answers: missing calculated vector: %w", model.ErrInvalidBigFive)
	}
	personalityType, aboutPersonalityType, err := s.ClassifyPersonality(*bigFive)
	if err != nil {
		return nil, err
	}
	psychoInput := &model.ProfilePsychoInput{
		PersonalityType:   personalityType,
		Openness:          bigFive.Openness,
		Conscientiousness: bigFive.Conscientiousness,
		Extraversion:      bigFive.Extraversion,
		Agreeableness:     bigFive.Agreeableness,
		Neuroticism:       bigFive.Neuroticism,
	}

	result, err := s.repo.SaveTestResult(ctx, profileID, answers, psychoInput)

	if err != nil {
		return nil, fmt.Errorf("submit test answers: save result: %w", err)
	}

	if result == nil {
		return nil, fmt.Errorf("submit test answers: repository returned nil result")
	}
	result.TestID = test.ID
	result.BigFive = *bigFive
	result.PersonalityType = personalityType
	result.AboutPersonalityType = aboutPersonalityType
	return result, nil
}

// GetMyTestResult получает последний сохранённый результат и добавляет описание типа личности.
func (s *PsychoTestService) GetMyTestResult(ctx context.Context, userID int64) (*model.TestResult, error) {
	profile, err := s.profiles.GetByUserIDCurrentProfile(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("get my test result: get profile: %w", profileRequired(err))
	}

	if profile == nil {
		return nil, fmt.Errorf("get my test result: profile service returned nil")
	}

	result, err := s.repo.GetTestResult(ctx, profile.ID)
	if errors.Is(err, model.ErrNotFound) {
		return nil, fmt.Errorf("get my test result: %w", model.ErrTestResultNotFound)
	}

	if err != nil {
		return nil, fmt.Errorf("get my test result: %w", err)
	}

	if result == nil {
		return nil, fmt.Errorf("get my test result: repository returned nil result")
	}

	result.TestID = s.test.ID
	result.AboutPersonalityType = result.PersonalityType.Description()
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

// ClassifyPersonality выбирает ближайший прототип личности по квадрату евклидова расстояния.
func (s *PsychoTestService) ClassifyPersonality(vector model.BigFive) (model.PersonalityType, string, error) {

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
