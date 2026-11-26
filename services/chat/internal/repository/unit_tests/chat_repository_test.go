package repository_test

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"reflect"
	"testing"
	"time"

	"dating-app/services/chat/internal/model"
	"dating-app/services/chat/internal/repository"
	"github.com/DATA-DOG/go-sqlmock"
)

var failure = errors.New("database failure")
var now = time.Date(2026, 11, 26, 12, 0, 0, 0, time.UTC)
var messageColumns = []string{"id", "chat_id", "client_message_id", "seq", "sender_id", "type", "body", "storage_key", "duration_seconds", "created_at", "updated_at", "edited_at", "deleted_at", "version"}

func ptr[T any](value T) *T { return &value }

func newRepository(t *testing.T) (*repository.ChatRepository, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Error(err)
		}
		db.Close()
	})
	return repository.NewChatRepository(db), mock
}

func assertError(t *testing.T, got, want error) {
	t.Helper()
	if !errors.Is(got, want) {
		t.Fatalf("error = %v, want %v", got, want)
	}
}

func messageValues(seq int64) []driver.Value {
	return []driver.Value{100 + seq, int64(10), "key", seq, int64(1), "text", "Привет", nil, nil, now, now, nil, nil, int64(1)}
}

func messageRows(seqs ...int64) *sqlmock.Rows {
	rows := sqlmock.NewRows(messageColumns)
	for _, seq := range seqs {
		rows.AddRow(messageValues(seq)...)
	}
	return rows
}

var stateColumns = []string{"id", "match_id", "created_at", "viewer_id", "viewer_seen", "viewer_read", "peer_id", "peer_seen", "peer_read", "viewer_disabled", "peer_disabled", "match_exists", "allows_messaging", "viewer_block", "peer_block", "last_seq", "unread", "last_activity", "message_id", "message_seq", "sender_id", "type", "preview", "duration", "message_created", "read_by_peer"}

func stateValues() []driver.Value {
	return []driver.Value{int64(10), int64(20), now, int64(1), nil, int64(3), int64(2), now, int64(4), false, false, true, true, false, false, int64(5), int64(2), now, nil, nil, nil, nil, nil, nil, nil, nil}
}

func TestGetChat(t *testing.T) {
	for _, tc := range []struct {
		name           string
		rows           *sqlmock.Rows
		dbErr, wantErr error
	}{
		{"успех", sqlmock.NewRows([]string{"id", "match_id", "created_at"}).AddRow(10, 20, now), nil, nil},
		{"нет чата", sqlmock.NewRows([]string{"id", "match_id", "created_at"}), nil, model.ErrChatNotFound},
		{"ошибка БД", nil, failure, failure},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r, mock := newRepository(t)
			expect := mock.ExpectQuery(getChatQuery).WithArgs(int64(10))
			if tc.dbErr != nil {
				expect.WillReturnError(tc.dbErr)
			} else {
				expect.WillReturnRows(tc.rows).RowsWillBeClosed()
			}
			got, err := r.GetChat(context.Background(), 10)
			assertError(t, err, tc.wantErr)
			if err == nil && !reflect.DeepEqual(got, &model.Chat{ID: 10, MatchID: 20, CreatedAt: now}) {
				t.Fatalf("chat = %#v", got)
			}
		})
	}
}

func TestGetMembers(t *testing.T) {
	for _, tc := range []struct {
		name           string
		rows           *sqlmock.Rows
		dbErr, wantErr error
	}{
		{"участники", sqlmock.NewRows([]string{"chat_id", "user_id", "seen_at", "last_read_seq"}).AddRow(10, 1, nil, 3).AddRow(10, 2, now, 4), nil, nil},
		{"пустой результат", sqlmock.NewRows([]string{"chat_id", "user_id", "seen_at", "last_read_seq"}), nil, nil},
		{"ошибка запроса", nil, failure, failure},
		{"ошибка итерации", sqlmock.NewRows([]string{"chat_id", "user_id", "seen_at", "last_read_seq"}).AddRow(10, 1, nil, 0).RowError(0, failure), nil, failure},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r, mock := newRepository(t)
			expect := mock.ExpectQuery(getMembersQuery).WithArgs(int64(10))
			if tc.dbErr != nil {
				expect.WillReturnError(tc.dbErr)
			} else {
				expect.WillReturnRows(tc.rows).RowsWillBeClosed()
			}
			got, err := r.GeyMembers(context.Background(), 10)
			assertError(t, err, tc.wantErr)
			if tc.name == "участники" {
				want := []model.ChatMember{{ChatID: 10, UserID: 1, LastReadSeq: 3}, {ChatID: 10, UserID: 2, SeenAt: &now, LastReadSeq: 4}}
				if !reflect.DeepEqual(got, want) {
					t.Fatalf("members = %#v", got)
				}
			}
			if tc.name == "пустой результат" && (got == nil || len(got) != 0) {
				t.Fatalf("members = %#v", got)
			}
		})
	}
}

func TestGetSummary(t *testing.T) {
	for _, dbErr := range []error{nil, failure} {
		t.Run("summary/"+errorName(dbErr), func(t *testing.T) {
			r, mock := newRepository(t)
			expect := mock.ExpectQuery(getSummaryQuery).WithArgs(int64(1))
			if dbErr != nil {
				expect.WillReturnError(dbErr)
			} else {
				expect.WillReturnRows(sqlmock.NewRows([]string{"new", "unread_chats", "unread_messages"}).AddRow(2, 3, 7))
			}
			got, err := r.GetSummary(context.Background(), 1)
			assertError(t, err, dbErr)
			if err == nil && !reflect.DeepEqual(got, &model.ChatSummary{NewChatsCount: 2, UnreadChatsCount: 3, UnreadMessagesCount: 7}) {
				t.Fatalf("summary = %#v", got)
			}
		})
	}
}

func errorName(err error) string {
	if err != nil {
		return "ошибка"
	}
	return "успех"
}

func TestGetChatState(t *testing.T) {
	for _, name := range []string{"пустой чат", "последнее сообщение", "нет доступа", "ошибка БД"} {
		t.Run(name, func(t *testing.T) {
			r, mock := newRepository(t)
			values := stateValues()
			if name == "последнее сообщение" {
				copy(values[18:], []driver.Value{int64(105), int64(5), int64(1), "text", "Привет", nil, now, true})
			}
			rows := sqlmock.NewRows(stateColumns)
			if name != "нет доступа" {
				rows.AddRow(values...)
			}
			expect := mock.ExpectQuery(getChatQueryState).WithArgs(int64(1), int64(10))
			if name == "ошибка БД" {
				expect.WillReturnError(failure)
			} else {
				expect.WillReturnRows(rows).RowsWillBeClosed()
			}
			got, err := r.GetChatState(context.Background(), 1, 10)
			if name == "нет доступа" {
				assertError(t, err, model.ErrChatNotFound)
				return
			}
			if name == "ошибка БД" {
				assertError(t, err, failure)
				return
			}
			assertError(t, err, nil)
			if got.Chat.ID != 10 || got.Viewer.ChatID != 10 || got.Peer.ChatID != 10 || got.Viewer.SeenAt != nil || got.Peer.SeenAt == nil || !got.MatchAllowsMessaging || got.LastSeq != 5 || got.UnreadCount != 2 || !got.LastActivityAt.Equal(now) {
				t.Fatalf("state = %#v", got)
			}
			if name == "пустой чат" && got.LastMessage != nil {
				t.Fatalf("last message = %#v", got.LastMessage)
			}
			if name == "последнее сообщение" {
				want := &model.ChatLastMessage{MessageID: 105, Seq: 5, SenderID: ptr(int64(1)), Type: model.ChatMessageTypeText, TextPreview: ptr("Привет"), CreatedAt: now, IsReadByPeer: ptr(true)}
				if !reflect.DeepEqual(got.LastMessage, want) {
					t.Fatalf("last message = %#v", got.LastMessage)
				}
			}
		})
	}
}

func TestGetMessage(t *testing.T) {
	for _, name := range []string{"текст", "медиа", "нет сообщения в чате", "ошибка БД", "ошибка сканирования"} {
		t.Run(name, func(t *testing.T) {
			r, mock := newRepository(t)
			values := messageValues(5)
			if name == "медиа" {
				values[5], values[6], values[7], values[8] = "voice", nil, "voice/key", 2.5
			}
			if name == "ошибка сканирования" {
				values[0] = "invalid id"
			}
			rows := sqlmock.NewRows(messageColumns)
			if name != "нет сообщения в чате" {
				rows.AddRow(values...)
			}
			expect := mock.ExpectQuery(getMessageQuery).WithArgs(int64(10), int64(105))
			if name == "ошибка БД" {
				expect.WillReturnError(failure)
			} else {
				expect.WillReturnRows(rows).RowsWillBeClosed()
			}
			got, err := r.GetMessage(context.Background(), 10, 105)
			if name == "нет сообщения в чате" {
				assertError(t, err, model.ErrMessageNotFound)
				return
			}
			if name == "ошибка БД" {
				assertError(t, err, failure)
				return
			}
			if name == "ошибка сканирования" {
				if err == nil {
					t.Fatal("expected scan error")
				}
				return
			}
			assertError(t, err, nil)
			want := &model.StoredMessage{ID: 105, ChatID: 10, ClientMessageID: ptr("key"), Seq: 5, SenderID: ptr(int64(1)), Type: model.ChatMessageTypeText, Text: ptr("Привет"), CreatedAt: now, UpdatedAt: now, Version: 1}
			if name == "медиа" {
				want.Type, want.Text, want.StorageKey, want.DurationSeconds = model.ChatMessageTypeVoice, nil, ptr("voice/key"), ptr(2.5)
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("message = %#v, want %#v", got, want)
			}
		})
	}
}

func TestCreateMessage(t *testing.T) {
	for _, name := range []string{"новое", "повтор", "конфликт", "чужой чат", "запрещено", "ошибка начала", "ошибка блокировки", "ошибка поиска", "ошибка доступа", "ошибка вставки", "ошибка commit", "ошибка commit повтора"} {
		t.Run(name, func(t *testing.T) {
			r, mock := newRepository(t)
			command := model.ChatSendMessage{ChatID: 10, ClientMessageID: "key", Text: "Привет"}
			wantErr := error(nil)
			begin := mock.ExpectBegin()
			if name == "ошибка начала" {
				begin.WillReturnError(failure)
				wantErr = failure
			} else {
				lock := mock.ExpectQuery(lockChatSequenceQuery).WithArgs(int64(1), int64(10))
				if name == "чужой чат" {
					lock.WillReturnError(sql.ErrNoRows)
					wantErr = model.ErrChatNotFound
				} else if name == "ошибка блокировки" {
					lock.WillReturnError(failure)
					wantErr = failure
				} else {
					lock.WillReturnRows(sqlmock.NewRows([]string{"last_seq"}).AddRow(4))
					find := mock.ExpectQuery(findClientMessageQuery).WithArgs(int64(10), int64(1), "key")
					switch name {
					case "повтор", "конфликт", "ошибка commit повтора":
						values := messageValues(5)
						if name == "конфликт" {
							values[6] = "Другой текст"
							wantErr = model.ErrIdempotencyConflict
						}
						find.WillReturnRows(sqlmock.NewRows(messageColumns).AddRow(values...))
					case "ошибка поиска":
						find.WillReturnError(failure)
						wantErr = failure
					default:
						find.WillReturnRows(messageRows())
						access := mock.ExpectQuery(canSendMessageQuery).WithArgs(int64(1), int64(10))
						if name == "ошибка доступа" {
							access.WillReturnError(failure)
							wantErr = failure
						} else {
							access.WillReturnRows(sqlmock.NewRows([]string{"allowed"}).AddRow(name != "запрещено"))
							if name == "запрещено" {
								wantErr = model.ErrMessagingNotAllowed
							} else {
								insert := mock.ExpectQuery(createMessageQuery).WithArgs(int64(10), int64(1), "key", "Привет")
								if name == "ошибка вставки" {
									insert.WillReturnError(failure)
									wantErr = failure
								} else {
									insert.WillReturnRows(messageRows(5))
								}
							}
						}
					}
				}
				if wantErr != nil {
					mock.ExpectRollback()
				} else {
					commit := mock.ExpectCommit()
					if name == "ошибка commit" || name == "ошибка commit повтора" {
						commit.WillReturnError(failure)
						wantErr = failure
					}
				}
			}
			got, created, err := r.CreateMessage(context.Background(), 1, command)
			assertError(t, err, wantErr)
			if err != nil {
				if got != nil || created {
					t.Fatalf("message = %#v, created = %v", got, created)
				}
				return
			}
			if got.ID != 105 || got.Seq != 5 || *got.Text != command.Text || created != (name == "новое") {
				t.Fatalf("message = %#v, created = %v", got, created)
			}
		})
	}
}

func TestGetMessages(t *testing.T) {
	for _, tc := range []struct {
		name           string
		query          model.ChatMessagesQuery
		seqs, wantSeqs []int64
		more           bool
		cursor         *int64
	}{
		{"старые", model.ChatMessagesQuery{Limit: ptr(2), BeforeSeq: ptr(int64(6))}, []int64{5, 4, 3}, []int64{4, 5}, true, ptr(int64(4))},
		{"новые", model.ChatMessagesQuery{Limit: ptr(2), AfterSeq: ptr(int64(2))}, []int64{3, 4, 5}, []int64{3, 4}, true, ptr(int64(4))},
		{"последняя страница", model.ChatMessagesQuery{Limit: ptr(2)}, []int64{2, 1}, []int64{1, 2}, false, nil},
		{"пустая", model.ChatMessagesQuery{}, nil, []int64{}, false, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r, mock := newRepository(t)
			query, direction := getOlderMessagesQuery, model.ChatPageDirectionOlder
			if tc.query.AfterSeq != nil {
				query, direction = getNewerMessagesQuery, model.ChatPageDirectionNewer
			}
			limit := 50
			if tc.query.Limit != nil {
				limit = *tc.query.Limit
			}
			mock.ExpectQuery(query).WithArgs(int64(1), int64(10), tc.query.BeforeSeq, tc.query.AfterSeq, limit+1).WillReturnRows(messageRows(tc.seqs...)).RowsWillBeClosed()
			got, err := r.GetMessages(context.Background(), 1, 10, tc.query)
			assertError(t, err, nil)
			seqs := make([]int64, 0, len(got.Items))
			for _, item := range got.Items {
				seqs = append(seqs, item.Seq)
			}
			if !reflect.DeepEqual(seqs, tc.wantSeqs) || got.HasMore != tc.more || got.Direction != direction || !reflect.DeepEqual(got.NextCursor, tc.cursor) {
				t.Fatalf("page = %#v, seqs = %v", got, seqs)
			}
		})
	}
}

func TestGetMessagesErrors(t *testing.T) {
	for _, name := range []string{"запрос", "сканирование", "итерация"} {
		t.Run(name, func(t *testing.T) {
			r, mock := newRepository(t)
			expect := mock.ExpectQuery(getOlderMessagesQuery).WithArgs(int64(1), int64(10), nil, nil, 51)
			if name == "запрос" {
				expect.WillReturnError(failure)
			} else {
				values := messageValues(5)
				if name == "сканирование" {
					values[0] = "invalid id"
				}
				rows := sqlmock.NewRows(messageColumns).AddRow(values...)
				if name == "итерация" {
					rows.RowError(0, failure)
				}
				expect.WillReturnRows(rows).RowsWillBeClosed()
			}
			got, err := r.GetMessages(context.Background(), 1, 10, model.ChatMessagesQuery{})
			if err == nil || got != nil {
				t.Fatalf("page = %#v, error = %v", got, err)
			}
			if name != "сканирование" {
				assertError(t, err, failure)
			}
		})
	}
}

func TestMarkSeen(t *testing.T) {
	for _, name := range []string{"успех", "нет участника", "ошибка запроса", "ошибка результата"} {
		t.Run(name, func(t *testing.T) {
			r, mock := newRepository(t)
			expect := mock.ExpectExec(markSeenQuery).WithArgs(int64(1), int64(10))
			wantErr := error(nil)
			switch name {
			case "нет участника":
				expect.WillReturnResult(sqlmock.NewResult(0, 0))
				wantErr = model.ErrChatNotFound
			case "ошибка запроса":
				expect.WillReturnError(failure)
				wantErr = failure
			case "ошибка результата":
				expect.WillReturnResult(sqlmock.NewErrorResult(failure))
				wantErr = failure
			default:
				expect.WillReturnResult(sqlmock.NewResult(0, 1))
			}
			assertError(t, r.MarkSeen(context.Background(), 1, 10), wantErr)
		})
	}
}

func TestReadChat(t *testing.T) {
	for _, name := range []string{"успех", "старая команда", "нулевая позиция", "позиция за концом", "отрицательная позиция", "отключён", "нет доступа", "ошибка состояния", "ошибка обновления", "участник удалён"} {
		t.Run(name, func(t *testing.T) {
			r, mock := newRepository(t)
			command := model.ChatRead{ChatID: 10, LastReadSeq: 5}
			if name == "старая команда" {
				command.LastReadSeq = 2
			}
			if name == "нулевая позиция" {
				command.LastReadSeq = 0
			}
			if name == "позиция за концом" {
				command.LastReadSeq = 6
			}
			if name == "отрицательная позиция" {
				command.LastReadSeq = -1
			}
			values := stateValues()
			if name == "отключён" {
				values[9] = true
			}
			state := mock.ExpectQuery(getChatQueryState).WithArgs(int64(1), int64(10))
			wantErr := error(nil)
			switch name {
			case "нет доступа":
				state.WillReturnError(sql.ErrNoRows)
				wantErr = model.ErrChatNotFound
			case "ошибка состояния":
				state.WillReturnError(failure)
				wantErr = failure
			default:
				state.WillReturnRows(sqlmock.NewRows(stateColumns).AddRow(values...))
			}
			if name == "отключён" {
				wantErr = model.ErrChatMemberBlocked
			}
			if name == "позиция за концом" || name == "отрицательная позиция" {
				wantErr = model.ErrInvalidReadSeq
			}
			wantSeq := int64(5)
			if command.LastReadSeq < 3 {
				wantSeq = 3
			}
			if wantErr == nil {
				update := mock.ExpectQuery(readChatQuery).WithArgs(int64(1), int64(10), command.LastReadSeq)
				switch name {
				case "ошибка обновления":
					update.WillReturnError(failure)
					wantErr = failure
				case "участник удалён":
					update.WillReturnError(sql.ErrNoRows)
					wantErr = model.ErrChatNotFound
				default:
					update.WillReturnRows(sqlmock.NewRows([]string{"last_read_seq"}).AddRow(wantSeq))
				}
			}
			got, err := r.ReadChat(context.Background(), 1, command)
			assertError(t, err, wantErr)
			if err == nil && !reflect.DeepEqual(got, &model.ChatReadUpdated{ChatID: 10, UserID: 1, LastReadSeq: wantSeq}) {
				t.Fatalf("read = %#v", got)
			}
		})
	}
}
