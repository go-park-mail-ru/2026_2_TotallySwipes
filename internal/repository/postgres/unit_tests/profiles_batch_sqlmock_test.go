package postgres_test

import (
	"context"
	"errors"
	"testing"

	"dating-app/internal/model"
	"dating-app/internal/repository/postgres"

	"github.com/DATA-DOG/go-sqlmock"
)

const profilesBatchSQL = profileSelectSQL + `
			WHERE p.user_id = ANY($1::bigint[])
			AND ($2::text IS NULL OR v.name ILIKE '%' || $2::text || '%' ESCAPE '\')
			AND ($3::int IS NULL OR date_part('year', age($6::date, v.birth_date)) >= $3::int)
			AND ($4::int IS NULL OR date_part('year', age($6::date, v.birth_date)) <= $4::int)
			AND ($5::text IS NULL OR v.sex = $5::text)`

func TestGetByUserIDsKeepsInputOrder(t *testing.T) {
	db, mock := newSQLMock(t)
	repo := postgres.NewProfileRepository(db)

	rows := sqlmock.NewRows(profileRowColumns).
		AddRow(profileValues(8, 102, true)...).
		AddRow(profileValues(7, 101, true)...)
	// Строки приходят не в порядке входного списка, ID 103 без анкеты.
	mock.ExpectBegin()
	mock.ExpectQuery(profilesBatchSQL).
		WithArgs([]int64{101, 102, 103}, nil, nil, nil, nil, sqlmock.AnyArg()).
		WillReturnRows(rows).RowsWillBeClosed()
	mock.ExpectQuery(feedTagsSQL).WithArgs(sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"profile_version_id", "id", "name"})).RowsWillBeClosed()
	mock.ExpectQuery(feedPhotosSQL).WithArgs(sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"profile_id", "id", "storage_key", "position"}).
			AddRow(int64(7), int64(1), "photos/a.jpg", 1).
			AddRow(int64(8), int64(2), "photos/b.jpg", 1)).RowsWillBeClosed()
	mock.ExpectCommit()

	got, err := repo.GetByUserIDs(context.Background(), []int64{101, 102, 103}, nil)
	if err != nil {
		t.Fatalf("GetByUserIDs: %v", err)
	}
	if len(got) != 2 || got[0].UserID != 101 || got[1].UserID != 102 {
		t.Fatalf("order = %+v", got)
	}
	if len(got[0].Photos) != 1 || got[0].Photos[0].StorageKey != "photos/a.jpg" || got[1].Photos[0].StorageKey != "photos/b.jpg" {
		t.Fatalf("photos = %+v / %+v", got[0].Photos, got[1].Photos)
	}
}

func TestGetByUserIDsPassesFilter(t *testing.T) {
	db, mock := newSQLMock(t)
	repo := postgres.NewProfileRepository(db)

	name, from, to, sex := "an_%", 20, 30, model.SexFemale
	mock.ExpectBegin()
	mock.ExpectQuery(profilesBatchSQL).
		WithArgs([]int64{101}, `an\_\%`, int64(from), int64(to), "female", sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows(profileRowColumns)).RowsWillBeClosed()
	mock.ExpectCommit()

	got, err := repo.GetByUserIDs(context.Background(), []int64{101},
		&model.ProfilesFilter{Name: &name, AgeFrom: &from, AgeTo: &to, Sex: &sex})
	if err != nil || len(got) != 0 {
		t.Fatalf("got=%v err=%v", got, err)
	}
}

func TestGetByUserIDsEmptyInputSkipsQuery(t *testing.T) {
	db, _ := newSQLMock(t)
	repo := postgres.NewProfileRepository(db)

	got, err := repo.GetByUserIDs(context.Background(), nil, nil)
	if err != nil || got == nil || len(got) != 0 {
		t.Fatalf("got=%v err=%v", got, err)
	}
}

func TestGetByUserIDsQueryError(t *testing.T) {
	db, mock := newSQLMock(t)
	repo := postgres.NewProfileRepository(db)
	failure := errors.New("db down")

	mock.ExpectBegin()
	mock.ExpectQuery(profilesBatchSQL).
		WithArgs([]int64{101}, nil, nil, nil, nil, sqlmock.AnyArg()).
		WillReturnError(failure)
	mock.ExpectRollback()

	if _, err := repo.GetByUserIDs(context.Background(), []int64{101}, nil); !errors.Is(err, failure) {
		t.Fatalf("err = %v, want %v", err, failure)
	}
}
