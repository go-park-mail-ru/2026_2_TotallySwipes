package client

import (
	"context"
	chatpb "dating-app/services/chat/api/gen/go/chat/v1"
	"fmt"
	"strconv"
)

type ChatAdapter struct {
	client chatpb.ChatServiceClient
}

func NewChatAdapter(client chatpb.ChatServiceClient) *ChatAdapter {

	return &ChatAdapter{client: client}
}

func (a *ChatAdapter) EnsureChat(ctx context.Context, matchID, firstUserID, secondUserID int64) (chatID int64, created bool, err error) {

	response, err := a.client.EnsureChat(ctx, &chatpb.EnsureChatRequest{
		MatchId:      matchID,
		FirstUserId:  firstUserID,
		SecondUserId: secondUserID,
	})

	if err != nil {
		return 0, false, err
	}

	if response == nil {
		return 0, false, fmt.Errorf("proto: missing ensure chat response")
	}

	chatID, err = strconv.ParseInt(response.GetChatId(), 10, 64)

	if err != nil {
		return 0, false, fmt.Errorf("proto: invalid chat_id: %w", err)
	}

	if chatID <= 0 {
		return 0, false, fmt.Errorf("proto: chat_id must be positive")
	}

	return chatID, response.GetCreated(), nil
}
