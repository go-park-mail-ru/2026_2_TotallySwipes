package client

import (
	"context"
	"dating-app/gateway/internal/client/mapper"
	"dating-app/gateway/internal/model"
	chatpb "dating-app/services/chat/api/gen/go/chat/v1"
	"fmt"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// EditMessage пока не поддерживается; вызов Chat Service не выполняется.
func (a *ChatAdapter) EditMessage(ctx context.Context, userID int64, command model.ChatEditMessage) (*model.ChatMessage, error) {

	return nil, status.Error(codes.Unimplemented, "message editing is not implemented")
}

// DeleteMessage пока не поддерживается; вызов Chat Service не выполняется.
func (a *ChatAdapter) DeleteMessage(ctx context.Context, userID int64, command model.ChatDeleteMessage) (*model.ChatMessage, error) {

	return nil, status.Error(codes.Unimplemented, "message deletion is not implemented")
}

type ChatEventStream struct {
	stream grpc.ServerStreamingClient[chatpb.ChatEvent]
}

func (a *ChatAdapter) SubscribeEvents(ctx context.Context, userID int64) (*ChatEventStream, error) {

	stream, err := a.client.SubscribeEvents(ctx, &chatpb.SubscribeEventsRequest{UserId: userID})

	if err != nil {
		return nil, err
	}

	chatEventStream := ChatEventStream{
		stream: stream,
	}

	return &chatEventStream, nil
}

func (a *ChatAdapter) SendMessage(ctx context.Context, userID int64, command model.ChatSendMessage) (*model.ChatMessage, error) {

	response, err := a.client.SendMessage(ctx, mapper.SendMessageToProto(userID, command))

	if err != nil {
		return nil, err
	}

	if response == nil {
		return nil, fmt.Errorf("proto: missing send message response")
	}

	return mapper.ChatMessageFromProto(response.GetMessage())
}

func (a *ChatAdapter) ReadChat(ctx context.Context, userID int64, command model.ChatRead) (*model.ChatReadUpdated, error) {

	response, err := a.client.ReadChat(ctx, mapper.ReadChatToProto(userID, command))

	if err != nil {
		return nil, err
	}

	return mapper.ReadChatResponseFromProto(response, userID, command.ChatID)
}

func (s *ChatEventStream) Recv() (*model.ChatEvent, error) {

	event, err := s.stream.Recv()

	if err != nil {
		return nil, err
	}

	return mapper.ChatEventFromProto(event)
}
