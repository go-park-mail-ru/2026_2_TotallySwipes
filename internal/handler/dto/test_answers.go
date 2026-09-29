package dto

import "time"

type SubmitTestAnswersRequest struct {
	Answers []TestAnswerRequest `json:"answers"`
}

type TestAnswerRequest struct {
	QuestionID string `json:"question_id"`
	Value      *int   `json:"value"`
}

type SubmitTestAnswersResponse struct {
	ResultID    string    `json:"result_id"`
	TestID      string    `json:"test_id"`
	Revision    int       `json:"revision"`
	CompletedAt time.Time `json:"completed_at"`
}
