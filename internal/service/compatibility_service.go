package service

import (
	"dating-app/internal/model"
	"fmt"
	"math"
	"slices"
)

type CompatibilityService interface {
	Calculate(candidate1 model.BigFive, candidate2 model.BigFive) (float64, error)
}

type CompatibilityServiceImpl struct {
	coeffs float64
}

func normalizeVector(vector []float64) ([]float64, error) {

	max := slices.Max(vector)
	min := slices.Min(vector)

	if math.IsNaN(min) || math.IsNaN(max) || math.IsInf(min, 0) || math.IsInf(max, 0) || max <= min {
		return nil, fmt.Errorf("invalid scale bounds")
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

func (s *CompatibilityServiceImpl) Calculate(candidate1 model.BigFive, candidate2 model.BigFive) (float64, error) {

	if candidate1.Agreeableness == nil ||
		candidate1.Conscientiousness == nil ||
		candidate1.Extraversion == nil ||
		candidate1.Neuroticism == nil ||
		candidate1.Openness == nil {
		return 0, fmt.Errorf("candidate1: incomplete Big Five")
	}

	if candidate2.Agreeableness == nil ||
		candidate2.Conscientiousness == nil ||
		candidate2.Extraversion == nil ||
		candidate2.Neuroticism == nil ||
		candidate2.Openness == nil {
		return 0, fmt.Errorf("candidate2: incomplete Big Five")
	}

	v1 := []float64{*candidate1.Agreeableness, *candidate1.Conscientiousness, *candidate1.Extraversion, *candidate1.Neuroticism, *candidate1.Openness}
	v2 := []float64{*candidate2.Agreeableness, *candidate2.Conscientiousness, *candidate2.Extraversion, *candidate2.Neuroticism, *candidate2.Openness}
	vector1, err := normalizeVector(v1)

	if err != nil {
		return 0, err
	}

	vector2, err := normalizeVector(v2)

	if err != nil {
		return 0, err
	}

	return 1 - difference(vector1, vector2), nil
}
