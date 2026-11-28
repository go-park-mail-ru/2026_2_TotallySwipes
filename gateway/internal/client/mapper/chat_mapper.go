package mapper

import (
	"dating-app/gateway/internal/model"
	chatpb "dating-app/services/chat/api/gen/go/chat/v1"
	"fmt"
	"strconv"
)

func ChatMessagesQueryToProto(userID, chatID int64, query model.ChatMessagesQuery) (*chatpb.GetMessagesRequest, error) {

	request := &chatpb.GetMessagesRequest{
		UserId: userID,
		ChatId: strconv.FormatInt(chatID, 10),
	}

	if query.Limit != nil {

		if *query.Limit < 1 || *query.Limit > 100 {
			return nil, fmt.Errorf("limit must be between 1 and 100")
		}

		limit := int32(*query.Limit)
		request.Limit = &limit
	}

	if query.BeforeSeq != nil && query.AfterSeq != nil {
		return nil, fmt.Errorf("before_seq and after_seq must not be set together")
	}

	if query.BeforeSeq != nil {

		if *query.BeforeSeq <= 0 {
			return nil, fmt.Errorf("before_seq must be positive")
		}

		request.Cursor = &chatpb.GetMessagesRequest_BeforeSeq{BeforeSeq: *query.BeforeSeq}
	}

	if query.AfterSeq != nil {

		if *query.AfterSeq < 0 {
			return nil, fmt.Errorf("after_seq must not be negative")
		}

		request.Cursor = &chatpb.GetMessagesRequest_AfterSeq{AfterSeq: *query.AfterSeq}
	}

	return request, nil
}

func ChatListQueryToProto(userID int64, query model.ChatListQuery) (*chatpb.GetChatsRequest, error) {

	request := &chatpb.GetChatsRequest{
		UserId: userID,
		Cursor: query.Cursor,
	}

	if query.Limit != nil {

		if *query.Limit < 1 || *query.Limit > 100 {
			return nil, fmt.Errorf("limit must be between 1 and 100")
		}

		limit := int32(*query.Limit)
		request.Limit = &limit
	}

	return request, nil
}

func ChatPageFromProto(page *chatpb.ChatPage) (*model.ChatPage, error) {

	if page == nil {
		return nil, fmt.Errorf("proto: missing chat page")
	}

	result := &model.ChatPage{
		Items:      make([]model.ChatCard, 0, len(page.GetItems())),
		NextCursor: page.NextCursor,
	}

	for _, item := range page.GetItems() {
		card, err := ChatCardFromProto(item)

		if err != nil {
			return nil, err
		}

		result.Items = append(result.Items, *card)
	}

	return result, nil
}

func ChatDetailsFromProto(details *chatpb.ChatDetails) (*model.ChatDetails, error) {

	if details == nil {
		return nil, fmt.Errorf("proto: missing chat details")
	}

	card, err := ChatCardFromProto(details.GetChat())

	if err != nil {
		return nil, err
	}

	messaging := details.GetMessaging()

	if messaging == nil {
		return nil, fmt.Errorf("proto: missing chat details.messaging")
	}

	return &model.ChatDetails{
		Chat:            *card,
		IsEmpty:         details.GetIsEmpty(),
		CommonTags:      append([]string{}, details.GetCommonTags()...),
		LastReadSeq:     details.GetLastReadSeq(),
		PeerLastReadSeq: details.GetPeerLastReadSeq(),
		Messaging: model.ChatMessaging{
			CanSend:     messaging.GetCanSend(),
			Restriction: messaging.Restriction,
		},
	}, nil
}

func ChatMessagesPageFromProto(page *chatpb.ChatMessagesPage) (*model.ChatMessagesPage, error) {

	if page == nil {
		return nil, fmt.Errorf("proto: missing chat messages page")
	}

	result := &model.ChatMessagesPage{
		Items:      make([]model.ChatMessage, 0, len(page.GetItems())),
		NextCursor: page.NextCursor,
		HasMore:    page.GetHasMore(),
	}

	switch page.GetDirection() {

	case chatpb.ChatPageDirection_CHAT_PAGE_DIRECTION_OLDER:
		result.Direction = model.ChatPageDirectionOlder

	case chatpb.ChatPageDirection_CHAT_PAGE_DIRECTION_NEWER:
		result.Direction = model.ChatPageDirectionNewer

	default:
		return nil, fmt.Errorf("proto: unknown chat page direction: %d", page.GetDirection())
	}

	for _, item := range page.GetItems() {
		message, err := ChatMessageFromProto(item)

		if err != nil {
			return nil, err
		}

		result.Items = append(result.Items, *message)
	}

	return result, nil
}

func ChatEventFromProto(event *chatpb.ChatEvent) (*model.ChatEvent, error) {

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

		message, err := ChatMessageFromProto(created.GetMessage())

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

		message, err := ChatMessageFromProto(updated.GetMessage())

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

		message, err := ChatMessageFromProto(deleted.GetMessage())

		if err != nil {
			return nil, err
		}

		result.Payload = model.MessageDeleted{
			ChatID:  chatID,
			Message: *message,
		}
	case *chatpb.ChatEvent_ChatCreated:
		card, err := ChatCardFromProto(payload.ChatCreated)

		if err != nil {
			return nil, err
		}

		result.Payload = model.ChatCreated{
			Chat: *card,
		}

	case *chatpb.ChatEvent_ChatUpdated:

		card, err := ChatCardFromProto(payload.ChatUpdated)

		if err != nil {
			return nil, err
		}

		result.Payload = model.ChatUpdated{Chat: *card}

	case *chatpb.ChatEvent_ChatSummaryUpdated:

		summary, err := ChatSummaryFromProto(payload.ChatSummaryUpdated)

		if err != nil {
			return nil, err
		}

		result.Payload = model.ChatsSummaryUpdated{Summary: *summary}

	case *chatpb.ChatEvent_ChatReadUpdated:

		read, err := ChatReadUpdatedFromProto(payload.ChatReadUpdated)

		if err != nil {
			return nil, err
		}

		result.Payload = *read

	default:
		return nil, fmt.Errorf("proto: missing or unsupported chat event payload: %T", event.GetPayload())
	}

	return result, nil
}

func ChatMessageFromProto(message *chatpb.ChatMessage) (*model.ChatMessage, error) {

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

	messageType, err := ChatMessageTypeFromProto(message.GetType())

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

func ChatCardFromProto(card *chatpb.ChatCard) (*model.ChatCard, error) {

	if card == nil {
		return nil, fmt.Errorf("proto: missing chat card")
	}

	chatID, err := ChatIDFromProto(card.GetChatId())

	if err != nil {
		return nil, err
	}

	peer, err := ChatPeerFromProto(card.GetPeer())

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
		lastMessage, err = ChatLastMessageFromProto(card.GetLastMessage())

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

func ChatPeerFromProto(peer *chatpb.ChatPeer) (*model.ChatPeer, error) {

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

func ChatLastMessageFromProto(message *chatpb.ChatLastMessage) (*model.ChatLastMessage, error) {

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

	messageType, err := ChatMessageTypeFromProto(message.GetType())

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

func ChatSummaryFromProto(summary *chatpb.ChatSummary) (*model.ChatSummary, error) {

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

func ChatReadUpdatedFromProto(read *chatpb.ChatReadUpdated) (*model.ChatReadUpdated, error) {

	if read == nil {
		return nil, fmt.Errorf("proto: missing chat_read_updated")
	}

	chatID, err := ChatIDFromProto(read.GetChatId())

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

func ChatIDFromProto(value string) (int64, error) {

	id, err := strconv.ParseInt(value, 10, 64)

	if err != nil {
		return 0, fmt.Errorf("proto: invalid chat_id: %w", err)
	}

	if id <= 0 {
		return 0, fmt.Errorf("proto: chat_id must be positive")
	}

	return id, nil
}

func ChatMessageTypeFromProto(value chatpb.ChatMessageType) (model.ChatMessageType, error) {

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

func SendMediaToProto(userID int64, command model.ChatSendMedia) (*chatpb.SendMediaRequest, error) {

	var mediaType chatpb.ChatMessageType

	switch command.Type {

	case model.ChatMessageTypeVoice:
		mediaType = chatpb.ChatMessageType_CHAT_MESSAGE_TYPE_VOICE

	case model.ChatMessageTypeVideoNote:
		mediaType = chatpb.ChatMessageType_CHAT_MESSAGE_TYPE_VIDEO_NOTE

	default:
		return nil, fmt.Errorf("unsupported media type: %q", command.Type)
	}

	return &chatpb.SendMediaRequest{
		UserId:          userID,
		ChatId:          strconv.FormatInt(command.ChatID, 10),
		ClientMessageId: command.ClientMessageID,
		Type:            mediaType,
		Data:            command.Data,
	}, nil
}

func SendMessageToProto(userID int64, command model.ChatSendMessage) *chatpb.SendMessageRequest {

	return &chatpb.SendMessageRequest{
		UserId:          userID,
		ChatId:          strconv.FormatInt(command.ChatID, 10),
		ClientMessageId: command.ClientMessageID,
		Text:            command.Text,
	}
}

func ReadChatToProto(userID int64, command model.ChatRead) *chatpb.ReadChatRequest {

	return &chatpb.ReadChatRequest{UserId: userID, ChatId: strconv.FormatInt(command.ChatID, 10), LastReadSeq: command.LastReadSeq}
}

func ReadChatResponseFromProto(response *chatpb.ReadChatResponse, userID, chatID int64) (*model.ChatReadUpdated, error) {

	if response == nil {
		return nil, fmt.Errorf("proto: missing read chat response")
	}

	return &model.ChatReadUpdated{ChatID: chatID, UserID: userID, LastReadSeq: response.GetLastReadSeq()}, nil
}
