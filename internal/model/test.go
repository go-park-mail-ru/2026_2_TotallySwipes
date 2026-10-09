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
	ID                   int64
	TestID               int64
	Revision             int
	CompletedA           time.Time
	BigFive              BigFive
	PersonalityType      PersonalityType
	AboutPersonalityType string
}
