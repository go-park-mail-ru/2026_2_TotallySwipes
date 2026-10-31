package repository

import (
	"context"
	"errors"
	"testing"

	"dating-app/internal/model"
	"dating-app/internal/repository"
	"github.com/DATA-DOG/go-sqlmock"
)

func TestMatchRepositoryGetParticipants(t *testing.T) {
	failure := errors.New("database unavailable")
	for _, name := range []string{"first participant", "second participant", "not found", "query error", "scan error", "row error"} {
		t.Run(name, func(t *testing.T) {
			db, mock := newSQLMock(t)
			userID := int64(10)
			if name == "second participant" {
				userID = 20
			}
			query := mock.ExpectQuery("SELECT user1_id, user2_id FROM match WHERE id = $1 AND (user1_id = $2 OR user2_id = $2);").WithArgs(int64(42), userID)
			var wantErr error
			rows := sqlmock.NewRows([]string{"user1_id", "user2_id"})
			switch name {
			case "not found":
				wantErr = model.ErrNotFound
				query.WillReturnRows(rows)
			case "query error":
				wantErr = failure
				query.WillReturnError(failure)
			case "scan error":
				query.WillReturnRows(rows.AddRow("not an integer", 20))
			case "row error":
				wantErr = failure
				query.WillReturnRows(rows.AddRow(10, 20).RowError(0, failure))
			default:
				query.WillReturnRows(rows.AddRow(10, 20))
			}
			got, err := repository.NewMatchRepository(db).GetParticipants(context.Background(), userID, 42)
			if name == "scan error" {
				if err == nil {
					t.Fatal("scan error swallowed")
				}
				return
			}
			if !errors.Is(err, wantErr) {
				t.Fatalf("error=%v want=%v", err, wantErr)
			}
			if wantErr == nil && got != [2]int64{10, 20} {
				t.Fatalf("participants=%v", got)
			}
		})
	}
}
