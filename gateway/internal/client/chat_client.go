package client

import (
	"context"
	"dating-app/gateway/internal/model"
	chatpb "dating-app/gen/go/chat/v1"
	"fmt"
	"strconv"

	"google.golang.org/grpc"
)

type ChatAdapter struct {
	client chatpb.ChatServiceClient
}

func NewChatAdapter(client chatpb.ChatServiceClient) *ChatAdapter {
	return &ChatAdapter{client: client}
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

func (s *ChatEventStream) Recv() (*model.ChatEvent, error) {

	event, err := s.stream.Recv()

	if err != nil {
		return nil, err
	}

	return chatEventFromProto(event)
}

func chatEventFromProto(event *chatpb.ChatEvent) (*model.ChatEvent, error) {

	if event == nil {
		return nil, fmt.Errorf("proto: missing chat event")
	}

	result := &model.ChatEvent{
		EventID: event.GetEventId(),
	}

	switch payload := event.GetPayload().(type) {
	case *chatpb.ChatEvent_MessageCreated:
		created := payload.MessageCreated

		if created == nil {
			return nil, fmt.Errorf("proto: missing message_created")
		}

		chatID, err := strconv.ParseInt(created.GetChatId(), 10, 64)

		if err != nil {
			return nil, fmt.Errorf("proto: invalid message_created.chat_id: %w", err)
		}

		if chatID <= 0 {
			return nil, fmt.Errorf("proto: message_created.chat_id must be positive")
		}

		message, err := chatMessageFromProto(created.GetMessage())

		if err != nil {
			return nil, err
		}

		result.Payload = model.MessageCreated{
			ChatID:          chatID,
			ClientMessageID: created.ClientMessageId,
			Message:         *message,
		}
	case *chatpb.ChatEvent_MessageUpdated:
		updated := payload.MessageUpdated

		if updated == nil {
			return nil, fmt.Errorf("proto: missing message_updated")
		}

		chatID, err := strconv.ParseInt(updated.GetChatId(), 10, 64)

		if err != nil {
			return nil, fmt.Errorf("proto: invalid message_updated.chat_id: %w", err)
		}

		if chatID <= 0 {
			return nil, fmt.Errorf("proto: message_updated.chat_id must be positive")
		}

		message, err := chatMessageFromProto(updated.GetMessage())

		if err != nil {
			return nil, err
		}

		result.Payload = model.MessageUpdated{
			ChatID:  chatID,
			Message: *message,
		}
	case *chatpb.ChatEvent_MessageDeleted:
		deleted := payload.MessageDeleted

		if deleted == nil {
			return nil, fmt.Errorf("proto: missing message_deleted")
		}

		chatID, err := strconv.ParseInt(deleted.GetChatId(), 10, 64)

		if err != nil {
			return nil, fmt.Errorf("proto: invalid message_deleted.chat_id: %w", err)
		}

		if chatID <= 0 {
			return nil, fmt.Errorf("proto: message_deleted.chat_id must be positive")
		}

		message, err := chatMessageFromProto(deleted.GetMessage())

		if err != nil {
			return nil, err
		}

		result.Payload = model.MessageDeleted{
			ChatID:  chatID,
			Message: *message,
		}
	case *chatpb.ChatEvent_ChatCreated:
		card, err := chatCardFromProto(payload.ChatCreated)

		if err != nil {
			return nil, err
		}

		result.Payload = model.ChatCreated{
			Chat: *card,
		}

	case *chatpb.ChatEvent_ChatUpdated:

		card, err := chatCardFromProto(payload.ChatUpdated)

		if err != nil {
			return nil, err
		}

		result.Payload = model.ChatUpdated{Chat: *card}

	case *chatpb.ChatEvent_ChatSummaryUpdated:

		summary, err := chatSummaryFromProto(payload.ChatSummaryUpdated)

		if err != nil {
			return nil, err
		}

		result.Payload = model.ChatsSummaryUpdated{Summary: *summary}

	case *chatpb.ChatEvent_ChatReadUpdated:

		read, err := chatReadUpdatedFromProto(payload.ChatReadUpdated)

		if err != nil {
			return nil, err
		}

		result.Payload = *read

	default:
		return nil, fmt.Errorf("proto: missing or unsupported chat event payload: %T", event.GetPayload())
	}

	return result, nil
}

func chatMessageFromProto(message *chatpb.ChatMessage) (*model.ChatMessage, error) {

	if message == nil {
		return nil, fmt.Errorf("proto: missing chat message")
	}

	messageID, err := strconv.ParseInt(message.GetMessageId(), 10, 64)

	if err != nil {
		return nil, fmt.Errorf("proto: invalid message_id: %w", err)
	}

	if messageID <= 0 {
		return nil, fmt.Errorf("proto: message_id must be positive")
	}

	if err := message.GetCreatedAt().CheckValid(); err != nil {
		return nil, fmt.Errorf("proto: invalid message.created_at: %w", err)
	}

	result := &model.ChatMessage{
		MessageID: messageID,
		Seq:       message.GetSeq(),
		SenderID:  message.SenderId,
		Text:      message.Text,
		CreatedAt: message.GetCreatedAt().AsTime(),
		Version:   message.GetVersion(),
	}

	messageType, err := chatMessageTypeFromProto(message.GetType())

	if err != nil {
		return nil, err
	}

	result.Type = messageType

	if attachment := message.GetAttachment(); attachment != nil {
		result.Attachment = &model.ChatAttachment{
			URL:             attachment.GetUrl(),
			DurationSeconds: attachment.GetDurationSeconds(),
		}
	}

	if editedAt := message.GetEditedAt(); editedAt != nil {

		if err := editedAt.CheckValid(); err != nil {
			return nil, fmt.Errorf("proto: invalid message.edited_at: %w", err)
		}

		value := editedAt.AsTime()
		result.EditedAt = &value
	}

	if deletedAt := message.GetDeletedAt(); deletedAt != nil {

		if err := deletedAt.CheckValid(); err != nil {
			return nil, fmt.Errorf("proto: invalid message.deleted_at: %w", err)
		}

		value := deletedAt.AsTime()
		result.DeletedAt = &value
	}

	return result, nil
}

func chatCardFromProto(card *chatpb.ChatCard) (*model.ChatCard, error) {

	if card == nil {
		return nil, fmt.Errorf("proto: missing chat card")
	}

	chatID, err := chatIDFromProto(card.GetChatId())

	if err != nil {
		return nil, err
	}

	peer, err := chatPeerFromProto(card.GetPeer())

	if err != nil {
		return nil, err
	}

	if err := card.GetCreatedAt().CheckValid(); err != nil {
		return nil, fmt.Errorf("proto: invalid chat_card.created_at: %w", err)
	}

	if err := card.GetLastActivityAt().CheckValid(); err != nil {
		return nil, fmt.Errorf("proto: invalid chat_card.last_activity_at: %w", err)
	}

	var lastMessage *model.ChatLastMessage
	if card.GetLastMessage() != nil {
		lastMessage, err = chatLastMessageFromProto(card.GetLastMessage())

		if err != nil {
			return nil, err
		}
	}

	return &model.ChatCard{
		ChatID:         chatID,
		MatchID:        card.GetMatchId(),
		Peer:           *peer,
		IsNew:          card.GetIsNew(),
		UnreadCount:    card.GetUnreadCount(),
		LastMessage:    lastMessage,
		CreatedAt:      card.GetCreatedAt().AsTime(),
		LastActivityAt: card.GetLastActivityAt().AsTime(),
		Version:        card.GetVersion(),
	}, nil
}

func chatPeerFromProto(peer *chatpb.ChatPeer) (*model.ChatPeer, error) {

	if peer == nil {
		return nil, fmt.Errorf("proto: missing chat peer")
	}

	result := &model.ChatPeer{
		UserID:    peer.GetUserId(),
		Name:      peer.Name,
		AvatarURL: peer.AvatarUrl,
	}

	if peer.Age != nil {
		age := int(peer.GetAge())
		result.Age = &age
	}

	return result, nil
}

func chatLastMessageFromProto(message *chatpb.ChatLastMessage) (*model.ChatLastMessage, error) {

	if message == nil {
		return nil, fmt.Errorf("proto: missing chat last message")
	}

	messageID, err := strconv.ParseInt(message.GetMessageId(), 10, 64)

	if err != nil {
		return nil, fmt.Errorf("proto: invalid last_message.message_id: %w", err)
	}

	if messageID <= 0 {
		return nil, fmt.Errorf("proto: last_message.message_id must be positive")
	}

	messageType, err := chatMessageTypeFromProto(message.GetType())

	if err != nil {
		return nil, err
	}

	if err := message.GetCreatedAt().CheckValid(); err != nil {
		return nil, fmt.Errorf("proto: invalid last_message.created_at: %w", err)
	}

	return &model.ChatLastMessage{
		MessageID:       messageID,
		Seq:             message.GetSeq(),
		SenderID:        message.SenderId,
		Type:            messageType,
		TextPreview:     message.TextPreview,
		DurationSeconds: message.DurationSeconds,
		CreatedAt:       message.GetCreatedAt().AsTime(),
		IsReadByPeer:    message.IsReadByPeer,
	}, nil
}

func chatSummaryFromProto(summary *chatpb.ChatSummary) (*model.ChatSummary, error) {

	if summary == nil {
		return nil, fmt.Errorf("proto: missing chat summary")
	}

	return &model.ChatSummary{
		NewChatsCount:       summary.GetNewChatsCount(),
		UnreadChatsCount:    summary.GetUnreadChatsCount(),
		UnreadMessagesCount: summary.GetUnreadMessagesCount(),
		Version:             summary.GetVersion(),
	}, nil
}

func chatReadUpdatedFromProto(read *chatpb.ChatReadUpdated) (*model.ChatReadUpdated, error) {

	if read == nil {
		return nil, fmt.Errorf("proto: missing chat_read_updated")
	}

	chatID, err := chatIDFromProto(read.GetChatId())

	if err != nil {
		return nil, err
	}

	if read.GetLastReadSeq() < 0 {
		return nil, fmt.Errorf("proto: chat_read_updated.last_read_seq must not be negative")
	}

	return &model.ChatReadUpdated{
		ChatID:      chatID,
		UserID:      read.GetUserId(),
		LastReadSeq: read.GetLastReadSeq(),
	}, nil
}

func chatIDFromProto(value string) (int64, error) {

	id, err := strconv.ParseInt(value, 10, 64)

	if err != nil {
		return 0, fmt.Errorf("proto: invalid chat_id: %w", err)
	}

	if id <= 0 {
		return 0, fmt.Errorf("proto: chat_id must be positive")
	}

	return id, nil
}

func chatMessageTypeFromProto(value chatpb.ChatMessageType) (model.ChatMessageType, error) {

	switch value {

	case chatpb.ChatMessageType_CHAT_MESSAGE_TYPE_TEXT:
		return model.ChatMessageTypeText, nil

	case chatpb.ChatMessageType_CHAT_MESSAGE_TYPE_VOICE:
		return model.ChatMessageTypeVoice, nil

	case chatpb.ChatMessageType_CHAT_MESSAGE_TYPE_VIDEO_NOTE:
		return model.ChatMessageTypeVideoNote, nil

	case chatpb.ChatMessageType_CHAT_MESSAGE_TYPE_SYSTEM:
		return model.ChatMessageTypeSystem, nil

	default:
		return "", fmt.Errorf("proto: unknown chat message type: %d", value)
	}
}
