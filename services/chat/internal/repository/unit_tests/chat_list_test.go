package repository_test

import (
	"context"
	"errors"
	"testing"

	"dating-app/services/chat/internal/model"
	"dating-app/services/chat/internal/repository"
	"github.com/DATA-DOG/go-sqlmock"
)

func TestGetChats(t *testing.T) {

	columns := []string{
		"chat_id", "match_id", "peer_id", "is_new", "unread_count", "created_at",
		"last_activity_at", "version", "message_id", "seq", "sender_id", "type",
		"text_preview", "duration_seconds", "message_created_at", "is_read_by_peer",
	}

	for _, name := range []string{"empty", "last page", "next page", "query error", "rows error", "scan error"} {
		t.Run(name, func(t *testing.T) {

			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()

			repo := repository.NewChatRepository(db)
			rows := sqlmock.NewRows(columns)
			query := model.ChatListQuery{Limit: ptr(1), Cursor: ptr("5")}
			expectation := mock.ExpectQuery(`SELECT[\s\S]*FROM chat_member AS viewer`).
				WithArgs(int64(1), 2, int64(5))

			switch name {
			case "query error":
				expectation.WillReturnError(failure)
			case "scan error":
				rows.AddRow("bad id", 20, 2, true, 0, now, now, 1, nil, nil, nil, nil, nil, nil, nil, nil)
				expectation.WillReturnRows(rows).RowsWillBeClosed()
			default:
				if name != "empty" {
					// Пустой чат должен попасть в результат без фиктивного последнего сообщения.
					rows.AddRow(10, 20, 2, true, 0, now, now, 1, nil, nil, nil, nil, nil, nil, nil, nil)
				}
				if name == "next page" {
					rows.AddRow(9, 19, 3, false, 2, now, now, 4, 100, 7, 3, "text", "Привет", nil, now, nil)
				}
				if name == "rows error" {
					rows.RowError(0, failure)
				}
				expectation.WillReturnRows(rows).RowsWillBeClosed()
			}

			page, err := repo.GetChats(context.Background(), 1, query)

			switch name {
			case "query error", "rows error":
				if !errors.Is(err, failure) || page != nil {
					t.Fatalf("page=%v error=%v", page, err)
				}
			case "scan error":
				if err == nil || page != nil {
					t.Fatalf("page=%v error=%v", page, err)
				}
			default:
				if err != nil || page == nil || page.Items == nil {
					t.Fatalf("page=%v error=%v", page, err)
				}
				if name == "empty" {
					if len(page.Items) != 0 || page.NextCursor != nil {
						t.Fatalf("unexpected empty page: %+v", page)
					}
				} else {
					if len(page.Items) != 1 || page.Items[0].ChatID != 10 || page.Items[0].Peer.UserID != 2 || page.Items[0].LastMessage != nil {
						t.Fatalf("unexpected cards: %+v", page.Items)
					}
					if name == "next page" {
						if page.NextCursor == nil || *page.NextCursor != "6" {
							t.Fatalf("unexpected next cursor: %v", page.NextCursor)
						}
					} else if page.NextCursor != nil {
						t.Fatal("last page has a cursor")
					}
				}
			}

			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestGetChatsLastMessage(t *testing.T) {

	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	rows := sqlmock.NewRows([]string{"chat", "match", "peer", "new", "unread", "created", "activity", "version", "message", "seq", "sender", "type", "text", "duration", "sent", "read"}).
		AddRow(10, 20, 2, false, 3, now, now, 7, 100, 8, 1, "text", "Привет", nil, now, true)

	mock.ExpectQuery(`SELECT[\s\S]*FROM chat_member AS viewer`).WithArgs(int64(1), 101, int64(0)).WillReturnRows(rows).RowsWillBeClosed()

	page, err := repository.NewChatRepository(db).GetChats(context.Background(), 1, model.ChatListQuery{})
	if err != nil {
		t.Fatal(err)
	}

	last := page.Items[0].LastMessage
	if last == nil || last.MessageID != 100 || last.Seq != 8 || last.TextPreview == nil || *last.TextPreview != "Привет" || last.IsReadByPeer == nil || !*last.IsReadByPeer {
		t.Fatalf("unexpected last message: %+v", last)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
