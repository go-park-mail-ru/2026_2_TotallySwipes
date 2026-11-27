package service_test

import (
	"context"
	"reflect"
	"testing"

	"dating-app/services/chat/internal/model"
	"github.com/google/uuid"
	"go.uber.org/mock/gomock"
)

func expectMembers(f fixture, count int) {
	f.repo.EXPECT().GeyMembers(gomock.Any(), int64(10)).Return([]model.ChatMember{
		{ChatID: 10, UserID: 1},
		{ChatID: 10, UserID: 2},
	}, nil).Times(count)
}

func expectCard(f fixture, userID int64) {
	peerID := int64(3) - userID
	f.repo.EXPECT().GetChatState(gomock.Any(), userID, int64(10)).Return(&model.ChatState{
		Chat:                 model.Chat{ID: 10, MatchID: 20},
		Viewer:               model.ChatMember{UserID: userID},
		Peer:                 model.ChatMember{UserID: peerID},
		MatchExists:          true,
		MatchAllowsMessaging: true,
	}, nil)
	f.profile.EXPECT().GetChatProfiles(gomock.Any(), []int64{userID, peerID}).Return(map[int64]model.ChatProfile{}, nil)
}

func expectSummary(f fixture, userID int64) model.ChatSummary {
	summary := model.ChatSummary{NewChatsCount: userID, UnreadMessagesCount: userID * 2}
	f.repo.EXPECT().GetSummary(gomock.Any(), userID).Return(&summary, nil)
	return summary
}

func checkEvent(t *testing.T, event model.ChatEvent) {
	t.Helper()
	if _, err := uuid.Parse(event.EventID); err != nil {
		t.Fatalf("invalid event id %q: %v", event.EventID, err)
	}
}

func TestEnsureChatValidation(t *testing.T) {
	for _, tc := range []struct {
		name                       string
		matchID, firstID, secondID int64
		want                       error
	}{
		{"неверный мэтч", 0, 1, 2, model.ErrInvalidMatchID},
		{"первый пользователь", 20, 0, 2, model.ErrInvalidUserID},
		{"второй пользователь", 20, 1, -1, model.ErrInvalidUserID},
		{"одинаковые участники", 20, 1, 1, model.ErrInvalidChatMembers},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newFixture(t)
			id, created, err := f.svc.EnsureChat(context.Background(), tc.matchID, tc.firstID, tc.secondID)
			assertError(t, err, tc.want)
			if id != 0 || created {
				t.Fatalf("id = %d, created = %v", id, created)
			}
		})
	}
}

func TestEnsureChat(t *testing.T) {
	for _, tc := range []struct {
		name                string
		created             bool
		repoErr, publishErr error
	}{
		{name: "новый чат", created: true},
		{name: "существующий чат"},
		{name: "ошибка репозитория", repoErr: dependencyErr},
		{name: "ошибка публикации первому не мешает второму", created: true, publishErr: dependencyErr},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newFixture(t)
			var saveFinished bool
			f.repo.EXPECT().CreateChat(gomock.Any(), int64(20), int64(1), int64(2)).DoAndReturn(
				func(context.Context, int64, int64, int64) (int64, bool, error) {
					saveFinished = true
					return 10, tc.created, tc.repoErr
				},
			)
			if tc.created && tc.repoErr == nil {
				expectMembers(f, 2)
				for _, userID := range []int64{1, 2} {
					expectCard(f, userID)
					f.broker.EXPECT().Publish(gomock.Any(), userID, gomock.Any()).DoAndReturn(
						func(_ context.Context, recipient int64, event model.ChatEvent) error {
							if !saveFinished {
								t.Fatal("event published before saving")
							}
							checkEvent(t, event)
							payload, ok := event.Payload.(model.ChatCreated)
							if !ok || payload.Chat.ChatID != 10 || payload.Chat.Peer.UserID != 3-recipient {
								t.Fatalf("recipient = %d, payload = %#v", recipient, event.Payload)
							}
							if recipient == 1 {
								return tc.publishErr
							}
							return nil
						},
					)
				}
				for _, userID := range []int64{1, 2} {
					want := expectSummary(f, userID)
					f.broker.EXPECT().Publish(gomock.Any(), userID, gomock.Any()).DoAndReturn(
						func(_ context.Context, _ int64, event model.ChatEvent) error {
							checkEvent(t, event)
							payload, ok := event.Payload.(model.ChatsSummaryUpdated)
							if !ok || payload.Summary != want {
								t.Fatalf("payload = %#v, want %#v", event.Payload, want)
							}
							return nil
						},
					)
				}
			}
			id, created, err := f.svc.EnsureChat(context.Background(), 20, 1, 2)
			assertError(t, err, tc.repoErr)
			if err != nil {
				if id != 0 || created {
					t.Fatalf("id = %d, created = %v", id, created)
				}
				return
			}
			if id != 10 || created != tc.created {
				t.Fatalf("id = %d, created = %v", id, created)
			}
		})
	}
}

func TestEnsureChatPublicationErrors(t *testing.T) {
	for _, stage := range []string{"участники", "карточка", "профиль", "summary", "брокер summary"} {
		t.Run(stage, func(t *testing.T) {
			f := newFixture(t)
			f.repo.EXPECT().CreateChat(gomock.Any(), int64(20), int64(1), int64(2)).Return(int64(10), true, nil)
			if stage == "участники" {
				f.repo.EXPECT().GeyMembers(gomock.Any(), int64(10)).Return(nil, dependencyErr).Times(2)
			} else {
				expectMembers(f, 2)
				for _, userID := range []int64{1, 2} {
					if userID == 1 && stage == "карточка" {
						f.repo.EXPECT().GetChatState(gomock.Any(), userID, int64(10)).Return(nil, dependencyErr)
					} else if userID == 1 && stage == "профиль" {
						f.repo.EXPECT().GetChatState(gomock.Any(), userID, int64(10)).Return(&model.ChatState{Viewer: model.ChatMember{UserID: 1}, Peer: model.ChatMember{UserID: 2}}, nil)
						f.profile.EXPECT().GetChatProfiles(gomock.Any(), []int64{1, 2}).Return(nil, dependencyErr)
					} else {
						expectCard(f, userID)
						f.broker.EXPECT().Publish(gomock.Any(), userID, gomock.Any()).Return(nil)
					}
				}
				for _, userID := range []int64{1, 2} {
					if userID == 1 && stage == "summary" {
						f.repo.EXPECT().GetSummary(gomock.Any(), userID).Return(nil, dependencyErr)
					} else {
						expectSummary(f, userID)
						publishErr := error(nil)
						if userID == 1 && stage == "брокер summary" {
							publishErr = dependencyErr
						}
						f.broker.EXPECT().Publish(gomock.Any(), userID, gomock.Any()).Return(publishErr)
					}
				}
			}
			id, created, err := f.svc.EnsureChat(context.Background(), 20, 1, 2)
			assertError(t, err, nil)
			if id != 10 || !created {
				t.Fatalf("id = %d, created = %v", id, created)
			}
		})
	}
}

func TestSubscribeEvents(t *testing.T) {
	for _, tc := range []struct {
		name               string
		userID             int64
		brokerErr, wantErr error
	}{
		{"успех", 1, nil, nil},
		{"неверный пользователь", 0, nil, model.ErrInvalidUserID},
		{"ошибка брокера", 1, dependencyErr, dependencyErr},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newFixture(t)
			stream := NewMockChatEventsStream(gomock.NewController(t))
			if tc.userID > 0 {
				f.broker.EXPECT().Subscribe(gomock.Any(), tc.userID).Return(stream, tc.brokerErr)
			}
			got, err := f.svc.SubscribeEvents(context.Background(), tc.userID)
			assertError(t, err, tc.wantErr)
			if err == nil && got != stream {
				t.Fatal("unexpected subscription")
			}
		})
	}
}

func TestSendMessageEvents(t *testing.T) {
	f := newFixture(t)
	command := model.ChatSendMessage{ChatID: 10, ClientMessageID: "key", Text: "Привет"}
	stored := &model.StoredMessage{ID: 100, ChatID: 10, ClientMessageID: &command.ClientMessageID, Seq: 5, Type: model.ChatMessageTypeText, Text: &command.Text}
	f.repo.EXPECT().CreateMessage(gomock.Any(), int64(1), command).Return(stored, true, nil)
	expectMembers(f, 3)
	for _, userID := range []int64{1, 2} {
		f.broker.EXPECT().Publish(gomock.Any(), userID, gomock.Any()).DoAndReturn(func(_ context.Context, recipient int64, event model.ChatEvent) error {
			checkEvent(t, event)
			payload, ok := event.Payload.(model.MessageCreated)
			if !ok || payload.ChatID != 10 || payload.ClientMessageID == nil || *payload.ClientMessageID != "key" || payload.Message.MessageID != 100 {
				t.Fatalf("payload = %#v", event.Payload)
			}
			if recipient == 1 {
				return dependencyErr
			}
			return nil
		})
	}
	for _, userID := range []int64{1, 2} {
		expectCard(f, userID)
		f.broker.EXPECT().Publish(gomock.Any(), userID, gomock.Any()).DoAndReturn(func(_ context.Context, recipient int64, event model.ChatEvent) error {
			payload, ok := event.Payload.(model.ChatUpdated)
			if !ok || payload.Chat.ChatID != 10 || payload.Chat.Peer.UserID != 3-recipient {
				t.Fatalf("payload = %#v", event.Payload)
			}
			return nil
		})
	}
	for _, userID := range []int64{1, 2} {
		want := expectSummary(f, userID)
		f.broker.EXPECT().Publish(gomock.Any(), userID, gomock.Any()).DoAndReturn(func(_ context.Context, _ int64, event model.ChatEvent) error {
			payload, ok := event.Payload.(model.ChatsSummaryUpdated)
			if !ok || payload.Summary != want {
				t.Fatalf("payload = %#v", event.Payload)
			}
			return nil
		})
	}
	got, err := f.svc.SendMessage(context.Background(), 1, command)
	assertError(t, err, nil)
	if got.MessageID != 100 {
		t.Fatalf("message = %#v", got)
	}
}

func TestReadChatEvents(t *testing.T) {
	f := newFixture(t)
	command := model.ChatRead{ChatID: 10, LastReadSeq: 2}
	updated := &model.ChatReadUpdated{ChatID: 10, UserID: 1, LastReadSeq: 7}
	f.repo.EXPECT().ReadChat(gomock.Any(), int64(1), command).Return(updated, nil)
	expectMembers(f, 2)
	for _, userID := range []int64{1, 2} {
		f.broker.EXPECT().Publish(gomock.Any(), userID, gomock.Any()).DoAndReturn(func(_ context.Context, recipient int64, event model.ChatEvent) error {
			if !reflect.DeepEqual(event.Payload, *updated) {
				t.Fatalf("payload = %#v, want %#v", event.Payload, updated)
			}
			if recipient == 1 {
				return dependencyErr
			}
			return nil
		})
	}
	for _, userID := range []int64{1, 2} {
		expectCard(f, userID)
		f.broker.EXPECT().Publish(gomock.Any(), userID, gomock.Any()).Return(nil)
	}
	expectSummary(f, 1)
	f.broker.EXPECT().Publish(gomock.Any(), int64(1), gomock.Any()).Return(nil)
	got, err := f.svc.ReadChat(context.Background(), 1, command)
	assertError(t, err, nil)
	if !reflect.DeepEqual(got, updated) {
		t.Fatalf("read = %#v", got)
	}
}
