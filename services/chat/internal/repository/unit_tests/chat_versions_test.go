package repository_test

import (
	"context"
	"database/sql"
	"reflect"
	"testing"

	"dating-app/services/chat/internal/model"
	"github.com/DATA-DOG/go-sqlmock"
)

func expectMessageVersionUpdates(mock sqlmock.Sqlmock) {
	mock.ExpectExec(incrementChatCardVersionsQuery).WithArgs(int64(10)).WillReturnResult(sqlmock.NewResult(0, 2))
	mock.ExpectExec(incrementChatSummaryVersionsQuery).WithArgs(int64(10), int64(1)).WillReturnResult(sqlmock.NewResult(0, 1))
}

func TestMarkSeen(t *testing.T) {
	for _, stage := range []string{"успех", "повтор", "нет участника", "begin", "lock", "update", "rows", "summary", "commit"} {
		t.Run(stage, func(t *testing.T) {
			r, mock := newRepository(t)
			wantErr := error(nil)
			begin := mock.ExpectBegin()
			if stage == "begin" {
				begin.WillReturnError(failure)
				wantErr = failure
			} else {
				lock := mock.ExpectQuery(lockChatSequenceQuery).WithArgs(int64(1), int64(10))
				switch stage {
				case "нет участника":
					lock.WillReturnError(sql.ErrNoRows)
					wantErr = model.ErrChatNotFound
				case "lock":
					lock.WillReturnError(failure)
					wantErr = failure
				default:
					lock.WillReturnRows(sqlmock.NewRows([]string{"last_seq"}).AddRow(5))
				}
				if wantErr == nil {
					update := mock.ExpectExec(markSeenQuery).WithArgs(int64(1), int64(10))
					switch stage {
					case "update":
						update.WillReturnError(failure)
						wantErr = failure
					case "rows":
						update.WillReturnResult(sqlmock.NewErrorResult(failure))
						wantErr = failure
					case "повтор":
						update.WillReturnResult(sqlmock.NewResult(0, 0))
					default:
						update.WillReturnResult(sqlmock.NewResult(0, 1))
					}
					if wantErr == nil && stage != "повтор" {
						version := mock.ExpectExec(incrementUserSummaryVersionQuery).WithArgs(int64(1))
						if stage == "summary" {
							version.WillReturnError(failure)
							wantErr = failure
						} else {
							version.WillReturnResult(sqlmock.NewResult(0, 1))
						}
					}
				}
				if wantErr != nil {
					mock.ExpectRollback()
				} else {
					commit := mock.ExpectCommit()
					if stage == "commit" {
						commit.WillReturnError(failure)
						wantErr = failure
					}
				}
			}
			assertError(t, r.MarkSeen(context.Background(), 1, 10), wantErr)
		})
	}
}

func TestReadChat(t *testing.T) {
	for _, stage := range []string{"успех", "старая команда", "повтор", "нулевая позиция", "позиция за концом", "отрицательная позиция", "отключён", "нет доступа", "begin", "lock", "read", "update", "карточки", "summary", "commit"} {
		t.Run(stage, func(t *testing.T) {
			r, mock := newRepository(t)
			command := model.ChatRead{ChatID: 10, LastReadSeq: 5}
			switch stage {
			case "старая команда":
				command.LastReadSeq = 2
			case "повтор":
				command.LastReadSeq = 3
			case "нулевая позиция":
				command.LastReadSeq = 0
			case "позиция за концом":
				command.LastReadSeq = 6
			case "отрицательная позиция":
				command.LastReadSeq = -1
			}
			wantErr := error(nil)
			begin := mock.ExpectBegin()
			if stage == "begin" {
				begin.WillReturnError(failure)
				wantErr = failure
			} else {
				lock := mock.ExpectQuery(lockChatSequenceQuery).WithArgs(int64(1), int64(10))
				switch stage {
				case "нет доступа":
					lock.WillReturnError(sql.ErrNoRows)
					wantErr = model.ErrChatNotFound
				case "lock":
					lock.WillReturnError(failure)
					wantErr = failure
				default:
					lock.WillReturnRows(sqlmock.NewRows([]string{"last_seq"}).AddRow(5))
				}
				if wantErr == nil {
					position := mock.ExpectQuery(getReadPositionQuery).WithArgs(int64(1), int64(10))
					if stage == "read" {
						position.WillReturnError(failure)
						wantErr = failure
					} else {
						position.WillReturnRows(sqlmock.NewRows([]string{"last_read_seq", "disabled"}).AddRow(3, stage == "отключён"))
					}
					if stage == "отключён" {
						wantErr = model.ErrChatMemberBlocked
					}
					if stage == "позиция за концом" || stage == "отрицательная позиция" {
						wantErr = model.ErrInvalidReadSeq
					}
				}
				if wantErr == nil && command.LastReadSeq > 3 {
					update := mock.ExpectQuery(readChatQuery).WithArgs(int64(1), int64(10), command.LastReadSeq)
					if stage == "update" {
						update.WillReturnError(failure)
						wantErr = failure
					} else {
						update.WillReturnRows(sqlmock.NewRows([]string{"last_read_seq"}).AddRow(5))
						cards := mock.ExpectExec(incrementChatCardVersionsQuery).WithArgs(int64(10))
						if stage == "карточки" {
							cards.WillReturnError(failure)
							wantErr = failure
						} else {
							cards.WillReturnResult(sqlmock.NewResult(0, 2))
							summary := mock.ExpectExec(incrementUserSummaryVersionQuery).WithArgs(int64(1))
							if stage == "summary" {
								summary.WillReturnError(failure)
								wantErr = failure
							} else {
								summary.WillReturnResult(sqlmock.NewResult(0, 1))
							}
						}
					}
				}
				if wantErr != nil {
					mock.ExpectRollback()
				} else {
					commit := mock.ExpectCommit()
					if stage == "commit" {
						commit.WillReturnError(failure)
						wantErr = failure
					}
				}
			}
			got, err := r.ReadChat(context.Background(), 1, command)
			assertError(t, err, wantErr)
			wantSeq := int64(5)
			if command.LastReadSeq <= 3 {
				wantSeq = 3
			}
			if err == nil && !reflect.DeepEqual(got, &model.ChatReadUpdated{ChatID: 10, UserID: 1, LastReadSeq: wantSeq}) {
				t.Fatalf("read = %#v", got)
			}
		})
	}
}

func TestMessageVersionFailureRollsBack(t *testing.T) {
	for _, stage := range []string{"карточки", "summary"} {
		t.Run(stage, func(t *testing.T) {
			r, mock := newRepository(t)
			mock.ExpectBegin()
			mock.ExpectQuery(lockChatSequenceQuery).WithArgs(int64(1), int64(10)).WillReturnRows(sqlmock.NewRows([]string{"last_seq"}).AddRow(4))
			mock.ExpectQuery(findClientMessageQuery).WithArgs(int64(10), int64(1), "key").WillReturnRows(messageRows())
			mock.ExpectQuery(canSendMessageQuery).WithArgs(int64(1), int64(10)).WillReturnRows(sqlmock.NewRows([]string{"allowed"}).AddRow(true))
			mock.ExpectQuery(createMessageQuery).WithArgs(int64(10), int64(1), "key", "Привет").WillReturnRows(messageRows(5))
			cards := mock.ExpectExec(incrementChatCardVersionsQuery).WithArgs(int64(10))
			if stage == "карточки" {
				cards.WillReturnError(failure)
			} else {
				cards.WillReturnResult(sqlmock.NewResult(0, 2))
				mock.ExpectExec(incrementChatSummaryVersionsQuery).WithArgs(int64(10), int64(1)).WillReturnError(failure)
			}
			mock.ExpectRollback()
			got, created, err := r.CreateMessage(context.Background(), 1, model.ChatSendMessage{ChatID: 10, ClientMessageID: "key", Text: "Привет"})
			assertError(t, err, failure)
			if got != nil || created {
				t.Fatalf("message = %#v, created = %v", got, created)
			}
		})
	}
}
