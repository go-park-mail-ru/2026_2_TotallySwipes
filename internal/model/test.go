package model

import "time"

type TestMethodology string

const (
	MethodologyTIPI     TestMethodology = "TIPI"
	MethodologyMiniIPIP TestMethodology = "MINI_IPIP"
)

type Test struct {
	Methodology   TestMethodology
	ID            int64
	Title         string
	Instructions  string
	AnswerOptions []AnswerOption
	Questions     []Question
}

type Question struct {
	ID          int64
	OperationID int64
	Body        string
}

type TestAnswers struct {
	TestID  int64
	Answers []Answer
}

type AnswerOption struct {
	Value int
	Label string
}

type Answer struct {
	QuestionID int64
	Value      int
}

type TestResult struct {
	ID         int64
	TestID     int64
	Revision   int
	CompletedA time.Time
}

func NewTIPITest(id int64) Test {
	return Test{
		ID:           id,
		Methodology:  MethodologyTIPI,
		Instructions: "Для каждого утверждения выберите один вариант ответа от 1 до 7.",
		AnswerOptions: []AnswerOption{
			{Value: 1, Label: "Совсем не согласен"},
			{Value: 2, Label: "Не согласен"},
			{Value: 3, Label: "Скорее не согласен"},
			{Value: 4, Label: "Ни согласен, ни не согласен"},
			{Value: 5, Label: "Скорее согласен"},
			{Value: 6, Label: "Согласен"},
			{Value: 7, Label: "Полностью согласен"},
		},
	}
}
