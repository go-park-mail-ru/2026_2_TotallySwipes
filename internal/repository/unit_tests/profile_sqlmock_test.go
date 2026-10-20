package repository

import (
	"context"
	"dating-app/internal/model"
	"dating-app/internal/repository"
	"errors"
	"fmt"
	"github.com/DATA-DOG/go-sqlmock"
	"reflect"
	"testing"
)

func photoKey(i int) string { return fmt.Sprintf("history/%d.jpg", i) }

func TestProfileRepo_GetCurrent(t *testing.T) {
	failure := errors.New("database unavailable")
	for _, tc := range []struct {
		name, failAt string
		filled       bool
		wantErr      error
	}{
		{name: "empty profile"}, {name: "filled profile", filled: true},
		{name: "begin error", failAt: "begin", wantErr: failure},
		{name: "not found", failAt: "missing", wantErr: model.ErrNotFound},
		{name: "query error", failAt: "query", wantErr: failure},
		{name: "tags error", failAt: "tags", filled: true, wantErr: failure},
		{name: "photos error", failAt: "photos", wantErr: failure},
		{name: "commit error", failAt: "commit", wantErr: failure},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Arrange
			db, mock := newSQLMock(t)
			repo := repository.NewProfileRepository(db)
			begin := mock.ExpectBegin()
			if tc.failAt == "begin" {
				begin.WillReturnError(failure)
			} else {
				query := mock.ExpectQuery(profileSelectSQL + " WHERE p.user_id = $1").WithArgs(int64(42))
				switch tc.failAt {
				case "missing":
					query.WillReturnRows(sqlmock.NewRows(profileRowColumns))
				case "query":
					query.WillReturnError(failure)
				default:
					query.WillReturnRows(profileRows(7, 42, tc.filled))
					if tc.filled {
						tags := mock.ExpectQuery(tagsSQL).WithArgs(int64(11))
						if tc.failAt == "tags" {
							tags.WillReturnError(failure)
						} else {
							tags.WillReturnRows(sqlmock.NewRows([]string{"id", "name"}).AddRow(int64(2), "music")).RowsWillBeClosed()
						}
					}
					if tc.failAt != "tags" {
						if tc.failAt == "photos" {
							mock.ExpectQuery(photosSQL).WithArgs(int64(7)).WillReturnError(failure)
						} else {
							expectPhotos(mock, 7, 2)
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
			got, err := repo.GetByUserIDCurrentProfile(context.Background(), 42)
			// Assert
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("err=%v want=%v", err, tc.wantErr)
			}
			if tc.wantErr != nil {
				if got != nil {
					t.Fatal("expected nil profile")
				}
				return
			}
			if got.ID != 7 || got.UserID != 42 || !got.CreatedAt.Equal(fixtureTime) || !got.UpdatedAt.Equal(fixtureTime) || got.HasSearchFilter != tc.filled {
				t.Fatalf("profile=%+v", got)
			}
			wantPhotos := []model.Photo{{ID: 1, StorageKey: photoKey(1), Position: 1}, {ID: 2, StorageKey: photoKey(2), Position: 2}}
			if !reflect.DeepEqual(got.Photos, wantPhotos) || got.CurrentPsycho != nil {
				t.Fatalf("relations=%+v", got)
			}
			if tc.filled {
				fields := got.CurrentVersion.ProfileFields
				if got.CurrentVersion.ID != 11 || got.CurrentVersion.ProfileID != 7 || *fields.Name != "Anna" || *fields.Height != 180 || *fields.Work != "Engineer" || fields.Alcohol != nil || *fields.Education != model.EducationHigher || *fields.Smoking != model.AttitudeNegative || !fields.BirthDate.Equal(fixtureBirth) || *fields.Sex != model.SexFemale || *fields.DatingGoal != model.DatingGoalFriendship {
					t.Fatalf("version=%+v", got.CurrentVersion)
				}
				if !reflect.DeepEqual(got.Tags, []model.Tag{{ID: 2, Name: "music"}}) {
					t.Fatalf("tags=%v", got.Tags)
				}
			} else if got.CurrentVersion.ID != 0 || got.CurrentVersion.Name != nil || got.Tags == nil || len(got.Tags) != 0 {
				t.Fatal("empty profile was not preserved")
			}
		})
	}
}

func TestProfileRepo_PatchProfile(t *testing.T) {
	failure := errors.New("write failed")
	for _, tc := range []struct {
		name, failAt           string
		replaceTags, clearTags bool
		wantErr                error
	}{
		{name: "inherit fields and tags"}, {name: "replace tags", replaceTags: true}, {name: "clear tags", clearTags: true},
		{name: "unknown tag", replaceTags: true, failAt: "unknown", wantErr: model.ErrUnknownTag},
		{name: "tag lookup error", replaceTags: true, failAt: "find", wantErr: failure},
		{name: "version error", failAt: "version", wantErr: failure},
		{name: "tag insert error", failAt: "tag", wantErr: failure},
		{name: "touch error", failAt: "touch", wantErr: failure},
		{name: "commit error", failAt: "commit", wantErr: failure},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Arrange
			db, mock := newSQLMock(t)
			repo := repository.NewProfileRepository(db)
			name := "Renamed"
			patch := &model.ProfilePatch{Name: &name}
			ids := []int64{1, 2}
			mock.ExpectBegin()
			expectProfileLoad(mock, true, true, 0)
			if tc.replaceTags {
				names := []string{"music", "coffee"}
				patch.Tags = &names
				ids = []int64{2, 1}
				query := mock.ExpectQuery(`SELECT id, name FROM tag WHERE name = ANY($1)`).WithArgs(names)
				if tc.failAt == "find" {
					query.WillReturnError(failure)
				} else {
					rows := sqlmock.NewRows([]string{"id", "name"}).AddRow(int64(2), "music")
					if tc.failAt != "unknown" {
						rows.AddRow(int64(1), "coffee")
					}
					query.WillReturnRows(rows).RowsWillBeClosed()
				}
			}
			if tc.clearTags {
				names := []string{}
				patch.Tags = &names
				ids = nil
			}
			if tc.failAt != "unknown" && tc.failAt != "find" {
				version := mock.ExpectQuery(insertVersionSQL).WithArgs(int64(7), name, fixtureBirth, "female", "friendship", nil, "higher", "Engineer", "negative", nil, 180)
				if tc.failAt == "version" {
					version.WillReturnError(failure)
				} else {
					version.WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(12)))
					for _, id := range ids {
						tag := mock.ExpectExec(`INSERT INTO profile_tag (profile_version_id, tag_id) VALUES ($1, $2)`).WithArgs(int64(12), id)
						if tc.failAt == "tag" {
							tag.WillReturnError(failure)
							break
						}
						tag.WillReturnResult(sqlmock.NewResult(0, 1))
					}
					if tc.failAt != "tag" {
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
			// Act
			err := repo.PatchProfile(context.Background(), 42, patch)
			// Assert
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("err=%v want=%v", err, tc.wantErr)
			}
		})
	}
}

func TestProfileRepo_AddPhoto(t *testing.T) {
	failure := errors.New("database unavailable")
	for _, tc := range []struct {
		name, failAt string
		count        int
		wantErr      error
	}{
		{name: "first photo"}, {name: "append photo", count: 2}, {name: "limit", count: model.MaxPhotos, wantErr: model.ErrPhotoLimit},
		{name: "begin error", failAt: "begin", wantErr: failure}, {name: "profile missing", failAt: "missing", wantErr: model.ErrNotFound},
		{name: "insert error", failAt: "insert", wantErr: failure}, {name: "touch error", failAt: "touch", wantErr: failure}, {name: "commit error", failAt: "commit", wantErr: failure},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Arrange
			db, mock := newSQLMock(t)
			repo := repository.NewProfileRepository(db)
			begin := mock.ExpectBegin()
			if tc.failAt == "begin" {
				begin.WillReturnError(failure)
			} else {
				if tc.failAt == "missing" {
					mock.ExpectQuery(lockProfileSQL).WithArgs(int64(42)).WillReturnRows(sqlmock.NewRows([]string{"id"}))
				} else {
					expectProfileLoad(mock, true, false, tc.count)
					if tc.count < model.MaxPhotos {
						insert := mock.ExpectExec(`INSERT INTO photo (profile_id, storage_key, position) VALUES ($1, $2, $3)`).WithArgs(int64(7), "new.jpg", tc.count+1)
						if tc.failAt == "insert" {
							insert.WillReturnError(failure)
						} else {
							insert.WillReturnResult(sqlmock.NewResult(0, 1))
							if tc.failAt == "touch" {
								mock.ExpectExec(touchSQL).WithArgs(int64(7)).WillReturnError(failure)
							} else {
								expectTouch(mock)
							}
						}
					}
				}
				if tc.failAt == "commit" {
					mock.ExpectCommit().WillReturnError(failure)
				} else if tc.wantErr != nil {
					mock.ExpectRollback()
				} else {
					mock.ExpectCommit()
				}
			}
			// Act
			err := repo.AddPhoto(context.Background(), 42, "new.jpg")
			// Assert
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("err=%v want=%v", err, tc.wantErr)
			}
		})
	}
}

func TestProfileRepo_DeletePhoto(t *testing.T) {
	failure := errors.New("write failed")
	for _, tc := range []struct {
		name, failAt string
		count        int
		filled       bool
		photoID      int64
		wantErr      error
	}{
		{name: "delete and shift", count: 3, photoID: 1},
		{name: "last photo of incomplete profile", count: 1, photoID: 1},
		{name: "last photo of complete profile", count: 1, filled: true, photoID: 1, wantErr: model.ErrLastPhoto},
		{name: "unknown photo", count: 2, photoID: 99, wantErr: model.ErrPhotoNotFound},
		{name: "delete error", count: 2, photoID: 1, failAt: "delete", wantErr: failure},
		{name: "shift error", count: 2, photoID: 1, failAt: "shift", wantErr: failure},
		{name: "touch error", count: 2, photoID: 1, failAt: "touch", wantErr: failure},
		{name: "commit error", count: 2, photoID: 1, failAt: "commit", wantErr: failure},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Arrange
			db, mock := newSQLMock(t)
			repo := repository.NewProfileRepository(db)
			mock.ExpectBegin()
			expectProfileLoad(mock, true, tc.filled, tc.count)
			if tc.wantErr != model.ErrPhotoNotFound && tc.wantErr != model.ErrLastPhoto {
				del := mock.ExpectExec(`DELETE FROM photo WHERE id = $1`).WithArgs(tc.photoID)
				if tc.failAt == "delete" {
					del.WillReturnError(failure)
				} else {
					del.WillReturnResult(sqlmock.NewResult(0, 1))
					shift := mock.ExpectExec(shiftSQL).WithArgs(int64(7), int(tc.photoID))
					if tc.failAt == "shift" {
						shift.WillReturnError(failure)
					} else {
						shift.WillReturnResult(sqlmock.NewResult(0, int64(tc.count-1)))
						if tc.failAt == "touch" {
							mock.ExpectExec(touchSQL).WithArgs(int64(7)).WillReturnError(failure)
						} else {
							expectTouch(mock)
						}
					}
				}
			}
			if tc.failAt == "commit" {
				mock.ExpectCommit().WillReturnError(failure)
			} else if tc.wantErr != nil {
				mock.ExpectRollback()
			} else {
				mock.ExpectCommit()
			}
			// Act
			key, err := repo.DeletePhoto(context.Background(), 42, tc.photoID)
			// Assert
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("err=%v want=%v", err, tc.wantErr)
			}
			if tc.wantErr != nil {
				if key != "" {
					t.Fatalf("unexpected key %q", key)
				}
				return
			}
			if key != photoKey(int(tc.photoID)) {
				t.Fatalf("key=%q", key)
			}
		})
	}
}

func TestProfileRepo_ReorderPhotos(t *testing.T) {
	failure := errors.New("write failed")
	for _, tc := range []struct {
		name, failAt string
		ids          []int64
		wantErr      error
	}{
		{name: "reverse", ids: []int64{3, 2, 1}}, {name: "same order", ids: []int64{1, 2, 3}},
		{name: "missing ID", ids: []int64{1, 2}, wantErr: model.ErrPhotoOrderMismatch},
		{name: "foreign ID", ids: []int64{1, 2, 99}, wantErr: model.ErrPhotoOrderMismatch},
		{name: "duplicate ID", ids: []int64{1, 1, 3}, wantErr: model.ErrPhotoOrderMismatch},
		{name: "update error", ids: []int64{3, 2, 1}, failAt: "update", wantErr: failure},
		{name: "touch error", ids: []int64{3, 2, 1}, failAt: "touch", wantErr: failure},
		{name: "commit error", ids: []int64{3, 2, 1}, failAt: "commit", wantErr: failure},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Arrange
			db, mock := newSQLMock(t)
			repo := repository.NewProfileRepository(db)
			mock.ExpectBegin()
			expectProfileLoad(mock, true, false, 3)
			if tc.wantErr != model.ErrPhotoOrderMismatch {
				update := mock.ExpectExec(reorderSQL).WithArgs(int64(7), tc.ids)
				if tc.failAt == "update" {
					update.WillReturnError(failure)
				} else {
					update.WillReturnResult(sqlmock.NewResult(0, 3))
					if tc.failAt == "touch" {
						mock.ExpectExec(touchSQL).WithArgs(int64(7)).WillReturnError(failure)
					} else {
						expectTouch(mock)
					}
				}
			}
			if tc.failAt == "commit" {
				mock.ExpectCommit().WillReturnError(failure)
			} else if tc.wantErr != nil {
				mock.ExpectRollback()
			} else {
				mock.ExpectCommit()
			}
			// Act
			err := repo.ReorderPhotos(context.Background(), 42, tc.ids)
			// Assert
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("err=%v want=%v", err, tc.wantErr)
			}
		})
	}
}
