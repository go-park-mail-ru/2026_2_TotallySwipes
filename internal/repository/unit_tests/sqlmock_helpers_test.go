package repository

import (
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"github.com/DATA-DOG/go-sqlmock"
	"testing"
	"time"
)

// sqlmock's default converter rejects PostgreSQL array arguments. Encode only
// the arrays used here, preserving their contents and order for WithArgs checks.
// This is a test comparison format, not PostgreSQL wire encoding.
type arrayConverter struct{}

func (arrayConverter) ConvertValue(v any) (driver.Value, error) {
	switch v.(type) {
	case []int64, []string:
		b, err := json.Marshal(v)
		return string(b), err
	}
	return driver.DefaultParameterConverter.ConvertValue(v)
}

func newSQLMock(t *testing.T) (*sql.DB, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual), sqlmock.ValueConverterOption(arrayConverter{}))
	if err != nil {
		t.Fatalf("create sqlmock: %v", err)
	}
	t.Cleanup(func() {
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("SQL expectations: %v", err)
		}
		_ = db.Close()
	})
	return db, mock
}

var fixtureTime = time.Date(2026, 1, 2, 12, 0, 0, 0, time.UTC)
var fixtureBirth = time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)

var profileRowColumns = []string{"id", "user_id", "created_at", "updated_at", "has_filter", "city", "version_id", "version_recorded_at", "name", "birth_date", "sex", "dating_goal", "about_me", "education", "work", "smoking", "alcohol", "height", "psycho_id", "psycho_recorded_at", "openness", "conscientiousness", "extraversion", "agreeableness", "neuroticism"}

func profileValues(id, userID int64, filled bool) []driver.Value {
	values := []driver.Value{id, userID, fixtureTime, fixtureTime, filled, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil}
	if filled {
		copy(values[6:18], []driver.Value{int64(11), fixtureTime, "Anna", fixtureBirth, "female", "friendship", nil, "higher", "Engineer", "negative", nil, int64(180)})
	}
	return values
}

func profileRows(id, userID int64, filled bool) *sqlmock.Rows {
	return sqlmock.NewRows(profileRowColumns).AddRow(profileValues(id, userID, filled)...)
}

func expectPhotos(mock sqlmock.Sqlmock, profileID int64, count int) {
	rows := sqlmock.NewRows([]string{"id", "storage_key", "position"})
	for i := 1; i <= count; i++ {
		rows.AddRow(int64(i), photoKey(i), i)
	}
	mock.ExpectQuery(photosSQL).WithArgs(profileID).WillReturnRows(rows).RowsWillBeClosed()
}

func expectProfileLoad(mock sqlmock.Sqlmock, lock, filled bool, count int) {
	if lock {
		mock.ExpectQuery(lockProfileSQL).WithArgs(int64(42)).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(7)))
	}
	mock.ExpectQuery(profileSelectSQL + " WHERE p.user_id = $1").WithArgs(int64(42)).WillReturnRows(profileRows(7, 42, filled))
	if filled {
		mock.ExpectQuery(tagsSQL).WithArgs(int64(11)).WillReturnRows(sqlmock.NewRows([]string{"id", "name"}).AddRow(int64(1), "coffee").AddRow(int64(2), "music")).RowsWillBeClosed()
	}
	expectPhotos(mock, 7, count)
}

func expectTouch(mock sqlmock.Sqlmock) {
	mock.ExpectExec(touchSQL).WithArgs(int64(7)).WillReturnResult(sqlmock.NewResult(0, 1))
}
