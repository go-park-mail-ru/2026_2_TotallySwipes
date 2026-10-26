package repository

import (
	"context"
	"database/sql/driver"
	"dating-app/internal/model"
	"dating-app/internal/repository"
	"encoding/json"
	"errors"
	"github.com/DATA-DOG/go-sqlmock"
	"reflect"
	"slices"
	"testing"
	"time"
)

const feedTagsSQL = `SELECT pt.profile_version_id, t.id, t.name
FROM profile_tag AS pt
JOIN tag AS t ON t.id = pt.tag_id
WHERE pt.profile_version_id = ANY($1::bigint[])
ORDER BY pt.profile_version_id, t.id`

const feedPhotosSQL = `SELECT profile_id, id, storage_key, position
FROM photo
WHERE profile_id = ANY($1::bigint[])
ORDER BY profile_id, position`

// arrayConverter encodes array arguments as JSON; map iteration order is arbitrary.
type idSetArgument []int64

func (want idSetArgument) Match(value driver.Value) bool {
	encoded, ok := value.(string)
	if !ok {
		return false
	}
	var got []int64
	if err := json.Unmarshal([]byte(encoded), &got); err != nil {
		return false
	}
	expected := append([]int64(nil), want...)
	slices.Sort(got)
	slices.Sort(expected)
	return slices.Equal(got, expected)
}

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
		{name: "tags query error", count: 1, failAt: "tags", wantErr: failure},
		{name: "photos query error", count: 1, failAt: "photos", wantErr: failure},
		{name: "tags iteration error", count: 2, failAt: "tags_rows", wantErr: failure},
		{name: "photos iteration error", count: 2, failAt: "photos_rows", wantErr: failure},
		{name: "tags scan error", count: 1, failAt: "tags_scan"},
		{name: "photos scan error", count: 1, failAt: "photos_scan"},
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
						values[6] = int64(11 + i)
						if tc.failAt == "scan" {
							values[0] = "invalid ID"
						}
						rows.AddRow(append(values, i%2 == 0)...)
					}
					if tc.failAt == "rows" {
						rows.RowError(1, failure)
					}
					query.WillReturnRows(rows).RowsWillBeClosed()
					if tc.failAt != "rows" && tc.failAt != "scan" && tc.count > 0 {
						var versionIDs, profileIDs idSetArgument
						tagsRows := sqlmock.NewRows([]string{"profile_version_id", "id", "name"})
						photosRows := sqlmock.NewRows([]string{"profile_id", "id", "storage_key", "position"})
						for i := 0; i < min(tc.count, 2); i++ {
							versionIDs = append(versionIDs, int64(11+i))
							profileIDs = append(profileIDs, int64(7+i))
						}
						// Return profiles in reverse order to verify association by ID.
						for i := min(tc.count, 2) - 1; i >= 0; i-- {
							var tagID, photoID any = int64(1 + i), int64(21 + i)
							if tc.failAt == "tags_scan" {
								tagID = "invalid ID"
							}
							if tc.failAt == "photos_scan" {
								photoID = "invalid ID"
							}
							tagsRows.AddRow(int64(11+i), tagID, []string{"coffee", "music"}[i])
							photosRows.AddRow(int64(7+i), photoID, photoKey(i+1), 1)
						}
						tags := mock.ExpectQuery(feedTagsSQL).WithArgs(versionIDs)
						if tc.failAt == "tags" {
							tags.WillReturnError(failure)
						} else {
							if tc.failAt == "tags_rows" {
								tagsRows.RowError(1, failure)
							}
							tags.WillReturnRows(tagsRows).RowsWillBeClosed()
							if tc.failAt != "tags_rows" && tc.failAt != "tags_scan" {
								photos := mock.ExpectQuery(feedPhotosSQL).WithArgs(profileIDs)
								if tc.failAt == "photos" {
									photos.WillReturnError(failure)
								} else {
									if tc.failAt == "photos_rows" {
										photosRows.RowError(1, failure)
									}
									photos.WillReturnRows(photosRows).RowsWillBeClosed()
								}
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
			got, next, err := repo.GetCandidatesByCursorAndLimit(context.Background(), 42, 2, tc.cursor)
			// Assert
			if tc.failAt == "scan" || tc.failAt == "tags_scan" || tc.failAt == "photos_scan" {
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
				if p.ID != int64(7+i) || p.UserID != int64(101+i) || candidate.LikedMe != (i%2 == 0) || p.CurrentVersion.ID != int64(11+i) || *p.CurrentVersion.Name != "Anna" || !p.HasSearchFilter {
					t.Fatalf("candidate=%+v", candidate)
				}
				if !reflect.DeepEqual(p.Tags, []model.Tag{{ID: int64(1 + i), Name: []string{"coffee", "music"}[i]}}) || !reflect.DeepEqual(p.Photos, []model.Photo{{ID: int64(21 + i), StorageKey: photoKey(i + 1), Position: 1}}) {
					t.Fatalf("relations=%+v", p)
				}
			}
		})
	}
}
