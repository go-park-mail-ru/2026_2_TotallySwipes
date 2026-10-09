// Package psychotest хранит единственный психологический тест приложения.
// Тест лежит в tipi.json, встраивается в бинарник и не хранится в БД.
package psychotest

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"strings"

	"dating-app/internal/model"
)

//go:embed tipi.json
var tipiJSON []byte

const (
	tipiQuestionsCount = 10
	tipiOptionsCount   = 7
)

type definition struct {
	ID            int64                 `json:"id"`
	Methodology   model.TestMethodology `json:"methodology"`
	Title         string                `json:"title"`
	Instructions  string                `json:"instructions"`
	AnswerOptions []struct {
		Value int    `json:"value"`
		Label string `json:"label"`
	} `json:"answer_options"`
	Questions []struct {
		ID   int64  `json:"id"`
		Body string `json:"body"`
	} `json:"questions"`
}

// Load разбирает встроенный тест и проверяет его структуру.
// Возвращает: тест, у которого номер вопроса совпадает с номером пункта TIPI, либо ошибку описания.
func Load() (model.Test, error) {
	var def definition
	if err := json.Unmarshal(tipiJSON, &def); err != nil {
		return model.Test{}, fmt.Errorf("psychotest: parse tipi.json: %w", err)
	}

	if def.ID <= 0 || def.Methodology != model.MethodologyTIPI ||
		strings.TrimSpace(def.Title) == "" || strings.TrimSpace(def.Instructions) == "" {
		return model.Test{}, fmt.Errorf("psychotest: invalid test header: %w", model.ErrInvalidTestDefinition)
	}

	test := model.Test{
		ID:            def.ID,
		Methodology:   def.Methodology,
		Title:         def.Title,
		Instructions:  def.Instructions,
		AnswerOptions: make([]model.AnswerOption, 0, len(def.AnswerOptions)),
		Questions:     make([]model.Question, 0, len(def.Questions)),
	}

	if len(def.AnswerOptions) != tipiOptionsCount {
		return model.Test{}, fmt.Errorf("psychotest: want %d answer options: %w", tipiOptionsCount, model.ErrInvalidTestDefinition)
	}
	for i, option := range def.AnswerOptions {
		if option.Value != i+1 || strings.TrimSpace(option.Label) == "" {
			return model.Test{}, fmt.Errorf("psychotest: invalid answer option %d: %w", i+1, model.ErrInvalidTestDefinition)
		}
		test.AnswerOptions = append(test.AnswerOptions, model.AnswerOption{Value: option.Value, Label: option.Label})
	}

	if len(def.Questions) != tipiQuestionsCount {
		return model.Test{}, fmt.Errorf("psychotest: want %d questions: %w", tipiQuestionsCount, model.ErrInvalidTestDefinition)
	}
	for i, question := range def.Questions {
		if question.ID != int64(i+1) || strings.TrimSpace(question.Body) == "" {
			return model.Test{}, fmt.Errorf("psychotest: invalid question %d: %w", i+1, model.ErrInvalidTestDefinition)
		}
		test.Questions = append(test.Questions, model.Question{ID: question.ID, OperationID: question.ID, Body: question.Body})
	}

	return test, nil
}
