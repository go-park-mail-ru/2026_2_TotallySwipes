package dto

import (
	"dating-app/internal/model"
	"errors"
	"strconv"
	"time"
)

var (
	ErrAnswersRequired = errors.New("поле answers обязательно и не может быть null")
	ErrAnswerInvalid   = errors.New("некорректный ID вопроса или отсутствует ответ")
)

type SubmitTestAnswersRequest struct {
	Answers []TestAnswerRequest `json:"answers"`
}

type TestAnswerRequest struct {
	QuestionID string `json:"question_id"`
	Value      *int   `json:"value"`
}

// ToModel проверяет форму ответов; полноту и значения проверяет сервис
func (r SubmitTestAnswersRequest) ToModel(testID int64) (*model.TestAnswers, error) {
	if r.Answers == nil {
		return nil, ErrAnswersRequired
	}
	answers := &model.TestAnswers{TestID: testID, Answers: make([]model.Answer, 0, len(r.Answers))}
	for _, a := range r.Answers {
		questionID, err := strconv.ParseInt(a.QuestionID, 10, 64)
		if err != nil || questionID <= 0 || a.Value == nil {
			return nil, ErrAnswerInvalid
		}
		answers.Answers = append(answers.Answers, model.Answer{QuestionID: questionID, Value: *a.Value})
	}
	return answers, nil
}

type TestResultResponse struct {
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

func NewTestResultResponse(r *model.TestResult) TestResultResponse {
	return TestResultResponse{
		ResultID:             strconv.FormatInt(r.ID, 10),
		TestID:               strconv.FormatInt(r.TestID, 10),
		Revision:             r.Revision,
		CompletedAt:          r.CompletedA,
		PersonalityType:      r.PersonalityType,
		AboutPersonalityType: r.AboutPersonalityType,
		BigFive: TestBigFive{
			Openness:          r.BigFive.Openness,
			Conscientiousness: r.BigFive.Conscientiousness,
			Extraversion:      r.BigFive.Extraversion,
			Agreeableness:     r.BigFive.Agreeableness,
			Neuroticism:       r.BigFive.Neuroticism,
		},
	}
}
