package grpc_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"reflect"
	"testing"
	"time"

	chatpb "dating-app/services/chat/api/gen/go/chat/v1"
	delivery "dating-app/services/chat/internal/delivery/grpc"
	"dating-app/services/chat/internal/model"
	"go.uber.org/mock/gomock"
	grpcapi "google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
)

//go:generate go run go.uber.org/mock/mockgen -source=../chat_handler.go -destination=chat_mocks_test.go -package=grpc_test

func ptr[T any](value T) *T { return &value }

func TestErrorMapping(t *testing.T) {
	for _, tc := range []struct {
		err  error
		code codes.Code
	}{
		{model.ErrInvalidMedia, codes.InvalidArgument},
		{model.ErrMediaTooLong, codes.InvalidArgument},
		{model.ErrMediaTooLarge, codes.ResourceExhausted},
		{model.ErrUnsupportedMedia, codes.InvalidArgument},
		{model.ErrInvalidUserID, codes.InvalidArgument},
		{model.ErrInvalidMatchID, codes.InvalidArgument},
		{model.ErrInvalidChatID, codes.InvalidArgument},
		{model.ErrInvalidMessageID, codes.InvalidArgument},
		{model.ErrInvalidChatMembers, codes.InvalidArgument},
		{model.ErrInvalidMessagesQuery, codes.InvalidArgument},
		{model.ErrInvalidClientMessageID, codes.InvalidArgument},
		{model.ErrInvalidMessageText, codes.InvalidArgument},
		{model.ErrInvalidReadSeq, codes.InvalidArgument},
		{model.ErrChatNotFound, codes.NotFound},
		{model.ErrMessageNotFound, codes.NotFound},
		{model.ErrMediaNotFound, codes.NotFound},
		{model.ErrChatMemberBlocked, codes.PermissionDenied},
		{model.ErrMessagingNotAllowed, codes.PermissionDenied},
		{model.ErrIdempotencyConflict, codes.AlreadyExists},
		{context.Canceled, codes.Canceled},
		{context.DeadlineExceeded, codes.DeadlineExceeded},
		{errors.New("database password secret"), codes.Internal},
	} {
		t.Run(tc.err.Error(), func(t *testing.T) {
			svc := NewMockChatService(gomock.NewController(t))
			svc.EXPECT().GetSummary(gomock.Any(), int64(1)).Return(nil, fmt.Errorf("private details: %w", tc.err))
			response, err := delivery.NewChatHandler(svc).GetSummary(context.Background(), &chatpb.GetSummaryRequest{UserId: 1})
			if response != nil || status.Code(err) != tc.code {
				t.Fatalf("response = %v, error = %v", response, err)
			}
			if tc.code == codes.Internal && status.Convert(err).Message() != "internal chat error" {
				t.Fatalf("internal details leaked: %v", err)
			}
			if tc.code != codes.Internal && tc.code != codes.Canceled && tc.code != codes.DeadlineExceeded && status.Convert(err).Message() != tc.err.Error() {
				t.Fatalf("unexpected error message: %v", err)
			}
		})
	}
}

func TestUnaryMappings(t *testing.T) {
	svc := NewMockChatService(gomock.NewController(t))
	h := delivery.NewChatHandler(svc)
	ctx := context.Background()
	now := time.Date(2026, 11, 27, 12, 0, 0, 0, time.UTC)

	svc.EXPECT().EnsureChat(ctx, int64(20), int64(1), int64(2)).Return(int64(10), true, nil)
	ensured, err := h.EnsureChat(ctx, &chatpb.EnsureChatRequest{MatchId: 20, FirstUserId: 1, SecondUserId: 2})
	if err != nil || ensured.ChatId != "10" || !ensured.Created {
		t.Fatalf("ensure = %v, error = %v", ensured, err)
	}

	svc.EXPECT().GetSummary(ctx, int64(1)).Return(&model.ChatSummary{NewChatsCount: 2, UnreadChatsCount: 3, UnreadMessagesCount: 7, Version: 4}, nil)
	summary, err := h.GetSummary(ctx, &chatpb.GetSummaryRequest{UserId: 1})
	if err != nil || !proto.Equal(summary.Summary, &chatpb.ChatSummary{NewChatsCount: 2, UnreadChatsCount: 3, UnreadMessagesCount: 7, Version: 4}) {
		t.Fatalf("summary = %v, error = %v", summary, err)
	}

	card := model.ChatCard{ChatID: 10, MatchID: 20, Peer: model.ChatPeer{UserID: 2, Name: ptr("Анна"), Age: ptr(25), AvatarURL: ptr("/photo")}, IsNew: true, UnreadCount: 3, CreatedAt: now, LastActivityAt: now, Version: 5,
		LastMessage: &model.ChatLastMessage{MessageID: 100, Seq: 7, Type: model.ChatMessageTypeVoice, DurationSeconds: ptr(2.5), CreatedAt: now, IsReadByPeer: ptr(false)},
	}
	svc.EXPECT().GetChat(ctx, int64(1), int64(10)).Return(&model.ChatDetails{Chat: card, CommonTags: []string{"кино"}, LastReadSeq: 3, PeerLastReadSeq: 4, Messaging: model.ChatMessaging{Restriction: ptr("PAIR_BLOCKED")}}, nil)
	details, err := h.GetChat(ctx, &chatpb.GetChatRequest{UserId: 1, ChatId: "10"})
	if err != nil {
		t.Fatal(err)
	}
	got := details.Details
	if got.Chat.ChatId != "10" || got.Chat.MatchId != 20 || got.Chat.Peer.GetAge() != 25 || got.Chat.Peer.GetName() != "Анна" || got.Chat.Peer.GetAvatarUrl() != "/photo" || got.Chat.LastMessage.MessageId != "100" || got.Chat.LastMessage.IsReadByPeer == nil || got.Chat.LastMessage.GetIsReadByPeer() || !got.Chat.LastActivityAt.AsTime().Equal(now) || got.LastReadSeq != 3 || got.PeerLastReadSeq != 4 || got.Messaging.GetRestriction() != "PAIR_BLOCKED" || !reflect.DeepEqual(got.CommonTags, []string{"кино"}) {
		t.Fatalf("details = %v", got)
	}

	message := &model.ChatMessage{MessageID: 100, Seq: 7, SenderID: ptr(int64(1)), Type: model.ChatMessageTypeText, Text: ptr("Привет"), CreatedAt: now, EditedAt: &now, Version: 2}
	command := model.ChatSendMessage{ChatID: 10, ClientMessageID: "key", Text: "Привет"}
	svc.EXPECT().SendMessage(ctx, int64(1), command).Return(message, nil)
	sent, err := h.SendMessage(ctx, &chatpb.SendMessageRequest{UserId: 1, ChatId: "10", ClientMessageId: "key", Text: "Привет"})
	if err != nil || sent.Message.MessageId != "100" || sent.Message.GetSenderId() != 1 || sent.Message.GetText() != "Привет" || sent.Message.DeletedAt != nil || !sent.Message.EditedAt.AsTime().Equal(now) {
		t.Fatalf("sent = %v, error = %v", sent, err)
	}

	svc.EXPECT().MarkSeen(ctx, int64(1), int64(10)).Return(nil)
	if _, err := h.MarkSeen(ctx, &chatpb.MarkSeenRequest{UserId: 1, ChatId: "10"}); err != nil {
		t.Fatal(err)
	}

	svc.EXPECT().ReadChat(ctx, int64(1), model.ChatRead{ChatID: 10, LastReadSeq: 2}).Return(&model.ChatReadUpdated{LastReadSeq: 7}, nil)
	read, err := h.ReadChat(ctx, &chatpb.ReadChatRequest{UserId: 1, ChatId: "10", LastReadSeq: 2})
	if err != nil || read.LastReadSeq != 7 {
		t.Fatalf("read = %v, error = %v", read, err)
	}

	svc.EXPECT().GetMedia(ctx, int64(1), int64(10), int64(100)).Return("https://storage/media", nil)
	media, err := h.GetMedia(ctx, &chatpb.GetMediaRequest{UserId: 1, ChatId: "10", MessageId: "100"})
	if err != nil || media.DownloadUrl != "https://storage/media" {
		t.Fatalf("media = %v, error = %v", media, err)
	}
}

func TestMessagesCursor(t *testing.T) {
	for _, name := range []string{"без курсора", "before", "after zero"} {
		t.Run(name, func(t *testing.T) {
			svc := NewMockChatService(gomock.NewController(t))
			request := &chatpb.GetMessagesRequest{UserId: 1, ChatId: "10"}
			want := model.ChatMessagesQuery{}
			if name == "before" {
				request.Limit = ptr(int32(2))
				request.Cursor = &chatpb.GetMessagesRequest_BeforeSeq{BeforeSeq: 7}
				want.Limit = ptr(2)
				want.BeforeSeq = ptr(int64(7))
			}
			if name == "after zero" {
				request.Cursor = &chatpb.GetMessagesRequest_AfterSeq{AfterSeq: 0}
				want.AfterSeq = ptr(int64(0))
			}
			direction := model.ChatPageDirectionOlder
			expectedDirection := chatpb.ChatPageDirection_CHAT_PAGE_DIRECTION_OLDER
			if name == "after zero" {
				direction = model.ChatPageDirectionNewer
				expectedDirection = chatpb.ChatPageDirection_CHAT_PAGE_DIRECTION_NEWER
			}
			svc.EXPECT().GetMessages(gomock.Any(), int64(1), int64(10), want).Return(&model.ChatMessagesPage{Items: []model.ChatMessage{{MessageID: 100, Type: model.ChatMessageTypeSystem}}, Direction: direction, NextCursor: ptr(int64(5)), HasMore: true}, nil)
			response, err := delivery.NewChatHandler(svc).GetMessages(context.Background(), request)
			if err != nil || response.Page.GetNextCursor() != 5 || !response.Page.HasMore || response.Page.Direction != expectedDirection || len(response.Page.Items) != 1 || response.Page.Items[0].SenderId != nil || response.Page.Items[0].Text != nil {
				t.Fatalf("page = %v, error = %v", response, err)
			}
		})
	}
}

func TestInvalidRequests(t *testing.T) {
	h := delivery.NewChatHandler(NewMockChatService(gomock.NewController(t)))
	ctx := context.Background()
	checks := []func() error{
		func() error { _, err := h.EnsureChat(ctx, nil); return err },
		func() error { _, err := h.GetSummary(ctx, nil); return err },
		func() error { _, err := h.GetChat(ctx, nil); return err },
		func() error { _, err := h.GetMessages(ctx, nil); return err },
		func() error { _, err := h.MarkSeen(ctx, nil); return err },
		func() error { _, err := h.SendMessage(ctx, nil); return err },
		func() error { _, err := h.ReadChat(ctx, nil); return err },
		func() error { _, err := h.GetMedia(ctx, nil); return err },
		func() error { _, err := h.GetChat(ctx, &chatpb.GetChatRequest{ChatId: "abc"}); return err },
		func() error { _, err := h.SendMessage(ctx, &chatpb.SendMessageRequest{ChatId: "0"}); return err },
		func() error { _, err := h.GetMessages(ctx, &chatpb.GetMessagesRequest{ChatId: "-1"}); return err },
		func() error {
			_, err := h.MarkSeen(ctx, &chatpb.MarkSeenRequest{ChatId: "9223372036854775808"})
			return err
		},
		func() error { _, err := h.ReadChat(ctx, &chatpb.ReadChatRequest{}); return err },
		func() error {
			_, err := h.GetMedia(ctx, &chatpb.GetMediaRequest{ChatId: "10", MessageId: "abc"})
			return err
		},
	}
	for i, check := range checks {
		if err := check(); status.Code(err) != codes.InvalidArgument {
			t.Errorf("check %d: %v", i, err)
		}
	}
	if _, err := h.EditMessage(ctx, nil); status.Code(err) != codes.Unimplemented {
		t.Fatal(err)
	}
	if _, err := h.DeleteMessage(ctx, nil); status.Code(err) != codes.Unimplemented {
		t.Fatal(err)
	}
}

// Тестовый поток фиксирует отправленные protobuf-события.
type serverStream struct {
	grpcapi.ServerStream
	ctx     context.Context
	sent    []*chatpb.ChatEvent
	sendErr error
}

func (s *serverStream) Context() context.Context { return s.ctx }
func (s *serverStream) Send(event *chatpb.ChatEvent) error {
	if s.sendErr != nil {
		return s.sendErr
	}
	s.sent = append(s.sent, event)
	return nil
}

type subscription struct {
	events  []*model.ChatEvent
	recvErr error
	closed  bool
}

func (s *subscription) Recv() (*model.ChatEvent, error) {
	if len(s.events) == 0 {
		if s.recvErr != nil {
			return nil, s.recvErr
		}
		return nil, io.EOF
	}
	event := s.events[0]
	s.events = s.events[1:]
	return event, nil
}
func (s *subscription) Close() error { s.closed = true; return nil }

func TestSubscribeEvents(t *testing.T) {
	for _, name := range []string{"события", "ошибка подписки", "ошибка чтения", "ошибка отправки", "отмена контекста", "неизвестный payload"} {
		t.Run(name, func(t *testing.T) {
			svc := NewMockChatService(gomock.NewController(t))
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			stream := &serverStream{ctx: ctx}
			sub := &subscription{}
			subErr := error(nil)
			wantCode := codes.OK
			switch name {
			case "ошибка подписки":
				subErr = model.ErrInvalidUserID
				wantCode = codes.InvalidArgument
			case "ошибка чтения":
				sub.recvErr = errors.New("broker failed")
				wantCode = codes.Internal
			case "ошибка отправки":
				sub.events = []*model.ChatEvent{{Payload: model.ChatReadUpdated{}}}
				stream.sendErr = status.Error(codes.Unavailable, "connection closed")
				wantCode = codes.Unavailable
			case "отмена контекста":
				cancel()
				sub.recvErr = context.Canceled
				wantCode = codes.Canceled
			case "неизвестный payload":
				sub.events = []*model.ChatEvent{{}}
				wantCode = codes.Internal
			default:
				payloads := []model.ChatEventPayload{
					model.MessageCreated{ChatID: 10, ClientMessageID: ptr("key"), Message: model.ChatMessage{MessageID: 100, Type: model.ChatMessageTypeText}},
					model.MessageUpdated{ChatID: 10, Message: model.ChatMessage{MessageID: 100, Type: model.ChatMessageTypeVoice, Attachment: &model.ChatAttachment{URL: "/media", DurationSeconds: 2.5}}},
					model.MessageDeleted{ChatID: 10, Message: model.ChatMessage{MessageID: 100, Type: model.ChatMessageTypeVideoNote}},
					model.ChatCreated{Chat: model.ChatCard{ChatID: 10}},
					model.ChatUpdated{Chat: model.ChatCard{ChatID: 10}},
					model.ChatsSummaryUpdated{Summary: model.ChatSummary{NewChatsCount: 3}},
					model.ChatReadUpdated{ChatID: 10, UserID: 1, LastReadSeq: 7},
				}
				for _, payload := range payloads {
					sub.events = append(sub.events, &model.ChatEvent{EventID: "event", Payload: payload})
				}
			}
			svc.EXPECT().SubscribeEvents(ctx, int64(1)).Return(sub, subErr)
			err := delivery.NewChatHandler(svc).SubscribeEvents(&chatpb.SubscribeEventsRequest{UserId: 1}, stream)
			if status.Code(err) != wantCode {
				t.Fatalf("error = %v, want %v", err, wantCode)
			}
			if subErr == nil && !sub.closed {
				t.Fatal("subscription was not closed")
			}
			if name == "события" {
				if len(stream.sent) != 7 {
					t.Fatalf("events = %d", len(stream.sent))
				}
				for _, event := range stream.sent {
					if event.EventId != "event" || event.Payload == nil {
						t.Fatalf("event = %v", event)
					}
				}
				if stream.sent[0].GetMessageCreated().GetChatId() != "10" || stream.sent[0].GetMessageCreated().GetClientMessageId() != "key" || stream.sent[1].GetMessageUpdated().GetMessage().GetAttachment().GetDurationSeconds() != 2.5 || stream.sent[5].GetChatSummaryUpdated().GetNewChatsCount() != 3 || stream.sent[6].GetChatReadUpdated().GetLastReadSeq() != 7 {
					t.Fatalf("events = %v", stream.sent)
				}
			}
		})
	}
}

func TestSendMediaMapping(t *testing.T) {
	svc := NewMockChatService(gomock.NewController(t))
	h := delivery.NewChatHandler(svc)
	command := model.ChatSendMedia{ChatID: 10, ClientMessageID: "key", Type: model.ChatMessageTypeVoice, Data: []byte("file")}
	svc.EXPECT().SendMedia(gomock.Any(), int64(1), command).Return(&model.ChatMessage{MessageID: 100, Type: model.ChatMessageTypeVoice}, false, nil)
	got, err := h.SendMedia(context.Background(), &chatpb.SendMediaRequest{UserId: 1, ChatId: "10", ClientMessageId: "key", Type: chatpb.ChatMessageType_CHAT_MESSAGE_TYPE_VOICE, Data: []byte("file")})
	if err != nil || got.Message.MessageId != "100" || got.Created {
		t.Fatalf("response = %v, error = %v", got, err)
	}
	for _, request := range []*chatpb.SendMediaRequest{nil, {ChatId: "invalid"}, {ChatId: "10", Type: chatpb.ChatMessageType_CHAT_MESSAGE_TYPE_TEXT}} {
		if _, err := h.SendMedia(context.Background(), request); status.Code(err) != codes.InvalidArgument {
			t.Fatalf("error = %v", err)
		}
	}
}
