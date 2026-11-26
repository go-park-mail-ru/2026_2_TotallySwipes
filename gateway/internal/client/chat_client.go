package client

import (
	"context"
	"dating-app/gateway/internal/client/mapper"
	"dating-app/gateway/internal/model"
	chatpb "dating-app/gen/go/chat/v1"
	"fmt"
	"strconv"
)

// ChatAdapter используется HTTP- и WebSocket-хэндлерами.
type ChatAdapter struct {
	client chatpb.ChatServiceClient
}

func NewChatAdapter(client chatpb.ChatServiceClient) *ChatAdapter {
	return &ChatAdapter{client: client}
}

func (a *ChatAdapter) GetSummary(ctx context.Context, userID int64) (*model.ChatSummary, error) {

	response, err := a.client.GetSummary(ctx, &chatpb.GetSummaryRequest{UserId: userID})

	if err != nil {
		return nil, err
	}

	return mapper.ChatSummaryFromProto(response.GetSummary())
}

func (a *ChatAdapter) GetChat(ctx context.Context, userID, chatID int64) (*model.ChatDetails, error) {

	response, err := a.client.GetChat(ctx, &chatpb.GetChatRequest{UserId: userID, ChatId: strconv.FormatInt(chatID, 10)})

	if err != nil {
		return nil, err
	}

	return mapper.ChatDetailsFromProto(response.GetDetails())
}

func (a *ChatAdapter) MarkSeen(ctx context.Context, userID, chatID int64) error {

	_, err := a.client.MarkSeen(ctx, &chatpb.MarkSeenRequest{UserId: userID, ChatId: strconv.FormatInt(chatID, 10)})

	return err
}

func (a *ChatAdapter) GetMessages(ctx context.Context, userID, chatID int64, query model.ChatMessagesQuery) (*model.ChatMessagesPage, error) {

	request, err := mapper.ChatMessagesQueryToProto(userID, chatID, query)

	if err != nil {
		return nil, err
	}

	response, err := a.client.GetMessages(ctx, request)

	if err != nil {
		return nil, err
	}

	return mapper.ChatMessagesPageFromProto(response.GetPage())
}

func (a *ChatAdapter) GetMedia(ctx context.Context, userID, chatID, messageID int64) (string, error) {
	response, err := a.client.GetMedia(ctx, &chatpb.GetMediaRequest{UserId: userID, ChatId: strconv.FormatInt(chatID, 10), MessageId: strconv.FormatInt(messageID, 10)})

	if err != nil {
		return "", err
	}

	if response.GetDownloadUrl() == "" {
		return "", fmt.Errorf("proto: missing download_url")
	}

	return response.GetDownloadUrl(), nil
}
