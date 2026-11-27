package grpc

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"strconv"

	chatpb "dating-app/services/chat/api/gen/go/chat/v1"
	"dating-app/services/chat/internal/model"
	"dating-app/services/chat/internal/service"

	grpcapi "google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type ChatService interface {
	SendMedia(ctx context.Context, userID int64, command model.ChatSendMedia) (*model.ChatMessage, bool, error)
	EnsureChat(ctx context.Context, matchID, firstUserID, secondUserID int64) (int64, bool, error)
	GetSummary(ctx context.Context, userID int64) (*model.ChatSummary, error)
	GetChat(ctx context.Context, userID, chatID int64) (*model.ChatDetails, error)
	GetMessages(ctx context.Context, userID, chatID int64, query model.ChatMessagesQuery) (*model.ChatMessagesPage, error)
	MarkSeen(ctx context.Context, userID, chatID int64) error
	SendMessage(ctx context.Context, userID int64, command model.ChatSendMessage) (*model.ChatMessage, error)
	ReadChat(ctx context.Context, userID int64, command model.ChatRead) (*model.ChatReadUpdated, error)
	GetMedia(ctx context.Context, userID, chatID, messageID int64) (string, error)
	SubscribeEvents(ctx context.Context, userID int64) (service.ChatEventsStream, error)
}

type ChatHandler struct {
	chatpb.UnimplementedChatServiceServer
	chat ChatService
}

var _ chatpb.ChatServiceServer = (*ChatHandler)(nil)

func NewChatHandler(chat ChatService) *ChatHandler {

	return &ChatHandler{chat: chat}
}

func (h *ChatHandler) EnsureChat(ctx context.Context, request *chatpb.EnsureChatRequest) (*chatpb.EnsureChatResponse, error) {

	if request == nil {
		return nil, status.Error(codes.InvalidArgument, "missing request")
	}

	chatID, created, err := h.chat.EnsureChat(ctx, request.GetMatchId(), request.GetFirstUserId(), request.GetSecondUserId())

	if err != nil {
		return nil, chatError(ctx, err)
	}

	return &chatpb.EnsureChatResponse{ChatId: strconv.FormatInt(chatID, 10), Created: created}, nil
}

func (h *ChatHandler) GetSummary(ctx context.Context, request *chatpb.GetSummaryRequest) (*chatpb.GetSummaryResponse, error) {

	if request == nil {
		return nil, status.Error(codes.InvalidArgument, "missing request")
	}

	summary, err := h.chat.GetSummary(ctx, request.GetUserId())

	if err != nil {
		return nil, chatError(ctx, err)
	}

	return &chatpb.GetSummaryResponse{Summary: chatSummaryToProto(summary)}, nil
}

func (h *ChatHandler) GetChat(ctx context.Context, request *chatpb.GetChatRequest) (*chatpb.GetChatResponse, error) {

	if request == nil {
		return nil, status.Error(codes.InvalidArgument, "missing request")
	}

	chatID, err := parseChatID(request.GetChatId())

	if err != nil {
		return nil, chatError(ctx, err)
	}

	details, err := h.chat.GetChat(ctx, request.GetUserId(), chatID)

	if err != nil {
		return nil, chatError(ctx, err)
	}

	return &chatpb.GetChatResponse{Details: chatDetailsToProto(details)}, nil
}

func (h *ChatHandler) GetMessages(ctx context.Context, request *chatpb.GetMessagesRequest) (*chatpb.GetMessagesResponse, error) {

	if request == nil {
		return nil, status.Error(codes.InvalidArgument, "missing request")
	}

	chatID, err := parseChatID(request.GetChatId())

	if err != nil {
		return nil, chatError(ctx, err)
	}

	page, err := h.chat.GetMessages(ctx, request.GetUserId(), chatID, chatMessagesQueryFromProto(request))

	if err != nil {
		return nil, chatError(ctx, err)
	}

	return &chatpb.GetMessagesResponse{Page: chatMessagesPageToProto(page)}, nil
}

func (h *ChatHandler) MarkSeen(ctx context.Context, request *chatpb.MarkSeenRequest) (*chatpb.MarkSeenResponse, error) {

	if request == nil {
		return nil, status.Error(codes.InvalidArgument, "missing request")
	}

	chatID, err := parseChatID(request.GetChatId())

	if err != nil {
		return nil, chatError(ctx, err)
	}

	if err := h.chat.MarkSeen(ctx, request.GetUserId(), chatID); err != nil {
		return nil, chatError(ctx, err)
	}

	return &chatpb.MarkSeenResponse{}, nil
}

func (h *ChatHandler) SendMessage(ctx context.Context, request *chatpb.SendMessageRequest) (*chatpb.SendMessageResponse, error) {

	if request == nil {
		return nil, status.Error(codes.InvalidArgument, "missing request")
	}

	chatID, err := parseChatID(request.GetChatId())

	if err != nil {
		return nil, chatError(ctx, err)
	}

	command := model.ChatSendMessage{
		ChatID:          chatID,
		ClientMessageID: request.GetClientMessageId(),
		Text:            request.GetText(),
	}

	message, err := h.chat.SendMessage(ctx, request.GetUserId(), command)

	if err != nil {
		return nil, chatError(ctx, err)
	}

	return &chatpb.SendMessageResponse{Message: chatMessageToProto(message)}, nil
}

func (h *ChatHandler) ReadChat(ctx context.Context, request *chatpb.ReadChatRequest) (*chatpb.ReadChatResponse, error) {

	if request == nil {
		return nil, status.Error(codes.InvalidArgument, "missing request")
	}

	chatID, err := parseChatID(request.GetChatId())

	if err != nil {
		return nil, chatError(ctx, err)
	}

	updated, err := h.chat.ReadChat(ctx, request.GetUserId(), model.ChatRead{ChatID: chatID, LastReadSeq: request.GetLastReadSeq()})

	if err != nil {
		return nil, chatError(ctx, err)
	}

	return &chatpb.ReadChatResponse{LastReadSeq: updated.LastReadSeq}, nil
}

func (h *ChatHandler) GetMedia(ctx context.Context, request *chatpb.GetMediaRequest) (*chatpb.GetMediaResponse, error) {

	if request == nil {
		return nil, status.Error(codes.InvalidArgument, "missing request")
	}

	chatID, err := parseChatID(request.GetChatId())

	if err != nil {
		return nil, chatError(ctx, err)
	}

	messageID, err := strconv.ParseInt(request.GetMessageId(), 10, 64)

	if err != nil || messageID <= 0 {
		return nil, chatError(ctx, model.ErrInvalidMessageID)
	}

	url, err := h.chat.GetMedia(ctx, request.GetUserId(), chatID, messageID)

	if err != nil {
		return nil, chatError(ctx, err)
	}

	return &chatpb.GetMediaResponse{DownloadUrl: url}, nil
}

func (h *ChatHandler) SubscribeEvents(request *chatpb.SubscribeEventsRequest, stream grpcapi.ServerStreamingServer[chatpb.ChatEvent]) error {

	ctx := stream.Context()

	if request == nil {
		return status.Error(codes.InvalidArgument, "missing request")
	}

	subscription, err := h.chat.SubscribeEvents(ctx, request.GetUserId())

	if err != nil {
		return chatError(ctx, err)
	}

	defer func() {
		if err := subscription.Close(); err != nil {
			slog.ErrorContext(ctx, "failed to close chat subscription", "user_id", request.GetUserId(), "error", err)
		}
	}()

	for {

		event, err := subscription.Recv()

		if err != nil {

			if ctx.Err() != nil {
				return chatError(ctx, ctx.Err())
			}

			if errors.Is(err, io.EOF) {
				return nil
			}

			return chatError(ctx, err)
		}

		response, err := chatEventToProto(event)

		if err != nil {
			return chatError(ctx, err)
		}

		if err := stream.Send(response); err != nil {
			return err
		}
	}
}

func (h *ChatHandler) EditMessage(context.Context, *chatpb.EditMessageRequest) (*chatpb.EditMessageResponse, error) {

	return nil, status.Error(codes.Unimplemented, "editing messages is not implemented")
}

func (h *ChatHandler) DeleteMessage(context.Context, *chatpb.DeleteMessageRequest) (*chatpb.DeleteMessageResponse, error) {

	return nil, status.Error(codes.Unimplemented, "deleting messages is not implemented")
}

func parseChatID(value string) (int64, error) {

	id, err := strconv.ParseInt(value, 10, 64)

	if err != nil || id <= 0 {
		return 0, model.ErrInvalidChatID
	}

	return id, nil
}

func (h *ChatHandler) SendMedia(ctx context.Context, request *chatpb.SendMediaRequest) (*chatpb.SendMediaResponse, error) {

	if request == nil {
		return nil, status.Error(codes.InvalidArgument, "missing request")
	}

	chatID, err := parseChatID(request.GetChatId())

	if err != nil {
		return nil, chatError(ctx, err)
	}

	var mediaType model.ChatMessageType

	switch request.GetType() {

	case chatpb.ChatMessageType_CHAT_MESSAGE_TYPE_VOICE:
		mediaType = model.ChatMessageTypeVoice

	case chatpb.ChatMessageType_CHAT_MESSAGE_TYPE_VIDEO_NOTE:
		mediaType = model.ChatMessageTypeVideoNote

	default:
		return nil, chatError(ctx, model.ErrInvalidMedia)
	}

	message, created, err := h.chat.SendMedia(ctx, request.GetUserId(), model.ChatSendMedia{
		ChatID:          chatID,
		ClientMessageID: request.GetClientMessageId(),
		Type:            mediaType,
		Data:            request.GetData(),
	})

	if err != nil {
		return nil, chatError(ctx, err)
	}

	return &chatpb.SendMediaResponse{Message: chatMessageToProto(message), Created: created}, nil
}
