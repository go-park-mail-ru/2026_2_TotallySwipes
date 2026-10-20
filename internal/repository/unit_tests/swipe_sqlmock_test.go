package repository

import (
	"context"
	"dating-app/internal/model"
	"dating-app/internal/repository"
	"errors"
	"github.com/DATA-DOG/go-sqlmock"
	"testing"
)

func TestSwipeRepo_SaveSwipeAndMatch(t *testing.T) {
	failure := errors.New("database unavailable")
	for _, tc := range []struct {
		name, action, failAt string
		matched              int64
		wantErr              error
	}{
		{name: "like without mutual match", action: "like"}, {name: "mutual match", action: "like", matched: 1},
		{name: "super like", action: "super_like", matched: 1}, {name: "dislike", action: "dislike"},
		{name: "begin error", action: "like", failAt: "begin", wantErr: failure},
		{name: "target not found", action: "like", failAt: "missing", wantErr: model.ErrNotFound},
		{name: "target query error", action: "like", failAt: "query", wantErr: failure},
		{name: "swipe insert error", action: "like", failAt: "swipe", wantErr: failure},
		{name: "match insert error", action: "like", failAt: "match", wantErr: failure},
		{name: "commit error", action: "like", failAt: "commit", wantErr: failure},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Arrange
			db, mock := newSQLMock(t)
			repo := repository.NewSwipeRepository(db)
			swipe := &model.Swipe{AuthorID: 42, TargetID: 43, Action: model.SwipeAction(tc.action)}
			begin := mock.ExpectBegin()
			if tc.failAt == "begin" {
				begin.WillReturnError(failure)
			} else {
				target := mock.ExpectQuery(`SELECT id FROM profile WHERE user_id = $1`).WithArgs(int64(43))
				switch tc.failAt {
				case "missing":
					target.WillReturnRows(sqlmock.NewRows([]string{"id"}))
				case "query":
					target.WillReturnError(failure)
				default:
					target.WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(8)))
					save := mock.ExpectExec(insertSwipeSQL).WithArgs(int64(8), int64(42), tc.action)
					if tc.failAt == "swipe" {
						save.WillReturnError(failure)
					} else {
						save.WillReturnResult(sqlmock.NewResult(0, 1))
						match := mock.ExpectExec(insertMatchSQL).WithArgs(int64(42), int64(43), tc.action)
						if tc.failAt == "match" {
							match.WillReturnError(failure)
						} else {
							match.WillReturnResult(sqlmock.NewResult(0, tc.matched))
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
			err := repo.SaveSwipeAndMatch(context.Background(), swipe)
			// Assert
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("err=%v want=%v", err, tc.wantErr)
			}
		})
	}
}
