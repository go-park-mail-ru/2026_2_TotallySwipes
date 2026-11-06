package repository

import (
	"context"
	"dating-app/internal/model"
	"dating-app/internal/repository"
	"errors"
	"github.com/DATA-DOG/go-sqlmock"
	"github.com/jackc/pgx/v5/pgconn"
	"reflect"
	"testing"
)

func TestUserRepo_CreateUser(t *testing.T) {
	failure := errors.New("database unavailable")
	for _, tc := range []struct {
		name, failAt string
		wantErr      error
	}{
		{name: "success"}, {name: "begin error", failAt: "begin", wantErr: failure},
		{name: "duplicate email", failAt: "duplicate", wantErr: model.ErrEmailAlreadyExists},
		{name: "user insert error", failAt: "user", wantErr: failure},
		{name: "profile insert error", failAt: "profile", wantErr: failure},
		{name: "commit error", failAt: "commit", wantErr: failure},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Arrange
			db, mock := newSQLMock(t)
			repo := repository.NewUserRepository(db)
			input := &model.UserInput{Email: "user@example.com", PasswordHash: "hash"}
			begin := mock.ExpectBegin()
			if tc.failAt == "begin" {
				begin.WillReturnError(failure)
			} else {
				insert := mock.ExpectQuery(`INSERT INTO "user" (email, password_hash) VALUES ($1, $2) RETURNING id`).WithArgs(input.Email, input.PasswordHash)
				switch tc.failAt {
				case "duplicate":
					insert.WillReturnError(&pgconn.PgError{Code: "23505"})
				case "user":
					insert.WillReturnError(failure)
				default:
					insert.WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(42)))
					profile := mock.ExpectExec(`INSERT INTO profile (user_id) VALUES ($1)`).WithArgs(int64(42))
					if tc.failAt == "profile" {
						profile.WillReturnError(failure)
					} else {
						profile.WillReturnResult(sqlmock.NewResult(0, 1))
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
			id, err := repo.CreateUser(context.Background(), input)
			// Assert
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("err=%v want=%v", err, tc.wantErr)
			}
			wantID := int64(42)
			if tc.wantErr != nil {
				wantID = 0
			}
			if id != wantID {
				t.Fatalf("id=%d want=%d", id, wantID)
			}
		})
	}
}

func TestUserRepo_GetUserByEmail(t *testing.T) {
	failure := errors.New("database unavailable")
	for _, tc := range []struct {
		name           string
		empty          bool
		dbErr, wantErr error
	}{
		{name: "found"}, {name: "not found", empty: true, wantErr: model.ErrNotFound}, {name: "database error", dbErr: failure, wantErr: failure},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Arrange
			db, mock := newSQLMock(t)
			repo := repository.NewUserRepository(db)
			want := &model.User{ID: 42, Email: "user@example.com", PasswordHash: "hash"}
			query := mock.ExpectQuery(`SELECT id, email, password_hash FROM "user" WHERE email = $1`).WithArgs(want.Email)
			rows := sqlmock.NewRows([]string{"id", "email", "password_hash"})
			if !tc.empty {
				rows.AddRow(want.ID, want.Email, want.PasswordHash)
			}
			if tc.dbErr != nil {
				query.WillReturnError(tc.dbErr)
			} else {
				query.WillReturnRows(rows)
			}
			// Act
			got, err := repo.GetUserByEmail(context.Background(), want.Email)
			// Assert
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("err=%v want=%v", err, tc.wantErr)
			}
			if tc.wantErr != nil {
				if got != nil {
					t.Fatal("expected nil user")
				}
				return
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("got=%+v want=%+v", got, want)
			}
		})
	}
}
