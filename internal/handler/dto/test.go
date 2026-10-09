package dto

import (
	"dating-app/internal/model"
	"strconv"
)

type TestQuestion struct {
	ID   string `json:"id"`
	Body string `json:"body"`
}

type TestOption struct {
	Value int    `json:"value"`
	Label string `json:"label"`
}

type CurrentTestResponse struct {
	TestID        string         `json:"test_id"`
	Title         string         `json:"title"`
	Instructions  string         `json:"instructions"`
	AnswerOptions []TestOption   `json:"answer_options"`
	Questions     []TestQuestion `json:"questions"`
}

func NewCurrentTestResponse(t *model.Test) CurrentTestResponse {
	resp := CurrentTestResponse{
		TestID:        strconv.FormatInt(t.ID, 10),
		Title:         t.Title,
		Instructions:  t.Instructions,
		AnswerOptions: make([]TestOption, 0, len(t.AnswerOptions)),
		Questions:     make([]TestQuestion, 0, len(t.Questions)),
	}
	for _, o := range t.AnswerOptions {
		resp.AnswerOptions = append(resp.AnswerOptions, TestOption{Value: o.Value, Label: o.Label})
	}
	for _, q := range t.Questions {
		resp.Questions = append(resp.Questions, TestQuestion{ID: strconv.FormatInt(q.ID, 10), Body: q.Body})
	}
	return resp
}
