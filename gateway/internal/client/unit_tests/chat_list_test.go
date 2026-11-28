package client_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"dating-app/gateway/internal/client"
	"dating-app/gateway/internal/model"
	chatpb "dating-app/services/chat/api/gen/go/chat/v1"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type stubChatListClient struct {
	chatpb.ChatServiceClient
	request  *chatpb.GetChatsRequest
	response *chatpb.GetChatsResponse
	err      error
}

func (s *stubChatListClient) GetChats(_ context.Context, request *chatpb.GetChatsRequest, _ ...grpc.CallOption) (*chatpb.GetChatsResponse, error) {
	s.request = request
	return s.response, s.err
}

func TestChatAdapterGetChats(t *testing.T) {

	limit, cursor, next := 20, "0", "20"
	name, age, avatar := "Анна", int32(25), "https://example.com/avatar.jpg"
	created := timestamppb.New(time.Now())
	remote := &stubChatListClient{response: &chatpb.GetChatsResponse{Page: &chatpb.ChatPage{
		Items: []*chatpb.ChatCard{{
			ChatId: "42", MatchId: 10,
			Peer:      &chatpb.ChatPeer{UserId: 2, Name: &name, Age: &age, AvatarUrl: &avatar},
			CreatedAt: created, LastActivityAt: created, IsNew: true, UnreadCount: 3, Version: 7,
		}},
		NextCursor: &next,
	}}}
	adapter := client.NewChatAdapter(remote)

	page, err := adapter.GetChats(context.Background(), 1, model.ChatListQuery{Limit: &limit, Cursor: &cursor})
	if err != nil {
		t.Fatal(err)
	}

	if remote.request.GetUserId() != 1 || remote.request.Limit == nil || remote.request.GetLimit() != 20 || remote.request.Cursor == nil || remote.request.GetCursor() != "0" {
		t.Fatalf("unexpected request: %v", remote.request)
	}

	if len(page.Items) != 1 || page.NextCursor == nil || *page.NextCursor != next {
		t.Fatalf("unexpected page: %+v", page)
	}

	card := page.Items[0]
	if card.ChatID != 42 || card.MatchID != 10 || card.Version != 7 || !card.IsNew || card.UnreadCount != 3 || card.Peer.Name == nil || *card.Peer.Name != name || card.Peer.Age == nil || *card.Peer.Age != int(age) || card.Peer.AvatarURL == nil || *card.Peer.AvatarURL != avatar {
		t.Fatalf("unexpected card: %+v", card)
	}

	remote.response = &chatpb.GetChatsResponse{Page: &chatpb.ChatPage{}}
	page, err = adapter.GetChats(context.Background(), 1, model.ChatListQuery{})
	if err != nil || remote.request.Limit != nil || remote.request.Cursor != nil || page.Items == nil || page.NextCursor != nil {
		t.Fatalf("optional fields or empty page lost: page=%+v err=%v", page, err)
	}
}

func TestChatAdapterGetChatsErrors(t *testing.T) {

	failure := errors.New("rpc failure")
	remote := &stubChatListClient{err: failure}
	adapter := client.NewChatAdapter(remote)
	if page, err := adapter.GetChats(context.Background(), 1, model.ChatListQuery{}); !errors.Is(err, failure) || page != nil {
		t.Fatalf("page=%v error=%v", page, err)
	}

	remote.err = nil
	for _, response := range []*chatpb.GetChatsResponse{
		nil,
		{},
		{Page: &chatpb.ChatPage{Items: []*chatpb.ChatCard{nil}}},
	} {
		remote.response = response
		if page, err := adapter.GetChats(context.Background(), 1, model.ChatListQuery{}); err == nil || !strings.HasPrefix(err.Error(), "proto:") || page != nil {
			t.Fatalf("page=%v error=%v", page, err)
		}
	}

	remote.request = nil
	invalidLimit := 101
	if _, err := adapter.GetChats(context.Background(), 1, model.ChatListQuery{Limit: &invalidLimit}); err == nil || remote.request != nil {
		t.Fatal("invalid limit reached rpc")
	}
}
