package service

import (
	"dating-app/internal/model"
	. "dating-app/internal/service"
	"math"
	"testing"
)

func TestCalculateBigFiveTIPI(t *testing.T) {
	type testCase struct {
		name    string
		values  [10]int
		want    [5]float64 // OCEAN
		mutate  func(*model.Test, *model.TestAnswers)
		wantErr bool
	}
	neutral := [10]int{4, 4, 4, 4, 4, 4, 4, 4, 4, 4}
	tests := []testCase{
		{name: "neutral", values: neutral, want: [5]float64{.5, .5, .5, .5, .5}},
		{name: "maximum", values: [10]int{7, 1, 7, 7, 7, 1, 7, 1, 1, 1}, want: [5]float64{1, 1, 1, 1, 1}},
		{name: "minimum", values: [10]int{1, 7, 1, 1, 1, 7, 1, 7, 7, 7}},
		{name: "distinct traits", values: [10]int{5, 2, 3, 4, 7, 2, 6, 7, 6, 1}, want: [5]float64{1, 1.0 / 6, .75, 5.0 / 6, 1.0 / 3}},
		{name: "unsupported", values: neutral, wantErr: true, mutate: func(d *model.Test, a *model.TestAnswers) { d.Methodology = model.MethodologyMiniIPIP }},
		{name: "missing methodology", values: neutral, wantErr: true, mutate: func(d *model.Test, a *model.TestAnswers) { d.Methodology = "" }},
		{name: "wrong test", values: neutral, wantErr: true, mutate: func(d *model.Test, a *model.TestAnswers) { a.TestID++ }},
		{name: "missing answer", values: neutral, wantErr: true, mutate: func(d *model.Test, a *model.TestAnswers) { a.Answers = a.Answers[:9] }},
		{name: "duplicate answer", values: neutral, wantErr: true, mutate: func(d *model.Test, a *model.TestAnswers) { a.Answers[0] = a.Answers[1] }},
		{name: "unknown question", values: neutral, wantErr: true, mutate: func(d *model.Test, a *model.TestAnswers) { a.Answers[0].QuestionID = 999 }},
		{name: "zero answer", values: neutral, wantErr: true, mutate: func(d *model.Test, a *model.TestAnswers) { a.Answers[0].Value = 0 }},
		{name: "above scale", values: neutral, wantErr: true, mutate: func(d *model.Test, a *model.TestAnswers) { a.Answers[0].Value = 8 }},
		{name: "bad operation", values: neutral, wantErr: true, mutate: func(d *model.Test, a *model.TestAnswers) { d.Questions[0].OperationID = 100 }},
		{name: "duplicate operation", values: neutral, wantErr: true, mutate: func(d *model.Test, a *model.TestAnswers) { d.Questions[0].OperationID = d.Questions[1].OperationID }},
		{name: "duplicate question", values: neutral, wantErr: true, mutate: func(d *model.Test, a *model.TestAnswers) { d.Questions[0].ID = d.Questions[1].ID }},
		{name: "bad scale", values: neutral, wantErr: true, mutate: func(d *model.Test, a *model.TestAnswers) { d.AnswerOptions[0].Value = 7 }},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			definition := model.Test{ID: 42, Methodology: model.MethodologyTIPI}
			answers := model.TestAnswers{TestID: 42}
			for i := 1; i <= 7; i++ {
				definition.AnswerOptions = append(definition.AnswerOptions, model.AnswerOption{Value: i})
			}
			for i := 0; i < 10; i++ {
				id := int64(100 + i*3)
				definition.Questions = append(definition.Questions, model.Question{ID: id, OperationID: int64(i + 1)})
				answers.Answers = append(answers.Answers, model.Answer{QuestionID: id, Value: tc.values[i]})
			}
			// Neither database IDs nor answer ordering determine the scoring operation.
			answers.Answers[0], answers.Answers[9] = answers.Answers[9], answers.Answers[0]
			definition.Questions[2], definition.Questions[5] = definition.Questions[5], definition.Questions[2]
			if tc.mutate != nil {
				tc.mutate(&definition, &answers)
			}
			svc := &CompatibilityServiceImpl{}
			got, err := svc.CalculateBigFive(&definition, &answers)
			if tc.wantErr {
				if err == nil || got != nil {
					t.Fatalf("got=%v err=%v", got, err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			for i, v := range []*float64{got.Openness, got.Conscientiousness, got.Extraversion, got.Agreeableness, got.Neuroticism} {
				if v == nil || math.IsNaN(*v) || math.Abs(*v-tc.want[i]) > 1e-12 {
					t.Fatalf("trait %d=%v, want %v", i, v, tc.want[i])
				}
			}
		})
	}
}
