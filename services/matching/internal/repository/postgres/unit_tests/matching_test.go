package postgres_test

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"math"
	"testing"
	"time"

	"dating-app/services/matching/internal/model"
	"dating-app/services/matching/internal/repository/postgres"
	"github.com/DATA-DOG/go-sqlmock"
)

var failure = errors.New("database failure")

type arrayConverter struct{}

func (arrayConverter) ConvertValue(value any) (driver.Value, error) {

	if array, ok := value.([]int64); ok {
		data, err := json.Marshal(array)
		return string(data), err
	}

	return driver.DefaultParameterConverter.ConvertValue(value)
}

func newSQLMock(t *testing.T) (*sql.DB, sqlmock.Sqlmock) {

	t.Helper()
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual), sqlmock.ValueConverterOption(arrayConverter{}))

	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {

		if err := mock.ExpectationsWereMet(); err != nil {
			t.Error(err)
		}

		db.Close()
	})
	return db, mock
}

func TestMatchPagination(t *testing.T) {

	for _, name := range []string{"страница", "последняя страница", "ошибка count", "ошибка select", "ошибка commit"} {
		t.Run(name, func(t *testing.T) {

			db, mock := newSQLMock(t)
			mock.ExpectBegin()
			count := mock.ExpectQuery(countMatchesQuery).WithArgs(int64(1))
			wantErr := error(nil)

			if name == "ошибка count" {
				count.WillReturnError(failure)
				wantErr = failure
			} else {
				count.WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(3))
				query := mock.ExpectQuery(selectMatchesQuery).WithArgs(int64(1), int64(math.MaxInt64), 3)

				if name == "ошибка select" {
					query.WillReturnError(failure)
					wantErr = failure
				} else {
					rows := sqlmock.NewRows([]string{"id", "chat_id", "created_at", "user_id", "is_new"}).
						AddRow(30, 300, time.Now(), 2, true).
						AddRow(20, nil, time.Now(), 3, false)

					if name != "последняя страница" {
						rows.AddRow(10, 100, time.Now(), 4, false)
					}

					query.WillReturnRows(rows).RowsWillBeClosed()
				}
			}

			if wantErr != nil {
				mock.ExpectRollback()
			} else {
				commit := mock.ExpectCommit()

				if name == "ошибка commit" {
					commit.WillReturnError(failure)
					wantErr = failure
				}
			}

			matches, total, next, err := postgres.NewMatchRepository(db).GetMatches(context.Background(), 1, 2, nil)

			if !errors.Is(err, wantErr) {
				t.Fatalf("error = %v, want %v", err, wantErr)
			}

			if err == nil && (len(matches) != 2 || matches[0].ID != 30 || matches[0].UserID != 2 || !matches[0].IsNew || total != 3) {
				t.Fatalf("matches = %v, total = %d", matches, total)
			}

			if err == nil && (matches[0].ChatID == nil || *matches[0].ChatID != 300 || matches[1].ChatID != nil) {
				t.Fatalf("chat IDs = %v, %v; want 300, nil", matches[0].ChatID, matches[1].ChatID)
			}

			if name == "страница" && (next == nil || *next != 20) {
				t.Fatalf("next = %v", next)
			}

			if name == "последняя страница" && next != nil {
				t.Fatalf("next = %v", next)
			}
		})
	}
}

func TestMatchReads(t *testing.T) {

	db, mock := newSQLMock(t)
	repo := postgres.NewMatchRepository(db)
	mock.ExpectQuery(selectMatchSummaryQuery).WithArgs(int64(1)).WillReturnRows(sqlmock.NewRows([]string{"total", "new"}).AddRow(3, 2))
	summary, err := repo.GetSummary(context.Background(), 1)

	if err != nil || summary.Total != 3 || summary.New != 2 {
		t.Fatalf("summary = %v, error = %v", summary, err)
	}

	mock.ExpectQuery(getMatchIDQuery).WithArgs(int64(1), int64(2)).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(20))

	if id, err := repo.GetMatchID(context.Background(), 1, 2); err != nil || id != 20 {
		t.Fatalf("id = %d, error = %v", id, err)
	}

	mock.ExpectQuery(getMatchIDQuery).WithArgs(int64(1), int64(3)).WillReturnError(sql.ErrNoRows)

	if _, err := repo.GetMatchID(context.Background(), 1, 3); !errors.Is(err, model.ErrNotFound) {
		t.Fatal(err)
	}

	mock.ExpectQuery(getParticipantsQuery).WithArgs(int64(20), int64(1)).WillReturnRows(sqlmock.NewRows([]string{"first", "second"}).AddRow(1, 2))

	if got, err := repo.GetParticipants(context.Background(), 1, 20); err != nil || got != [2]int64{1, 2} {
		t.Fatalf("participants = %v, error = %v", got, err)
	}

	mock.ExpectExec(markMatchSeenQuery).WithArgs(int64(1), int64(20)).WillReturnResult(sqlmock.NewResult(0, 1))

	if err := repo.MarkSeen(context.Background(), 1, 20); err != nil {
		t.Fatal(err)
	}

	mock.ExpectExec(markMatchSeenQuery).WithArgs(int64(3), int64(20)).WillReturnResult(sqlmock.NewResult(0, 0))

	if err := repo.MarkSeen(context.Background(), 3, 20); !errors.Is(err, model.ErrNotFound) {
		t.Fatal(err)
	}
}

func TestFeedSwipeStates(t *testing.T) {

	db, mock := newSQLMock(t)
	repo := postgres.NewFeedRepository(db)

	if got, err := repo.GetSwipeStates(context.Background(), 1, 101, nil); err != nil || len(got) != 0 {
		t.Fatalf("states = %v, error = %v", got, err)
	}

	mock.ExpectQuery(selectSwipeStatesQuery).WithArgs(int64(1), "[2,3]", "[102,103]", int64(101)).WillReturnRows(sqlmock.NewRows([]string{"user_id", "swiped", "liked_me"}).AddRow(2, true, false).AddRow(3, false, true)).RowsWillBeClosed()
	got, err := repo.GetSwipeStates(context.Background(), 1, 101, []model.Profile{{ID: 102, UserID: 2}, {ID: 103, UserID: 3}})

	if err != nil || !got[2].Swiped || got[2].LikedMe || !got[3].LikedMe || got[3].Swiped {
		t.Fatalf("states = %v, error = %v", got, err)
	}
}
