package handler

import (
	"context"

	"dating-app/gateway/internal/model"
)

// ChatHTTPService описывает операции адаптера для HTTP ручек чата.
type ChaHTTPService interface {
	GetSummary(ctx context.Context, userID int64) (*model.ChatSummary, error)
	GetChat(ctx context.Context, userID, chatID int64) (*model.ChatDetails, error)
	MarkSeen(ctx context.Context, userID, chatID int64) error
	GetMessages(ctx context.Context, userID, chatID int64, query model.ChatMessagesQuery) (*model.ChatMessagesPage, error)

	GetMedia(ctx context.Context, userID, chatID, messageID int64) (string, error)
}

type ChatHTTPHandler struct {
	svc ChaHTTPService
}

func NewChatHandler(svc ChaHTTPService) *ChatHTTPHandler {
	return &ChatHTTPHandler{svc: svc}
}
