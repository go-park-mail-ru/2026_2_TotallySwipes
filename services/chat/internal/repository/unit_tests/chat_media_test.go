package repository_test

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"strings"
	"testing"

	"dating-app/services/chat/internal/model"
	"github.com/DATA-DOG/go-sqlmock"
)

func mediaValues() []driver.Value {
	values := messageValues(5)
	values[5], values[6], values[7], values[8] = "voice", nil, "voice/key", 2.5
	return values
}

func mediaLookupRows(digest string) *sqlmock.Rows {
	columns := append(append([]string{}, messageColumns...), "media_sha256")
	return sqlmock.NewRows(columns).AddRow(append(mediaValues(), digest)...)
}

func TestGetMediaByClientID(t *testing.T) {
	for _, name := range []string{"найдено", "нет сообщения", "ошибка БД"} {
		t.Run(name, func(t *testing.T) {
			r, mock := newRepository(t)
			query := mock.ExpectQuery(findClientMediaQuery).WithArgs(int64(10), int64(1), "key")
			wantErr := error(nil)
			switch name {
			case "нет сообщения":
				query.WillReturnError(sql.ErrNoRows)
				wantErr = model.ErrMessageNotFound
			case "ошибка БД":
				query.WillReturnError(failure)
				wantErr = failure
			default:
				query.WillReturnRows(mediaLookupRows(strings.Repeat("a", 64)))
			}
			got, err := r.GetMediaByClientID(context.Background(), 1, 10, "key")
			assertError(t, err, wantErr)
			if err == nil && (got.Message.ID != 105 || got.Message.Text != nil || *got.Message.StorageKey != "voice/key" || *got.Message.DurationSeconds != 2.5 || got.SHA256 != strings.Repeat("a", 64)) {
				t.Fatalf("media = %#v", got)
			}
		})
	}
}

func TestCreateMediaMessage(t *testing.T) {
	for _, name := range []string{"новое", "повтор", "конфликт", "ошибка begin", "нет доступа", "ошибка блокировки", "ошибка поиска", "запрещено", "ошибка доступа", "ошибка вставки", "неизвестный commit", "commit повтора"} {
		t.Run(name, func(t *testing.T) {
			r, mock := newRepository(t)
			digest := strings.Repeat("a", 64)
			command := model.ChatCreateMedia{ChatID: 10, ClientMessageID: "key", Type: model.ChatMessageTypeVoice, StorageKey: "voice/key", DurationSeconds: 2.5, SHA256: digest}
			wantErr := error(nil)
			begin := mock.ExpectBegin()
			if name == "ошибка begin" {
				begin.WillReturnError(failure)
				wantErr = failure
			} else {
				lock := mock.ExpectQuery(lockChatSequenceQuery).WithArgs(int64(1), int64(10))
				if name == "нет доступа" {
					lock.WillReturnError(sql.ErrNoRows)
					wantErr = model.ErrChatNotFound
				} else if name == "ошибка блокировки" {
					lock.WillReturnError(failure)
					wantErr = failure
				} else {
					lock.WillReturnRows(sqlmock.NewRows([]string{"last_seq"}).AddRow(4))
					lookup := mock.ExpectQuery(findClientMediaQuery).WithArgs(int64(10), int64(1), "key")
					switch name {
					case "повтор", "конфликт", "commit повтора":
						existingDigest := digest
						if name == "конфликт" {
							existingDigest = strings.Repeat("b", 64)
							wantErr = model.ErrIdempotencyConflict
						}
						lookup.WillReturnRows(mediaLookupRows(existingDigest))
					case "ошибка поиска":
						lookup.WillReturnError(failure)
						wantErr = failure
					default:
						lookup.WillReturnError(sql.ErrNoRows)
						access := mock.ExpectQuery(canSendMessageQuery).WithArgs(int64(1), int64(10))
						if name == "ошибка доступа" {
							access.WillReturnError(failure)
							wantErr = failure
						} else {
							access.WillReturnRows(sqlmock.NewRows([]string{"allowed"}).AddRow(name != "запрещено"))
							if name == "запрещено" {
								wantErr = model.ErrMessagingNotAllowed
							} else {
								insert := mock.ExpectQuery(createMediaMessageQuery).WithArgs(int64(10), int64(1), "key", "voice", "voice/key", 2.5, digest)
								if name == "ошибка вставки" {
									insert.WillReturnError(failure)
									wantErr = failure
								} else {
									insert.WillReturnRows(sqlmock.NewRows(messageColumns).AddRow(mediaValues()...))
									expectMessageVersionUpdates(mock)
								}
							}
						}
					}
				}
				if wantErr != nil {
					mock.ExpectRollback()
				} else {
					commit := mock.ExpectCommit()
					if name == "неизвестный commit" || name == "commit повтора" {
						commit.WillReturnError(failure)
						wantErr = model.ErrMediaCommitUnknown
					}
				}
			}
			message, created, err := r.CreateMediaMessage(context.Background(), 1, command)
			assertError(t, err, wantErr)
			if errors.Is(err, model.ErrMediaCommitUnknown) && !errors.Is(err, failure) {
				t.Fatalf("lost commit error: %v", err)
			}
			if err != nil {
				if message != nil || created {
					t.Fatalf("message = %v, created = %v", message, created)
				}
				return
			}
			if message.ID != 105 || message.Seq != 5 || *message.StorageKey != "voice/key" || message.Text != nil || created != (name == "новое") {
				t.Fatalf("message = %#v, created = %v", message, created)
			}
		})
	}
}
