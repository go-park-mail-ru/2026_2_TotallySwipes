package repository

import (
	"context"
	"dating-app/internal/model"
	"dating-app/internal/repository"
	"errors"
	"github.com/DATA-DOG/go-sqlmock"
	"reflect"
	"testing"
)

func TestPsychoTestRepo_SaveTestResult(t *testing.T) {
	failure := errors.New("database unavailable")
	for _, tc := range []struct {
		name, failAt string
		wantErr      error
	}{
		{name: "success"}, {name: "begin error", failAt: "begin", wantErr: failure},
		{name: "missing profile", failAt: "missing", wantErr: model.ErrNotFound},
		{name: "lock error", failAt: "lock", wantErr: failure},
		{name: "psycho insert error", failAt: "psycho", wantErr: failure},
		{name: "answers error", failAt: "answers", wantErr: failure},
		{name: "touch error", failAt: "touch", wantErr: failure},
		{name: "commit error", failAt: "commit", wantErr: failure},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Arrange
			db, mock := newSQLMock(t)
			repo := repository.NewPsychoTestRepository(db)
			values := [5]float64{.1, .2, .3, .4, .5}
			psycho := &model.ProfilePsychoInput{Openness: &values[0], Conscientiousness: &values[1], Extraversion: &values[2], Agreeableness: &values[3], Neuroticism: &values[4], PersonalityType: model.PersonalityKeeper}
			answers := &model.TestAnswers{TestID: 1, Answers: []model.Answer{{QuestionID: 1, Value: 4}, {QuestionID: 2, Value: 6}}}
			begin := mock.ExpectBegin()
			if tc.failAt == "begin" {
				begin.WillReturnError(failure)
			} else {
				lock := mock.ExpectQuery(`SELECT id FROM profile WHERE id = $1 FOR UPDATE`).WithArgs(int64(7))
				switch tc.failAt {
				case "missing":
					lock.WillReturnRows(sqlmock.NewRows([]string{"id"}))
				case "lock":
					lock.WillReturnError(failure)
				default:
					lock.WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(7)))
					insert := mock.ExpectQuery(insertPsychoSQL).WithArgs(int64(7), .1, .2, .3, .4, .5, string(model.PersonalityKeeper))
					if tc.failAt == "psycho" {
						insert.WillReturnError(failure)
					} else {
						insert.WillReturnRows(sqlmock.NewRows([]string{"id", "revision", "recorded_at"}).AddRow(int64(9), 2, fixtureTime))
						save := mock.ExpectExec(`INSERT INTO user_answer (profile_psycho_id, question_no, answer_value) VALUES ($1, $2, $3), ($1, $4, $5)`).WithArgs(int64(9), int64(1), 4, int64(2), 6)
						if tc.failAt == "answers" {
							save.WillReturnError(failure)
						} else {
							save.WillReturnResult(sqlmock.NewResult(0, 2))
							if tc.failAt == "touch" {
								mock.ExpectExec(touchSQL).WithArgs(int64(7)).WillReturnError(failure)
							} else {
								expectTouch(mock)
							}
						}
					}
				}
				switch tc.failAt {
				case "":
					mock.ExpectCommit()
				case "commit":
					mock.ExpectCommit().WillReturnError(failure)
				default:
					mock.ExpectRollback()
				}
			}
			// Act
			got, err := repo.SaveTestResult(context.Background(), 7, answers, psycho)
			// Assert
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("err=%v want=%v", err, tc.wantErr)
			}
			if tc.wantErr != nil {
				if got != nil {
					t.Fatal("expected nil result")
				}
				return
			}
			want := &model.TestResult{ID: 9, Revision: 2, CompletedA: fixtureTime}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("got=%+v want=%+v", got, want)
			}
		})
	}
}

func TestPsychoTestRepo_SaveInvalidInput(t *testing.T) {
	for _, tc := range []struct {
		name    string
		answers *model.TestAnswers
		psycho  *model.ProfilePsychoInput
	}{
		{name: "nil answers", psycho: &model.ProfilePsychoInput{}},
		{name: "nil psycho", answers: &model.TestAnswers{Answers: []model.Answer{{QuestionID: 1, Value: 4}}}},
		{name: "empty answers", answers: &model.TestAnswers{}, psycho: &model.ProfilePsychoInput{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Arrange
			db, _ := newSQLMock(t)
			repo := repository.NewPsychoTestRepository(db)
			// Act
			got, err := repo.SaveTestResult(context.Background(), 7, tc.answers, tc.psycho)
			// Assert
			if err == nil || got != nil {
				t.Fatalf("got=%v err=%v", got, err)
			}
		})
	}
}

func TestPsychoTestRepo_GetTestResult(t *testing.T) {
	failure := errors.New("query failed")
	for _, tc := range []struct {
		name            string
		empty, nullType bool
		dbErr, wantErr  error
	}{
		{name: "stored result"}, {name: "null personality", nullType: true},
		{name: "not found", empty: true, wantErr: model.ErrNotFound},
		{name: "query error", dbErr: failure, wantErr: failure},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Arrange
			db, mock := newSQLMock(t)
			repo := repository.NewPsychoTestRepository(db)
			rows := sqlmock.NewRows([]string{"id", "revision", "recorded_at", "personality_type", "openness", "conscientiousness", "extraversion", "agreeableness", "neuroticism"})
			var personality any = string(model.PersonalityKeeper)
			if tc.nullType {
				personality = nil
			}
			if !tc.empty {
				rows.AddRow(int64(9), 3, fixtureTime, personality, .1, .2, .3, .4, .5)
			}
			query := mock.ExpectQuery(getResultSQL).WithArgs(int64(7))
			if tc.dbErr != nil {
				query.WillReturnError(tc.dbErr)
			} else {
				query.WillReturnRows(rows)
			}
			// Act
			got, err := repo.GetTestResult(context.Background(), 7)
			// Assert
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("err=%v want=%v", err, tc.wantErr)
			}
			if tc.wantErr != nil {
				if got != nil {
					t.Fatal("expected nil result")
				}
				return
			}
			if got.ID != 9 || got.Revision != 3 || !got.CompletedA.Equal(fixtureTime) {
				t.Fatalf("result=%+v", got)
			}
			if tc.nullType {
				if got.PersonalityType != "" {
					t.Fatal("expected empty personality")
				}
			} else if got.PersonalityType != model.PersonalityKeeper {
				t.Fatal("wrong personality")
			}
			for i, value := range []*float64{got.BigFive.Openness, got.BigFive.Conscientiousness, got.BigFive.Extraversion, got.BigFive.Agreeableness, got.BigFive.Neuroticism} {
				if value == nil || *value != []float64{.1, .2, .3, .4, .5}[i] {
					t.Fatalf("wrong BigFive field %d: %v", i, value)
				}
			}
		})
	}
}
