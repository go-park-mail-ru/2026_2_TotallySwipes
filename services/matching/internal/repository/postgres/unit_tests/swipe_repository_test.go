package postgres_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"dating-app/services/matching/internal/model"
	"dating-app/services/matching/internal/repository/postgres"
	"github.com/DATA-DOG/go-sqlmock"
)

func TestSwipeTransaction(t *testing.T) {

	tests := []struct {
		name         string
		action       model.SwipeAction
		insertedRows int64
		matchID      int64
		failAt       string
	}{
		{name: "новый мэтч", action: model.SwipeLike, insertedRows: 1, matchID: 20},
		{name: "существующий мэтч", action: model.SwipeLike, matchID: 20},
		{name: "суперлайк с взаимностью", action: model.SwipeSuperLike, insertedRows: 1, matchID: 20},
		{name: "нет встречного лайка", action: model.SwipeLike},
		{name: "суперлайк без взаимности", action: model.SwipeSuperLike},
		{name: "дизлайк", action: model.SwipeDislike},
		{name: "ошибка begin", failAt: "begin"},
		{name: "ошибка lock", failAt: "lock"},
		{name: "ошибка записи свайпа", failAt: "swipe"},
		{name: "ошибка создания мэтча", failAt: "match"},
		{name: "ошибка чтения мэтча", failAt: "select"},
		{name: "ошибка scan", failAt: "scan"},
		{name: "ошибка commit", matchID: 20, failAt: "commit"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			db, mock := newSQLMock(t)
			action := tt.action

			if action == "" {
				action = model.SwipeSuperLike
			}

			// Порядок ожиданий проверяет чтение ID до единственного Commit.
			prepare := func() {

				begin := mock.ExpectBegin()

				if tt.failAt == "begin" {
					begin.WillReturnError(failure)
					return
				}

				lock := mock.ExpectExec(lockSwipeQuery).WithArgs(int64(1), int64(2))

				if tt.failAt == "lock" {
					lock.WillReturnError(failure)
					mock.ExpectRollback()
					return
				}

				lock.WillReturnResult(sqlmock.NewResult(0, 1))
				swipe := mock.ExpectExec(upsertSwipeQuery).WithArgs(int64(102), int64(1), string(action))

				if tt.failAt == "swipe" {
					swipe.WillReturnError(failure)
					mock.ExpectRollback()
					return
				}

				swipe.WillReturnResult(sqlmock.NewResult(0, 1))
				match := mock.ExpectExec(insertMutualMatchQuery).WithArgs(int64(1), int64(2), string(action), int64(101))

				if tt.failAt == "match" {
					match.WillReturnError(failure)
					mock.ExpectRollback()
					return
				}

				match.WillReturnResult(sqlmock.NewResult(0, tt.insertedRows))
				query := mock.ExpectQuery(selectMatchIDQuery).WithArgs(int64(1), int64(2), string(action))

				if tt.failAt == "select" {
					query.WillReturnError(failure)
					mock.ExpectRollback()
					return
				}

				rows := sqlmock.NewRows([]string{"id"})

				if tt.failAt == "scan" {
					query.WillReturnRows(rows.AddRow("invalid ID")).RowsWillBeClosed()
					mock.ExpectRollback()
					return
				}

				if tt.matchID != 0 {
					rows.AddRow(tt.matchID)
				}

				query.WillReturnRows(rows).RowsWillBeClosed()
				commit := mock.ExpectCommit()

				if tt.failAt == "commit" {
					commit.WillReturnError(failure)
				}
			}

			prepare()

			matchID, err := postgres.NewSwipeRepository(db).SaveSwipeAndMatch(
				context.Background(),
				&model.Swipe{AuthorID: 1, TargetID: 2, Action: action},
				101,
				102,
			)

			if tt.failAt != "" {

				if matchID != 0 || err == nil {
					t.Fatalf("match ID = %d, error = %v; want zero ID and error", matchID, err)
				}

				if tt.failAt == "scan" {

					if !strings.Contains(err.Error(), "select match") {
						t.Fatalf("error = %v; want match lookup error", err)
					}

				} else if !errors.Is(err, failure) {
					t.Fatalf("error = %v, want %v", err, failure)
				}

				return
			}

			if err != nil || matchID != tt.matchID {
				t.Fatalf("match ID = %d, error = %v; want %d, nil", matchID, err, tt.matchID)
			}
		})
	}
}
