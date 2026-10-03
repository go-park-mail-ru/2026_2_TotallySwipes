package dto

import (
	"dating-app/internal/model"
	"time"
)

type SubmitTestAnswersRequest struct {
	Answers []TestAnswerRequest `json:"answers"`
}

type TestAnswerRequest struct {
	QuestionID string `json:"question_id"`
	Value      *int   `json:"value"`
}

type SubmitTestAnswersResponse struct {
	ResultID             string                `json:"result_id"`
	TestID               string                `json:"test_id"`
	Revision             int                   `json:"revision"`
	CompletedAt          time.Time             `json:"completed_at"`
	BigFive              TestBigFive           `json:"big_five"`
	PersonalityType      model.PersonalityType `json:"personality_type"`
	AboutPersonalityType string                `json:"about_personality_type"`
}

type TestBigFive struct {
	Openness          *float64 `json:"openness"`
	Conscientiousness *float64 `json:"conscientiousness"`
	Extraversion      *float64 `json:"extraversion"`
	Agreeableness     *float64 `json:"agreeableness"`
	Neuroticism       *float64 `json:"neuroticism"`
}
