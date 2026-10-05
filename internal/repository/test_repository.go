package repository

import (
	"context"
	"database/sql"
	"dating-app/internal/model"
	"errors"
	"fmt"
	"strings"
)

type TestRepository interface {
	// GetCurrentTest получает текущий тест с вопросами и вариантами ответов.
	// Принимает: контекст ctx.
	// Возвращает: тест или ошибку, если получить его не удалось.
	GetCurrentTest(ctx context.Context) (*model.Test, error)
	// SaveTestResult атомарно сохраняет новую ревизию психопрофиля и ответы на тест.
	// Принимает: контекст ctx, ID профиля profileID, ответы answers и рассчитанный психопрофиль psycho.
	// Возвращает: метаданные результата (ID, ID теста, ревизию и время) или ошибку; Big Five в возвращаемом объекте не заполняет.
	SaveTestResult(ctx context.Context, profileID int64, answers *model.TestAnswers, psycho *model.ProfilePsychoInput) (*model.TestResult, error)
	// GetTestResult читает последнюю завершённую ревизию результата тестирования.
	// Принимает: контекст ctx и ID профиля profileID.
	// Возвращает: сохранённый вектор, тип личности и метаданные результата либо ошибку; при отсутствии — model.ErrNotFound.
	GetTestResult(ctx context.Context, profileID int64) (*model.TestResult, error)
}

type TestRepo struct {
	db          *sql.DB
	currentTest model.Test
}

// NewTestRepository создаёт репозиторий тестов и их результатов.
// Принимает: подключение db и конфигурацию текущего теста currentTest.
// Возвращает: экземпляр TestRepo.
func NewTestRepository(db *sql.DB, currentTest model.Test) *TestRepo {
	return &TestRepo{db: db, currentTest: currentTest}
}

// GetCurrentTest получает текущий тест с вопросами и вариантами ответов.
// Принимает: контекст ctx.
// Возвращает: тест или ошибку, если получить его не удалось.
func (r *TestRepo) GetCurrentTest(ctx context.Context) (*model.Test, error) {
	if r.currentTest.ID <= 0 {
		return nil, model.ErrNotFound
	}

	if r.currentTest.Methodology == "" {
		return nil, fmt.Errorf("get current test: test configuration is missing")
	}

	result := r.currentTest
	result.AnswerOptions = append([]model.AnswerOption(nil), result.AnswerOptions...)
	result.Questions = make([]model.Question, 0)
	err := r.db.QueryRowContext(ctx, `SELECT name FROM test WHERE id = $1`, result.ID).Scan(&result.Title)

	if err == sql.ErrNoRows {
		return nil, model.ErrNotFound
	}

	if err != nil {
		return nil, fmt.Errorf("get current test: %w", err)
	}

	rows, err := r.db.QueryContext(ctx, `SELECT id, operation_id, body FROM question WHERE test_id = $1 ORDER BY id`, result.ID)

	if err != nil {
		return nil, fmt.Errorf("get test questions: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var question model.Question
		if err := rows.Scan(&question.ID, &question.OperationID, &question.Body); err != nil {
			return nil, fmt.Errorf("scan test question: %w", err)
		}
		result.Questions = append(result.Questions, question)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read test questions: %w", err)
	}

	return &result, nil
}

// SaveTestResult атомарно сохраняет новую ревизию психопрофиля и ответы на тест.
// Принимает: контекст ctx, ID профиля profileID, ответы answers и рассчитанный психопрофиль psycho.
// Возвращает: метаданные результата (ID, ID теста, ревизию и время) или ошибку; Big Five в возвращаемом объекте не заполняет.
func (r *TestRepo) SaveTestResult(ctx context.Context, profileID int64, answers *model.TestAnswers, psycho *model.ProfilePsychoInput) (*model.TestResult, error) {

	if answers == nil || psycho == nil {
		return nil, fmt.Errorf("save test result: answers and psycho are required")
	}
	if answers.TestID != psycho.TestID {
		return nil, fmt.Errorf("save test result: test IDs differ")
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
			profile_id, test_id, revision, openness, conscientiousness,
			extraversion, agreeableness, neuroticism, personality_type
		)
		SELECT $1, $2, COALESCE(MAX(revision), 0) + 1, $3, $4, $5, $6, $7, $8
		FROM profile_psycho WHERE profile_id = $1
		RETURNING id, test_id, revision, recorded_at`,
		profileID, answers.TestID, psycho.Openness, psycho.Conscientiousness,
		psycho.Extraversion, psycho.Agreeableness, psycho.Neuroticism,
		nullIfEmpty(string(psycho.PersonalityType)),
	).Scan(&result.ID, &result.TestID, &result.Revision, &result.CompletedA)

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
		INSERT INTO user_answer (profile_psycho_id, question_id, answer_value)
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

// GetTestResult читает последнюю завершённую ревизию результата тестирования.
// Принимает: контекст ctx и ID профиля profileID.
// Возвращает: сохранённый вектор, тип личности и метаданные результата либо ошибку; при отсутствии — model.ErrNotFound.
func (r *TestRepo) GetTestResult(ctx context.Context, profileID int64) (*model.TestResult, error) {
	var result model.TestResult
	var personalityType sql.NullString
	err := r.db.QueryRowContext(ctx, `
		SELECT id, test_id, revision, recorded_at, personality_type,
		       openness, conscientiousness, extraversion, agreeableness, neuroticism
		FROM profile_psycho
		WHERE profile_id = $1 AND openness IS NOT NULL
		ORDER BY revision DESC
		LIMIT 1`,
		profileID,
	).Scan(&result.ID, &result.TestID, &result.Revision, &result.CompletedA, &personalityType,
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
