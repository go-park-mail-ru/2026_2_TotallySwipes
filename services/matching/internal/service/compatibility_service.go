package service

import (
	"dating-app/services/matching/internal/model"
	"fmt"
	"math"
)

type CompatibilityService interface {
	// CalculateDistance возвращает совместимость в [0,1] или model.ErrInvalidBigFive.
	CalculateDistance(candidate1 model.BigFive, candidate2 model.BigFive) (float64, error)
}

type CompatibilityServiceImpl struct{}

func NewCompatibilityService() *CompatibilityServiceImpl {

	return &CompatibilityServiceImpl{}
}

// difference вычисляет среднее абсолютное различие координат.
func difference(v1 []float64, v2 []float64) float64 {

	res := float64(0.0)

	for idx := range v1 {
		res += math.Abs(v1[idx] - v2[idx])
	}

	return res / float64(len(v1))
}

// CalculateDistance - совместимость в [0,1]: единица минус среднее абсолютное различие координат
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

	for i := range v1 {
		if !isValid(v1[i]) {
			return 0, fmt.Errorf("candidate1: value at index %d must be in [0,1]: %w", i, model.ErrInvalidBigFive)
		}

		if !isValid(v2[i]) {
			return 0, fmt.Errorf("candidate2: value at index %d must be in [0,1]: %w", i, model.ErrInvalidBigFive)
		}
	}

	return 1 - difference(v1, v2), nil
}

func isValid(v float64) bool {

	return !math.IsNaN(v) && v >= 0 && v <= 1
}
