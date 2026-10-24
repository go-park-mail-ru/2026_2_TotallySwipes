package repository

import (
	"context"
	"database/sql/driver"
	"dating-app/internal/model"
	"dating-app/internal/repository"
	"errors"
	"github.com/DATA-DOG/go-sqlmock"
	"reflect"
	"testing"
	"time"
)

type dateArgument struct{ first, last string }

func (d dateArgument) Match(value driver.Value) bool {
	s, ok := value.(string)
	return ok && (s == d.first || s == d.last)
}

func TestProfileRepo_Feed(t *testing.T) {
	failure := errors.New("database unavailable")
	cursor := int64(100)
	for _, tc := range []struct {
		name, failAt string
		cursor       *int64
		count        int
		wantErr      error
	}{
		{name: "empty"}, {name: "single result", count: 1}, {name: "full final page", count: 2},
		{name: "next page", cursor: &cursor, count: 3},
		{name: "begin error", failAt: "begin", wantErr: failure},
		{name: "query error", failAt: "query", wantErr: failure},
		{name: "iteration error", count: 2, failAt: "rows", wantErr: failure},
		{name: "scan error", count: 1, failAt: "scan"},
		{name: "relations error", count: 1, failAt: "relations", wantErr: failure},
		{name: "commit error", count: 1, failAt: "commit", wantErr: failure},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Arrange
			db, mock := newSQLMock(t)
			repo := repository.NewProfileRepository(db)
			before := time.Now()
			date := dateArgument{before.Format(time.DateOnly), before.Add(time.Second).Format(time.DateOnly)}
			after := int64(0)
			if tc.cursor != nil {
				after = *tc.cursor
			}
			begin := mock.ExpectBegin()
			if tc.failAt == "begin" {
				begin.WillReturnError(failure)
			} else {
				query := mock.ExpectQuery(feedSQL).WithArgs(after, int64(42), 3, date)
				if tc.failAt == "query" {
					query.WillReturnError(failure)
				} else {
					rows := sqlmock.NewRows(append(append([]string{}, profileRowColumns...), "liked_me"))
					for i := 0; i < tc.count; i++ {
						values := profileValues(int64(7+i), int64(101+i), true)
						if tc.failAt == "scan" {
							values[0] = "invalid ID"
						}
						rows.AddRow(append(values, i%2 == 0)...)
					}
					if tc.failAt == "rows" {
						rows.RowError(1, failure)
					}
					query.WillReturnRows(rows).RowsWillBeClosed()
					if tc.failAt != "rows" && tc.failAt != "scan" {
						for i := 0; i < tc.count && i < 2; i++ {
							tags := mock.ExpectQuery(tagsSQL).WithArgs(int64(11))
							if tc.failAt == "relations" {
								tags.WillReturnError(failure)
								break
							}
							tags.WillReturnRows(sqlmock.NewRows([]string{"id", "name"}).AddRow(int64(1), "coffee")).RowsWillBeClosed()
							expectPhotos(mock, int64(7+i), 1)
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
			got, next, err := repo.GetCandidatesByCursorAndLimit(context.Background(), 42, 2, tc.cursor)
			// Assert
			if tc.failAt == "scan" {
				if err == nil || got != nil || next != nil {
					t.Fatalf("got=%v next=%v err=%v", got, next, err)
				}
				return
			}
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("err=%v want=%v", err, tc.wantErr)
			}
			if tc.wantErr != nil {
				if got != nil || next != nil {
					t.Fatal("expected nil result")
				}
				return
			}
			size := min(tc.count, 2)
			if got == nil || len(got) != size {
				t.Fatalf("page=%+v", got)
			}
			if tc.count > 2 {
				if next == nil || *next != 102 {
					t.Fatalf("next=%v", next)
				}
			} else if next != nil {
				t.Fatalf("unexpected next=%d", *next)
			}
			for i, candidate := range got {
				p := candidate.Profile
				if p.ID != int64(7+i) || p.UserID != int64(101+i) || candidate.LikedMe != (i%2 == 0) || p.CurrentVersion.ID != 11 || *p.CurrentVersion.Name != "Anna" || !p.HasSearchFilter {
					t.Fatalf("candidate=%+v", candidate)
				}
				if !reflect.DeepEqual(p.Tags, []model.Tag{{ID: 1, Name: "coffee"}}) || !reflect.DeepEqual(p.Photos, []model.Photo{{ID: 1, StorageKey: photoKey(1), Position: 1}}) {
					t.Fatalf("relations=%+v", p)
				}
			}
		})
	}
}
