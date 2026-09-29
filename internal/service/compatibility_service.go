package service

import (
	"dating-app/internal/model"
	"fmt"
	"math"
	"slices"
)

type CompatibilityService interface {
	CalculateDistance(candidate1 model.BigFive, candidate2 model.BigFive) (float64, error)

	CalculateBigFive(test *model.Test, answers *model.TestAnswers) (*model.BigFive, error)

	UpdateBigFive(current *model.BigFive, incoming *model.BigFive) (*model.BigFive, error)
}

type CompatibilityServiceImpl struct {
	alpha float64
}

func normalizeVector(vector []float64) ([]float64, error) {

	max := slices.Max(vector)
	min := slices.Min(vector)

	if math.IsNaN(min) || math.IsNaN(max) || math.IsInf(min, 0) || math.IsInf(max, 0) || max <= min {
		return nil, fmt.Errorf("invalid scale bounds: %w", model.ErrInvalidBigFive)
	}

	result := make([]float64, len(vector))
	for idx, el := range vector {
		result[idx] = (el - min) / (max - min)
	}

	return result, nil
}

func difference(v1 []float64, v2 []float64) float64 {
	res := float64(0.0)
	for idx := range v1 {
		res += math.Abs(v1[idx] - v2[idx])
	}

	return res / float64(len(v1))
}

func (s *CompatibilityServiceImpl) CalculateDistance(candidate1 model.BigFive, candidate2 model.BigFive) (float64, error) {

	if candidate1.Agreeableness == nil ||
		candidate1.Conscientiousness == nil ||
		candidate1.Extraversion == nil ||
		candidate1.Neuroticism == nil ||
		candidate1.Openness == nil {
		return 0, fmt.Errorf("candidate1: incomplete Big Five: %w", model.ErrInvalidBigFive)
	}

	if candidate2.Agreeableness == nil ||
		candidate2.Conscientiousness == nil ||
		candidate2.Extraversion == nil ||
		candidate2.Neuroticism == nil ||
		candidate2.Openness == nil {
		return 0, fmt.Errorf("candidate2: incomplete Big Five: %w", model.ErrInvalidBigFive)
	}

	v1 := []float64{*candidate1.Agreeableness, *candidate1.Conscientiousness, *candidate1.Extraversion, *candidate1.Neuroticism, *candidate1.Openness}
	v2 := []float64{*candidate2.Agreeableness, *candidate2.Conscientiousness, *candidate2.Extraversion, *candidate2.Neuroticism, *candidate2.Openness}
	vector1, err := normalizeVector(v1)

	if err != nil {
		return 0, fmt.Errorf("normalize first candidate: %w", err)
	}

	vector2, err := normalizeVector(v2)

	if err != nil {
		return 0, fmt.Errorf("normalize second candidate: %w", err)
	}

	return 1 - difference(vector1, vector2), nil
}

const (
	TIPIExtraversionDirect       int64 = 1
	TIPIAgreeablenessReverse     int64 = 2
	TIPIConscientiousnessDirect  int64 = 3
	TIPINeuroticismDirect        int64 = 4
	TIPIOpennessDirect           int64 = 5
	TIPIExtraversionReverse      int64 = 6
	TIPIAgreeablenessDirect      int64 = 7
	TIPIConscientiousnessReverse int64 = 8
	TIPINeuroticismReverse       int64 = 9
	TIPIOpennessReverse          int64 = 10
)

func (s *CompatibilityServiceImpl) CalculateBigFive(test *model.Test, answers *model.TestAnswers) (*model.BigFive, error) {
	if test == nil || answers == nil {
		return nil, fmt.Errorf("test and answers are required: %w", model.ErrInvalidTestDefinition)
	}
	switch test.Methodology {
	case model.MethodologyTIPI:
		result, err := calculateTIPI(*test, *answers)
		if err != nil {
			return nil, fmt.Errorf("calculate TIPI: %w", err)
		}
		return result, nil
	default:
		return nil, fmt.Errorf("unsupported test methodology: %q: %w", test.Methodology, model.ErrInvalidTestDefinition)
	}
}

func score(values map[int64]float64, direct, reverse int64) *float64 {
	value := ((values[direct]+8-values[reverse])/2 - 1) / 6
	return &value
}

func calculateTIPI(test model.Test, answers model.TestAnswers) (*model.BigFive, error) {

	if answers.TestID != test.ID {
		return nil, fmt.Errorf("test ID mismatch or invalid test ID: %w", model.ErrInvalidTestDefinition)
	}

	if len(test.Questions) != 10 {
		return nil, fmt.Errorf("TIPI requires exactly 10 questions: %w", model.ErrInvalidTestDefinition)
	}
	if len(answers.Answers) != 10 {
		return nil, fmt.Errorf("%w: TIPI requires exactly 10 answers", model.ErrInvalidAnswers)
	}

	if len(test.AnswerOptions) != 7 {
		return nil, fmt.Errorf("TIPI requires answer options 1 through 7: %w", model.ErrInvalidTestDefinition)
	}

	options := make(map[int]bool, 7)
	for _, option := range test.AnswerOptions {
		if option.Value < 1 || option.Value > 7 || options[option.Value] {
			return nil, fmt.Errorf("invalid TIPI answer options: %w", model.ErrInvalidTestDefinition)
		}
		options[option.Value] = true
	}

	operations := make(map[int64]int64, 10)
	seenOperations := make(map[int64]bool, 10)
	for _, question := range test.Questions {

		if operations[question.ID] != 0 {
			return nil, fmt.Errorf("invalid or duplicate question ID: %d: %w", question.ID, model.ErrInvalidTestDefinition)
		}

		if question.OperationID > 10 || seenOperations[question.OperationID] {
			return nil, fmt.Errorf("invalid or duplicate TIPI operation: %d: %w", question.OperationID, model.ErrInvalidTestDefinition)
		}
		operations[question.ID] = question.OperationID
		seenOperations[question.OperationID] = true
	}

	values := make(map[int64]float64, 10)
	seenAnswers := make(map[int64]bool, 10)
	for _, answer := range answers.Answers {
		operation, exists := operations[answer.QuestionID]

		if !exists || seenAnswers[answer.QuestionID] {
			return nil, fmt.Errorf("%w: unknown or duplicate question: %d", model.ErrInvalidAnswers, answer.QuestionID)
		}

		if !options[answer.Value] {
			return nil, fmt.Errorf("%w: answer for question %d must be in [1,7]", model.ErrInvalidAnswers, answer.QuestionID)
		}
		seenAnswers[answer.QuestionID] = true
		values[operation] = float64(answer.Value)
	}

	return &model.BigFive{
		Openness:          score(values, TIPIOpennessDirect, TIPIOpennessReverse),
		Conscientiousness: score(values, TIPIConscientiousnessDirect, TIPIConscientiousnessReverse),
		Extraversion:      score(values, TIPIExtraversionDirect, TIPIExtraversionReverse),
		Agreeableness:     score(values, TIPIAgreeablenessDirect, TIPIAgreeablenessReverse),
		Neuroticism:       score(values, TIPINeuroticismDirect, TIPINeuroticismReverse),
	}, nil
}

func blend(current, incoming, alpha float64) float64 {
	return (1-alpha)*current + alpha*incoming
}

func isValid(v float64) bool {
	return !math.IsNaN(v) && v >= 0 && v <= 1
}

func (s *CompatibilityServiceImpl) UpdateBigFive(current *model.BigFive, incoming *model.BigFive) (*model.BigFive, error) {

	if incoming == nil {
		return nil, fmt.Errorf("incoming Big Five is required: %w", model.ErrInvalidBigFive)
	}
	if current == nil {
		current = &model.BigFive{}
	}
	if math.IsNaN(s.alpha) || s.alpha < 0 || s.alpha > 1 {
		return nil, fmt.Errorf("alpha must be in [0,1]")
	}

	result := &model.BigFive{}

	fields := []struct {
		name     string
		current  *float64
		incoming *float64
		result   **float64
	}{
		{"openness", current.Openness, incoming.Openness, &result.Openness},
		{"conscientiousness", current.Conscientiousness, incoming.Conscientiousness, &result.Conscientiousness},
		{"extraversion", current.Extraversion, incoming.Extraversion, &result.Extraversion},
		{"agreeableness", current.Agreeableness, incoming.Agreeableness, &result.Agreeableness},
		{"neuroticism", current.Neuroticism, incoming.Neuroticism, &result.Neuroticism},
	}

	for _, field := range fields {
		if field.incoming == nil || !isValid(*field.incoming) {
			return nil, fmt.Errorf("%s: invalid incoming value: %w", field.name, model.ErrInvalidBigFive)
		}

		value := *field.incoming
		if field.current != nil {
			if !isValid(*field.current) {
				return nil, fmt.Errorf("%s: invalid current value: %w", field.name, model.ErrInvalidBigFive)
			}

			value = blend(*field.current, *field.incoming, s.alpha)
		}

		*field.result = &value
	}

	return result, nil
}
