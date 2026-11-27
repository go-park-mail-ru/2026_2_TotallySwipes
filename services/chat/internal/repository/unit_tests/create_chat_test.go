package repository_test

import (
	"context"
	"database/sql"
	"testing"

	"dating-app/services/chat/internal/model"
	"github.com/DATA-DOG/go-sqlmock"
)

const createChatQuery = `
    INSERT INTO chat (match_id)
    VALUES ($1)
    ON CONFLICT (match_id) DO NOTHING
    RETURNING id`

const getChatByMatchQuery = `
    SELECT id
    FROM chat
    WHERE match_id = $1`

const createChatMembersQuery = `
    INSERT INTO chat_member (chat_id, user_id)
    VALUES ($1, $2), ($1, $3)`

const createChatSequenceQuery = `
    INSERT INTO chat_sequence (chat_id, last_seq)
    VALUES ($1, 0)`

const checkChatMembersQuery = `
    SELECT
        COUNT(*),
        COUNT(*) FILTER (WHERE user_id IN ($2, $3))
    FROM chat_member
    WHERE chat_id = $1`

func TestCreateChat(t *testing.T) {
	for _, tc := range []struct {
		name              string
		existing          bool
		firstID, secondID int64
	}{
		{"новый чат", false, 1, 2},
		{"существующий чат", true, 1, 2},
		{"участники в обратном порядке", true, 2, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r, mock := newRepository(t)
			mock.ExpectBegin()
			insert := mock.ExpectQuery(createChatQuery).WithArgs(int64(20))
			if tc.existing {
				insert.WillReturnRows(sqlmock.NewRows([]string{"id"}))
				mock.ExpectQuery(getChatByMatchQuery).WithArgs(int64(20)).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(10))
				mock.ExpectQuery(checkChatMembersQuery).WithArgs(int64(10), tc.firstID, tc.secondID).WillReturnRows(sqlmock.NewRows([]string{"total", "matched"}).AddRow(2, 2))
			} else {
				insert.WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(10))
				mock.ExpectExec(createChatMembersQuery).WithArgs(int64(10), tc.firstID, tc.secondID).WillReturnResult(sqlmock.NewResult(0, 2))
				mock.ExpectExec(createChatSequenceQuery).WithArgs(int64(10)).WillReturnResult(sqlmock.NewResult(0, 1))
				mock.ExpectExec(incrementChatSummaryVersionsQuery).WithArgs(int64(10), nil).WillReturnResult(sqlmock.NewResult(0, 2))
			}
			mock.ExpectCommit()
			id, created, err := r.CreateChat(context.Background(), 20, tc.firstID, tc.secondID)
			assertError(t, err, nil)
			if id != 10 || created == tc.existing {
				t.Fatalf("id = %d, created = %v", id, created)
			}
		})
	}
}

func TestCreateChatInvalidMembers(t *testing.T) {
	for _, tc := range []struct {
		name                     string
		total, matched, secondID int64
	}{
		{"другой участник", 2, 1, 3},
		{"нет участников", 0, 0, 2},
		{"лишний участник", 3, 2, 2},
		{"одинаковые ID", 2, 2, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r, mock := newRepository(t)
			mock.ExpectBegin()
			mock.ExpectQuery(createChatQuery).WithArgs(int64(20)).WillReturnError(sql.ErrNoRows)
			mock.ExpectQuery(getChatByMatchQuery).WithArgs(int64(20)).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(10))
			mock.ExpectQuery(checkChatMembersQuery).WithArgs(int64(10), int64(1), tc.secondID).WillReturnRows(sqlmock.NewRows([]string{"total", "matched"}).AddRow(tc.total, tc.matched))
			mock.ExpectRollback()
			id, created, err := r.CreateChat(context.Background(), 20, 1, tc.secondID)
			assertError(t, err, model.ErrInvalidChatMembers)
			if id != 0 || created {
				t.Fatalf("id = %d, created = %v", id, created)
			}
		})
	}
}

func TestCreateChatErrors(t *testing.T) {
	for _, stage := range []string{"begin", "insert", "members", "sequence", "lookup", "check members", "commit new", "commit existing", "summary"} {
		t.Run(stage, func(t *testing.T) {
			r, mock := newRepository(t)
			begin := mock.ExpectBegin()
			if stage == "begin" {
				begin.WillReturnError(failure)
			} else {
				existing := stage == "lookup" || stage == "check members" || stage == "commit existing"
				insert := mock.ExpectQuery(createChatQuery).WithArgs(int64(20))
				if stage == "insert" {
					insert.WillReturnError(failure)
				} else if existing {
					insert.WillReturnError(sql.ErrNoRows)
					lookup := mock.ExpectQuery(getChatByMatchQuery).WithArgs(int64(20))
					if stage == "lookup" {
						lookup.WillReturnError(failure)
					} else {
						lookup.WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(10))
						check := mock.ExpectQuery(checkChatMembersQuery).WithArgs(int64(10), int64(1), int64(2))
						if stage == "check members" {
							check.WillReturnError(failure)
						} else {
							check.WillReturnRows(sqlmock.NewRows([]string{"total", "matched"}).AddRow(2, 2))
						}
					}
				} else {
					insert.WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(10))
					members := mock.ExpectExec(createChatMembersQuery).WithArgs(int64(10), int64(1), int64(2))
					if stage == "members" {
						members.WillReturnError(failure)
					} else {
						members.WillReturnResult(sqlmock.NewResult(0, 2))
						sequence := mock.ExpectExec(createChatSequenceQuery).WithArgs(int64(10))
						if stage == "sequence" {
							sequence.WillReturnError(failure)
						} else {
							sequence.WillReturnResult(sqlmock.NewResult(0, 1))
							versions := mock.ExpectExec(incrementChatSummaryVersionsQuery).WithArgs(int64(10), nil)
							if stage == "summary" {
								versions.WillReturnError(failure)
							} else {
								versions.WillReturnResult(sqlmock.NewResult(0, 2))
							}
						}
					}
				}
				if stage == "commit new" || stage == "commit existing" {
					mock.ExpectCommit().WillReturnError(failure)
				} else {
					mock.ExpectRollback()
				}
			}
			id, created, err := r.CreateChat(context.Background(), 20, 1, 2)
			assertError(t, err, failure)
			if id != 0 || created {
				t.Fatalf("id = %d, created = %v", id, created)
			}
		})
	}
}
