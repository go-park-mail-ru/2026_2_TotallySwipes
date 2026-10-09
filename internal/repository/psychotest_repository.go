package repository

import (
	"context"
	"database/sql"
	"dating-app/internal/model"
	"errors"
	"fmt"
	"strings"
)

type PsychoTestRepo struct {
	db *sql.DB
}

func NewPsychoTestRepository(db *sql.DB) *PsychoTestRepo {
	return &PsychoTestRepo{db: db}
}

// SaveTestResult атомарно сохраняет новую ревизию психопрофиля и ответы на тест.
func (r *PsychoTestRepo) SaveTestResult(ctx context.Context, profileID int64, answers *model.TestAnswers, psycho *model.ProfilePsychoInput) (*model.TestResult, error) {

	if answers == nil || psycho == nil {
		return nil, fmt.Errorf("save test result: answers and psycho are required")
	}
	if len(answers.Answers) == 0 {
		return nil, fmt.Errorf("save test result: answers are empty")
	}

	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return nil, fmt.Errorf("save test result: begin transaction: %w", err)
	}
	defer tx.Rollback()

	var lockedID int64
	err = tx.QueryRowContext(ctx, `SELECT id FROM profile WHERE id = $1 FOR UPDATE`, profileID).Scan(&lockedID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, model.ErrNotFound
	}

	if err != nil {
		return nil, fmt.Errorf("save test result: lock profile: %w", err)
	}

	var result model.TestResult
	err = tx.QueryRowContext(ctx, `
		INSERT INTO profile_psycho (
			profile_id, revision, openness, conscientiousness,
			extraversion, agreeableness, neuroticism, personality_type
		)
		SELECT $1, COALESCE(MAX(revision), 0) + 1, $2, $3, $4, $5, $6, $7
		FROM profile_psycho WHERE profile_id = $1
		RETURNING id, revision, recorded_at`,
		profileID, psycho.Openness, psycho.Conscientiousness,
		psycho.Extraversion, psycho.Agreeableness, psycho.Neuroticism,
		nullIfEmpty(string(psycho.PersonalityType)),
	).Scan(&result.ID, &result.Revision, &result.CompletedA)

	if err != nil {
		return nil, fmt.Errorf("save test result: insert psycho: %w", err)
	}

	args := make([]any, 1, 1+2*len(answers.Answers))
	args[0] = result.ID
	placeholders := make([]string, len(answers.Answers))
	for i, answer := range answers.Answers {
		n := 2 + i*2
		placeholders[i] = fmt.Sprintf("($1, $%d, $%d)", n, n+1)
		args = append(args, answer.QuestionID, answer.Value)
	}

	_, err = tx.ExecContext(ctx,
		`
		INSERT INTO user_answer (profile_psycho_id, question_no, answer_value)
		VALUES `+strings.Join(placeholders, ", "), args...)

	if err != nil {
		return nil, fmt.Errorf("save test result: insert answers: %w", err)
	}

	_, err = tx.ExecContext(ctx, `UPDATE profile SET updated_at = CURRENT_TIMESTAMP WHERE id = $1`, profileID)
	if err != nil {
		return nil, fmt.Errorf("save test result: update profile timestamp: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("save test result: commit: %w", err)
	}

	return &result, nil
}

// GetTestResult читает последнюю ревизию результата; нет результата - model.ErrNotFound
func (r *PsychoTestRepo) GetTestResult(ctx context.Context, profileID int64) (*model.TestResult, error) {
	var result model.TestResult
	var personalityType sql.NullString
	err := r.db.QueryRowContext(ctx, `
		SELECT id, revision, recorded_at, personality_type,
		       openness, conscientiousness, extraversion, agreeableness, neuroticism
		FROM profile_psycho
		WHERE profile_id = $1 AND openness IS NOT NULL
		ORDER BY revision DESC
		LIMIT 1`,
		profileID,
	).Scan(&result.ID, &result.Revision, &result.CompletedA, &personalityType,
		&result.BigFive.Openness, &result.BigFive.Conscientiousness, &result.BigFive.Extraversion,
		&result.BigFive.Agreeableness, &result.BigFive.Neuroticism)

	if errors.Is(err, sql.ErrNoRows) {
		return nil, model.ErrNotFound
	}

	if err != nil {
		return nil, fmt.Errorf("get test result: %w", err)
	}

	result.PersonalityType = model.PersonalityType(personalityType.String)
	return &result, nil
}
