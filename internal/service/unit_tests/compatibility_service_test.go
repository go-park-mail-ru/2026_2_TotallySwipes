package service

import (
	"dating-app/internal/model"
	. "dating-app/internal/service"
	"math"
	"testing"
)

func TestCompatibilityServiceCalculate(t *testing.T) {
	// Классы: совпадение, максимальное различие, промежуточное сходство,
	// неполные данные и нечисловые/бесконечные показатели.
	type testCase struct {
		name          string
		first, second model.BigFive
		want          float64
		wantErr       bool
	}
	tests := []testCase{
		{name: "identical profiles", first: bigFiveTestVector([5]float64{1, 2, 3, 4, 7}), second: bigFiveTestVector([5]float64{1, 2, 3, 4, 7}), want: 1},
		{name: "maximally different profiles", first: bigFiveTestVector([5]float64{1, 7, 1, 7, 1}), second: bigFiveTestVector([5]float64{7, 1, 7, 1, 7}), want: 0},
		{name: "intermediate similarity", first: bigFiveTestVector([5]float64{1, 2, 3, 4, 7}), second: bigFiveTestVector([5]float64{1, 2, 3, 7, 7}), want: 0.9},
		{name: "both tests missing", wantErr: true},
	}
	// Каждый nullable-показатель может отсутствовать у любого из участников.
	for participant := 0; participant < 2; participant++ {
		for field, name := range []string{"openness", "conscientiousness", "extraversion", "agreeableness", "neuroticism"} {
			pair := [2]model.BigFive{bigFiveTestVector([5]float64{1, 2, 3, 4, 7}), bigFiveTestVector([5]float64{1, 2, 3, 4, 7})}
			v := &pair[participant]
			fields := []**float64{&v.Openness, &v.Conscientiousness, &v.Extraversion, &v.Agreeableness, &v.Neuroticism}
			*fields[field] = nil
			side := []string{"first/", "second/"}[participant]
			tests = append(tests, testCase{name: side + "missing/" + name, first: pair[0], second: pair[1], wantErr: true})
		}
		for _, invalid := range []struct {
			name  string
			value float64
		}{
			{"NaN", math.NaN()}, {"positive infinity", math.Inf(1)}, {"negative infinity", math.Inf(-1)},
		} {
			pair := [2]model.BigFive{bigFiveTestVector([5]float64{1, 2, 3, 4, 7}), bigFiveTestVector([5]float64{1, 2, 3, 4, 7})}
			pair[participant].Extraversion = &invalid.value
			tests = append(tests, testCase{name: []string{"first/", "second/"}[participant] + invalid.name, first: pair[0], second: pair[1], wantErr: true})
		}
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			svc := &CompatibilityServiceImpl{}
			got, err := svc.CalculateDistance(tc.first, tc.second)
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if math.IsNaN(got) || math.Abs(got-tc.want) > 1e-12 {
				t.Fatalf("score=%v, want %v", got, tc.want)
			}
			reverse, err := svc.CalculateDistance(tc.second, tc.first)
			if err != nil || math.IsNaN(reverse) || math.Abs(reverse-got) > 1e-12 {
				t.Fatalf("asymmetric result: %v, error=%v", reverse, err)
			}
		})
	}
}

// OCEAN. Values are copied into separate addresses.
func bigFiveTestVector(values [5]float64) model.BigFive {
	return model.BigFive{Openness: &values[0], Conscientiousness: &values[1], Extraversion: &values[2], Agreeableness: &values[3], Neuroticism: &values[4]}
}
