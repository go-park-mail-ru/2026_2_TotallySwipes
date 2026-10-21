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

func TestFilterRepo_Get(t *testing.T) {
	failure := errors.New("database unavailable")
	saved := model.SearchFilter{Sex: model.SearchSexFemale, AgeFrom: 20, AgeTo: 30}
	for _, tc := range []struct {
		name           string
		empty, badRow  bool
		dbErr, wantErr error
	}{
		{name: "found"}, {name: "not found", empty: true, wantErr: model.ErrNotFound}, {name: "database error", dbErr: failure, wantErr: failure}, {name: "scan error", badRow: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Arrange
			db, mock := newSQLMock(t)
			repo := repository.NewFilterRepository(db)
			rows := sqlmock.NewRows([]string{"sex", "age_from", "age_to"})
			if tc.badRow {
				rows.AddRow("female", "not an integer", 30)
			} else if !tc.empty {
				rows.AddRow(string(saved.Sex), saved.AgeFrom, saved.AgeTo)
			}
			query := mock.ExpectQuery(`SELECT sex, age_from, age_to FROM search_filter WHERE user_id = $1`).WithArgs(int64(42))
			if tc.dbErr != nil {
				query.WillReturnError(tc.dbErr)
			} else {
				query.WillReturnRows(rows)
			}
			// Act
			got, err := repo.Get(context.Background(), 42)
			// Assert
			if tc.badRow {
				if err == nil || got != nil {
					t.Fatalf("got=%v err=%v", got, err)
				}
				return
			}
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("err=%v want=%v", err, tc.wantErr)
			}
			if tc.wantErr != nil {
				if got != nil {
					t.Fatal("expected nil filter")
				}
				return
			}
			if !reflect.DeepEqual(got, &saved) {
				t.Fatalf("got=%+v want=%+v", got, saved)
			}
		})
	}
}

func TestFilterRepo_Upsert(t *testing.T) {
	for _, wantErr := range []error{nil, errors.New("write failed")} {
		t.Run(errorName(wantErr), func(t *testing.T) {
			// Arrange
			db, mock := newSQLMock(t)
			repo := repository.NewFilterRepository(db)
			filter := model.SearchFilter{Sex: model.SearchSexFemale, AgeFrom: 20, AgeTo: 30}
			exec := mock.ExpectExec(`INSERT INTO search_filter (user_id, sex, age_from, age_to) VALUES ($1, $2, $3, $4)
   ON CONFLICT (user_id) DO UPDATE
   SET sex = EXCLUDED.sex, age_from = EXCLUDED.age_from, age_to = EXCLUDED.age_to,
   updated_at = CURRENT_TIMESTAMP`).WithArgs(int64(42), string(filter.Sex), filter.AgeFrom, filter.AgeTo)
			if wantErr != nil {
				exec.WillReturnError(wantErr)
			} else {
				exec.WillReturnResult(sqlmock.NewResult(0, 1))
			}
			// Act
			err := repo.Upsert(context.Background(), 42, &filter)
			// Assert
			if !errors.Is(err, wantErr) {
				t.Fatalf("err=%v want=%v", err, wantErr)
			}
		})
	}
}
func errorName(err error) string {
	if err == nil {
		return "success"
	}
	return err.Error()
}
