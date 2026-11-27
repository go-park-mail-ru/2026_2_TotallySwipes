package service_test

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"dating-app/services/chat/internal/model"
	"dating-app/services/chat/internal/service"
	"go.uber.org/mock/gomock"
)

var dependencyErr = errors.New("dependency failure")
var now = time.Date(2026, 11, 26, 12, 0, 0, 0, time.UTC)

func ptr[T any](value T) *T { return &value }

type fixture struct {
	svc       *service.ChatService
	repo      *MockChatRepository
	profile   *MockProfileProvider
	broker    *MockChatEventBroker
	storage   *MockMediaStorage
	links     *MockMediaLinkBuilder
	inspector *MockMediaInspector
}

func newFixture(t *testing.T) fixture {
	t.Helper()
	ctrl := gomock.NewController(t)
	f := fixture{
		repo:      NewMockChatRepository(ctrl),
		profile:   NewMockProfileProvider(ctrl),
		broker:    NewMockChatEventBroker(ctrl),
		storage:   NewMockMediaStorage(ctrl),
		links:     NewMockMediaLinkBuilder(ctrl),
		inspector: NewMockMediaInspector(ctrl),
	}
	f.svc = service.NewChatService(f.repo, f.profile, f.broker, f.storage, f.links, f.inspector)
	return f
}

func assertError(t *testing.T, got, want error) {
	t.Helper()
	if !errors.Is(got, want) {
		t.Fatalf("error = %v, want %v", got, want)
	}
}

func TestGetSummary(t *testing.T) {
	for _, tc := range []struct {
		name             string
		userID           int64
		repoErr, wantErr error
	}{
		{"успех", 1, nil, nil},
		{"неверный пользователь", 0, nil, model.ErrInvalidUserID},
		{"ошибка репозитория", 1, dependencyErr, dependencyErr},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newFixture(t)
			want := &model.ChatSummary{NewChatsCount: 2, UnreadChatsCount: 3, UnreadMessagesCount: 7}
			if tc.userID > 0 {
				f.repo.EXPECT().GetSummary(gomock.Any(), tc.userID).Return(want, tc.repoErr)
			}
			got, err := f.svc.GetSummary(context.Background(), tc.userID)
			assertError(t, err, tc.wantErr)
			if err == nil && !reflect.DeepEqual(got, want) {
				t.Fatalf("summary = %#v, want %#v", got, want)
			}
		})
	}
}

func TestSendMessageValidation(t *testing.T) {
	for _, tc := range []struct {
		name    string
		userID  int64
		command model.ChatSendMessage
		want    error
	}{
		{"пользователь", 0, model.ChatSendMessage{}, model.ErrInvalidUserID},
		{"чат", 1, model.ChatSendMessage{}, model.ErrInvalidChatID},
		{"пустой ключ", 1, model.ChatSendMessage{ChatID: 10, ClientMessageID: " \n\t", Text: "текст"}, model.ErrInvalidClientMessageID},
		{"длинный ключ", 1, model.ChatSendMessage{ChatID: 10, ClientMessageID: strings.Repeat("я", 129), Text: "текст"}, model.ErrInvalidClientMessageID},
		{"кодировка ключа", 1, model.ChatSendMessage{ChatID: 10, ClientMessageID: string([]byte{255}), Text: "текст"}, model.ErrInvalidClientMessageID},
		{"пустой текст", 1, model.ChatSendMessage{ChatID: 10, ClientMessageID: "key", Text: " \n"}, model.ErrInvalidMessageText},
		{"длинный текст", 1, model.ChatSendMessage{ChatID: 10, ClientMessageID: "key", Text: strings.Repeat("я", 10001)}, model.ErrInvalidMessageText},
		{"кодировка текста", 1, model.ChatSendMessage{ChatID: 10, ClientMessageID: "key", Text: string([]byte{255})}, model.ErrInvalidMessageText},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newFixture(t)
			got, err := f.svc.SendMessage(context.Background(), tc.userID, tc.command)
			assertError(t, err, tc.want)
			if got != nil {
				t.Fatalf("message = %#v, want nil", got)
			}
		})
	}
}

func TestSendMessage(t *testing.T) {
	for _, tc := range []struct {
		name    string
		created bool
		text    string
		repoErr error
	}{
		{"новое сообщение с пробелами", true, "  Привет!\n", nil},
		{"повтор команды", false, "Привет", nil},
		{"граница длины в символах", true, strings.Repeat("я", 10000), nil},
		{"ошибка репозитория", false, "Привет", dependencyErr},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newFixture(t)
			command := model.ChatSendMessage{ChatID: 10, ClientMessageID: strings.Repeat("я", 128), Text: tc.text}
			stored := &model.StoredMessage{ID: 100, ChatID: 10, Seq: 5, SenderID: ptr(int64(1)), Type: model.ChatMessageTypeText, Text: &command.Text, CreatedAt: now, Version: 1}
			f.repo.EXPECT().CreateMessage(gomock.Any(), int64(1), command).Return(stored, tc.created, tc.repoErr)
			if tc.created && tc.repoErr == nil {
				f.repo.EXPECT().GeyMembers(gomock.Any(), int64(10)).Return([]model.ChatMember{}, nil).Times(3)
			}
			got, err := f.svc.SendMessage(context.Background(), 1, command)
			assertError(t, err, tc.repoErr)
			if err != nil {
				return
			}
			want := &model.ChatMessage{MessageID: 100, Seq: 5, SenderID: ptr(int64(1)), Type: model.ChatMessageTypeText, Text: &command.Text, CreatedAt: now, Version: 1}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("message = %#v, want %#v", got, want)
			}
		})
	}
}

func TestGetChat(t *testing.T) {
	for _, tc := range []struct {
		name                         string
		change                       func(*model.ChatState)
		restriction                  string
		repoErr, profileErr, wantErr error
	}{
		{name: "доступный чат"},
		{name: "собеседник отключён", change: func(s *model.ChatState) { s.PeerDisabled = true }, restriction: model.MessagingRestrictionPeerDisabled},
		{name: "блокировка собеседника", change: func(s *model.ChatState) { s.ViewerBlockedPeer = true }, restriction: model.MessagingRestrictionPairBlocked},
		{name: "блокировка пользователем", change: func(s *model.ChatState) { s.PeerBlockedViewer = true }, restriction: model.MessagingRestrictionPairBlocked},
		{name: "нет мэтча", change: func(s *model.ChatState) { s.MatchExists = false }, restriction: model.MessagingRestrictionMatchNotFound},
		{name: "мэтч не разрешает переписку", change: func(s *model.ChatState) { s.MatchAllowsMessaging = false }, restriction: model.MessagingRestrictionNotAllowed},
		{name: "пользователь отключён", change: func(s *model.ChatState) { s.ViewerDisabled = true }, wantErr: model.ErrChatMemberBlocked},
		{name: "ошибка репозитория", repoErr: dependencyErr, wantErr: dependencyErr},
		{name: "ошибка профиля", profileErr: dependencyErr, wantErr: dependencyErr},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newFixture(t)
			state := &model.ChatState{Chat: model.Chat{ID: 10, MatchID: 20, CreatedAt: now}, Viewer: model.ChatMember{UserID: 1, LastReadSeq: 3}, Peer: model.ChatMember{UserID: 2, LastReadSeq: 4}, MatchExists: true, MatchAllowsMessaging: true, LastActivityAt: now.Add(time.Hour), UnreadCount: 2, LastSeq: 5, CardVersion: 7}
			if tc.change != nil {
				tc.change(state)
			}
			f.repo.EXPECT().GetChatState(gomock.Any(), int64(1), int64(10)).Return(state, tc.repoErr)
			if tc.repoErr == nil && !state.ViewerDisabled {
				profiles := map[int64]model.ChatProfile{1: {Tags: []string{"go", "кино", "go"}}, 2: {Name: ptr("Анна"), Age: ptr(25), AvatarURL: ptr("/photo"), Tags: []string{"go", "спорт"}}}
				f.profile.EXPECT().GetChatProfiles(gomock.Any(), []int64{1, 2}).Return(profiles, tc.profileErr)
			}
			got, err := f.svc.GetChat(context.Background(), 1, 10)
			assertError(t, err, tc.wantErr)
			if err != nil {
				return
			}
			if got.Chat.ChatID != 10 || got.Chat.MatchID != 20 || !got.Chat.IsNew || got.Chat.UnreadCount != 2 || got.Chat.Version != 7 || got.IsEmpty || got.LastReadSeq != 3 || got.PeerLastReadSeq != 4 {
				t.Fatalf("details = %#v", got)
			}

			if !got.Chat.LastActivityAt.Equal(state.LastActivityAt) {
				t.Fatalf("last activity = %v, want %v", got.Chat.LastActivityAt, state.LastActivityAt)
			}
			if !reflect.DeepEqual(got.CommonTags, []string{"go"}) || got.Chat.Peer.UserID != 2 || *got.Chat.Peer.Name != "Анна" {
				t.Fatalf("profile mapping = %#v", got)
			}
			if tc.restriction == "" {
				if !got.Messaging.CanSend || got.Messaging.Restriction != nil {
					t.Fatalf("messaging = %#v", got.Messaging)
				}
			} else if got.Messaging.CanSend || got.Messaging.Restriction == nil || *got.Messaging.Restriction != tc.restriction {
				t.Fatalf("messaging = %#v", got.Messaging)
			}
		})
	}
}

func TestGetMessagesValidation(t *testing.T) {
	for _, tc := range []struct {
		name           string
		userID, chatID int64
		query          model.ChatMessagesQuery
		want           error
	}{
		{"пользователь", 0, 10, model.ChatMessagesQuery{}, model.ErrInvalidUserID},
		{"чат", 1, 0, model.ChatMessagesQuery{}, model.ErrInvalidChatID},
		{"два направления", 1, 10, model.ChatMessagesQuery{BeforeSeq: ptr(int64(5)), AfterSeq: ptr(int64(1))}, model.ErrInvalidMessagesQuery},
		{"неверный before", 1, 10, model.ChatMessagesQuery{BeforeSeq: ptr(int64(0))}, model.ErrInvalidMessagesQuery},
		{"неверный after", 1, 10, model.ChatMessagesQuery{AfterSeq: ptr(int64(-1))}, model.ErrInvalidMessagesQuery},
		{"неверный limit", 1, 10, model.ChatMessagesQuery{Limit: ptr(0)}, model.ErrInvalidMessagesQuery},
		{"limit больше максимума", 1, 10, model.ChatMessagesQuery{Limit: ptr(101)}, model.ErrInvalidMessagesQuery},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newFixture(t)
			_, err := f.svc.GetMessages(context.Background(), tc.userID, tc.chatID, tc.query)
			assertError(t, err, tc.want)
		})
	}
}

func TestGetMessagesLimitBoundaries(t *testing.T) {

	for _, limit := range []int{1, 100} {

		f := newFixture(t)
		query := model.ChatMessagesQuery{Limit: ptr(limit)}
		f.repo.EXPECT().GetChatState(gomock.Any(), int64(1), int64(10)).Return(&model.ChatState{}, nil)
		f.repo.EXPECT().GetMessages(gomock.Any(), int64(1), int64(10), query).
			Return(&model.StoredMessagesPage{Direction: model.ChatPageDirectionOlder}, nil)

		if _, err := f.svc.GetMessages(context.Background(), 1, 10, query); err != nil {
			t.Fatalf("limit = %d, error = %v", limit, err)
		}
	}
}

func TestGetMessages(t *testing.T) {
	for _, tc := range []struct {
		name                       string
		blocked                    bool
		stateErr, pageErr, wantErr error
	}{
		{name: "маппинг страницы"},
		{name: "нет доступа", stateErr: model.ErrChatNotFound, wantErr: model.ErrChatNotFound},
		{name: "пользователь отключён", blocked: true, wantErr: model.ErrChatMemberBlocked},
		{name: "ошибка чтения страницы", pageErr: dependencyErr, wantErr: dependencyErr},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newFixture(t)
			f.repo.EXPECT().GetChatState(gomock.Any(), int64(1), int64(10)).Return(&model.ChatState{ViewerDisabled: tc.blocked}, tc.stateErr)
			if tc.stateErr == nil && !tc.blocked {
				page := &model.StoredMessagesPage{Direction: model.ChatPageDirectionOlder, HasMore: true, NextCursor: ptr(int64(3)), Items: []model.StoredMessage{
					{ID: 3, ChatID: 10, Seq: 3, Type: model.ChatMessageTypeText, Text: ptr("текст")},
					{ID: 4, ChatID: 10, Seq: 4, Type: model.ChatMessageTypeVoice, StorageKey: ptr("key"), DurationSeconds: ptr(2.5)},
					{ID: 5, ChatID: 10, Seq: 5, Type: model.ChatMessageTypeVoice, DeletedAt: &now, Text: ptr("скрыто"), StorageKey: ptr("deleted"), DurationSeconds: ptr(1.0)},
				}}
				if tc.pageErr != nil {
					page = nil
				}
				f.repo.EXPECT().GetMessages(gomock.Any(), int64(1), int64(10), model.ChatMessagesQuery{Limit: ptr(50)}).Return(page, tc.pageErr)
				if tc.pageErr == nil {
					f.links.EXPECT().BuildMessageMediaURL(int64(10), int64(4)).Return("/media/4")
				}
			}
			got, err := f.svc.GetMessages(context.Background(), 1, 10, model.ChatMessagesQuery{})
			assertError(t, err, tc.wantErr)
			if err != nil {
				return
			}
			if len(got.Items) != 3 || !got.HasMore || *got.NextCursor != 3 || got.Direction != model.ChatPageDirectionOlder {
				t.Fatalf("page = %#v", got)
			}
			if *got.Items[0].Text != "текст" || got.Items[0].Attachment != nil || got.Items[1].Attachment.URL != "/media/4" || got.Items[1].Attachment.DurationSeconds != 2.5 || got.Items[2].Text != nil || got.Items[2].Attachment != nil {
				t.Fatalf("items = %#v", got.Items)
			}
		})
	}
}

func TestMarkSeen(t *testing.T) {
	for _, tc := range []struct {
		name             string
		userID, chatID   int64
		repoErr, wantErr error
	}{
		{"успех", 1, 10, nil, nil}, {"пользователь", 0, 10, nil, model.ErrInvalidUserID}, {"чат", 1, 0, nil, model.ErrInvalidChatID}, {"ошибка репозитория", 1, 10, dependencyErr, dependencyErr},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newFixture(t)
			if tc.userID > 0 && tc.chatID > 0 {
				f.repo.EXPECT().MarkSeen(gomock.Any(), tc.userID, tc.chatID).Return(tc.repoErr)
				if tc.repoErr == nil {
					f.repo.EXPECT().GeyMembers(gomock.Any(), tc.chatID).Return([]model.ChatMember{}, nil)
					f.repo.EXPECT().GetSummary(gomock.Any(), tc.userID).Return(&model.ChatSummary{}, nil)
					f.broker.EXPECT().Publish(gomock.Any(), tc.userID, gomock.Any()).Return(nil)
				}
			}
			assertError(t, f.svc.MarkSeen(context.Background(), tc.userID, tc.chatID), tc.wantErr)
		})
	}
}

func TestReadChat(t *testing.T) {
	for _, tc := range []struct {
		name             string
		userID           int64
		command          model.ChatRead
		repoErr, wantErr error
	}{
		{"успех", 1, model.ChatRead{ChatID: 10, LastReadSeq: 5}, nil, nil},
		{"нулевая позиция", 1, model.ChatRead{ChatID: 10}, nil, nil},
		{"пользователь", 0, model.ChatRead{ChatID: 10}, nil, model.ErrInvalidUserID},
		{"чат", 1, model.ChatRead{}, nil, model.ErrInvalidChatID},
		{"отрицательная позиция", 1, model.ChatRead{ChatID: 10, LastReadSeq: -1}, nil, model.ErrInvalidReadSeq},
		{"ошибка репозитория", 1, model.ChatRead{ChatID: 10}, dependencyErr, dependencyErr},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newFixture(t)
			want := &model.ChatReadUpdated{ChatID: 10, UserID: 1, LastReadSeq: 7}
			if tc.userID > 0 && tc.command.ChatID > 0 && tc.command.LastReadSeq >= 0 {
				f.repo.EXPECT().ReadChat(gomock.Any(), tc.userID, tc.command).Return(want, tc.repoErr)
				if tc.repoErr == nil {
					f.repo.EXPECT().GeyMembers(gomock.Any(), tc.command.ChatID).Return([]model.ChatMember{}, nil).Times(2)
					f.repo.EXPECT().GetSummary(gomock.Any(), tc.userID).Return(&model.ChatSummary{}, nil)
					f.broker.EXPECT().Publish(gomock.Any(), tc.userID, gomock.Any()).Return(nil)
				}
			}
			got, err := f.svc.ReadChat(context.Background(), tc.userID, tc.command)
			assertError(t, err, tc.wantErr)
			if err == nil && !reflect.DeepEqual(got, want) {
				t.Fatalf("read = %#v, want %#v", got, want)
			}
		})
	}
}

func TestGetMedia(t *testing.T) {
	for _, tc := range []struct {
		name                                      string
		userID, chatID, messageID                 int64
		blocked                                   bool
		message                                   *model.StoredMessage
		stateErr, messageErr, storageErr, wantErr error
	}{
		{name: "успех", userID: 1, chatID: 10, messageID: 100, message: &model.StoredMessage{StorageKey: ptr("voice/key")}},
		{name: "пользователь", chatID: 10, messageID: 100, wantErr: model.ErrInvalidUserID},
		{name: "чат", userID: 1, messageID: 100, wantErr: model.ErrInvalidChatID},
		{name: "сообщение", userID: 1, chatID: 10, wantErr: model.ErrInvalidMessageID},
		{name: "чужой чат", userID: 1, chatID: 10, messageID: 100, stateErr: model.ErrChatNotFound, wantErr: model.ErrChatNotFound},
		{name: "отключён", userID: 1, chatID: 10, messageID: 100, blocked: true, wantErr: model.ErrChatMemberBlocked},
		{name: "нет сообщения", userID: 1, chatID: 10, messageID: 100, messageErr: model.ErrMessageNotFound, wantErr: model.ErrMessageNotFound},
		{name: "удалено", userID: 1, chatID: 10, messageID: 100, message: &model.StoredMessage{StorageKey: ptr("key"), DeletedAt: &now}, wantErr: model.ErrMediaNotFound},
		{name: "нет вложения", userID: 1, chatID: 10, messageID: 100, message: &model.StoredMessage{}, wantErr: model.ErrMediaNotFound},
		{name: "пустой ключ", userID: 1, chatID: 10, messageID: 100, message: &model.StoredMessage{StorageKey: ptr("")}, wantErr: model.ErrMediaNotFound},
		{name: "ошибка хранилища", userID: 1, chatID: 10, messageID: 100, message: &model.StoredMessage{StorageKey: ptr("key")}, storageErr: dependencyErr, wantErr: dependencyErr},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newFixture(t)
			if tc.userID > 0 && tc.chatID > 0 && tc.messageID > 0 {
				f.repo.EXPECT().GetChatState(gomock.Any(), tc.userID, tc.chatID).Return(&model.ChatState{ViewerDisabled: tc.blocked}, tc.stateErr)
				if tc.stateErr == nil && !tc.blocked {
					f.repo.EXPECT().GetMessage(gomock.Any(), tc.chatID, tc.messageID).Return(tc.message, tc.messageErr)
					if tc.messageErr == nil && tc.message.DeletedAt == nil && tc.message.StorageKey != nil && *tc.message.StorageKey != "" {
						f.storage.EXPECT().GetPresignedDownloadURL(gomock.Any(), *tc.message.StorageKey).Return("https://storage/media", tc.storageErr)
					}
				}
			}
			got, err := f.svc.GetMedia(context.Background(), tc.userID, tc.chatID, tc.messageID)
			assertError(t, err, tc.wantErr)
			if err == nil && got != "https://storage/media" {
				t.Fatalf("url = %q", got)
			}
		})
	}
}
