package grpc

import (
	"fmt"
	"strconv"
	"time"

	chatpb "dating-app/services/chat/api/gen/go/chat/v1"
	"dating-app/services/chat/internal/model"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func chatSummaryToProto(summary *model.ChatSummary) *chatpb.ChatSummary {

	if summary == nil {
		return nil
	}

	return &chatpb.ChatSummary{
		NewChatsCount:       summary.NewChatsCount,
		UnreadChatsCount:    summary.UnreadChatsCount,
		UnreadMessagesCount: summary.UnreadMessagesCount,
		Version:             summary.Version,
	}
}

func chatMessageTypeToProto(value model.ChatMessageType) chatpb.ChatMessageType {

	switch value {

	case model.ChatMessageTypeText:
		return chatpb.ChatMessageType_CHAT_MESSAGE_TYPE_TEXT

	case model.ChatMessageTypeVoice:
		return chatpb.ChatMessageType_CHAT_MESSAGE_TYPE_VOICE

	case model.ChatMessageTypeVideoNote:
		return chatpb.ChatMessageType_CHAT_MESSAGE_TYPE_VIDEO_NOTE

	case model.ChatMessageTypeSystem:
		return chatpb.ChatMessageType_CHAT_MESSAGE_TYPE_SYSTEM

	default:
		return chatpb.ChatMessageType_CHAT_MESSAGE_TYPE_UNSPECIFIED
	}
}

func optionalTimestamp(value *time.Time) *timestamppb.Timestamp {

	if value == nil {
		return nil
	}

	return timestamppb.New(*value)
}

func chatMessageToProto(message *model.ChatMessage) *chatpb.ChatMessage {

	if message == nil {
		return nil
	}

	result := &chatpb.ChatMessage{
		MessageId: strconv.FormatInt(message.MessageID, 10),
		Seq:       message.Seq,
		SenderId:  message.SenderID,
		Type:      chatMessageTypeToProto(message.Type),
		Text:      message.Text,
		CreatedAt: timestamppb.New(message.CreatedAt),
		EditedAt:  optionalTimestamp(message.EditedAt),
		DeletedAt: optionalTimestamp(message.DeletedAt),
		Version:   message.Version,
	}

	if message.Attachment != nil {
		result.Attachment = &chatpb.ChatAttachment{
			Url:             message.Attachment.URL,
			DurationSeconds: message.Attachment.DurationSeconds,
		}
	}

	return result
}

func chatCardToProto(card model.ChatCard) *chatpb.ChatCard {

	peer := &chatpb.ChatPeer{
		UserId:    card.Peer.UserID,
		Name:      card.Peer.Name,
		AvatarUrl: card.Peer.AvatarURL,
	}

	if card.Peer.Age != nil {
		age := int32(*card.Peer.Age)
		peer.Age = &age
	}

	result := &chatpb.ChatCard{
		ChatId:         strconv.FormatInt(card.ChatID, 10),
		MatchId:        card.MatchID,
		Peer:           peer,
		IsNew:          card.IsNew,
		UnreadCount:    card.UnreadCount,
		CreatedAt:      timestamppb.New(card.CreatedAt),
		LastActivityAt: timestamppb.New(card.LastActivityAt),
		Version:        card.Version,
	}

	if last := card.LastMessage; last != nil {
		result.LastMessage = &chatpb.ChatLastMessage{
			MessageId:       strconv.FormatInt(last.MessageID, 10),
			Seq:             last.Seq,
			SenderId:        last.SenderID,
			Type:            chatMessageTypeToProto(last.Type),
			TextPreview:     last.TextPreview,
			DurationSeconds: last.DurationSeconds,
			CreatedAt:       timestamppb.New(last.CreatedAt),
			IsReadByPeer:    last.IsReadByPeer,
		}
	}

	return result
}

func chatPageToProto(page *model.ChatPage) *chatpb.ChatPage {

	if page == nil {
		return nil
	}

	items := make([]*chatpb.ChatCard, len(page.Items))

	for index, card := range page.Items {
		items[index] = chatCardToProto(card)
	}

	return &chatpb.ChatPage{
		Items:      items,
		NextCursor: page.NextCursor,
	}
}

func chatDetailsToProto(details *model.ChatDetails) *chatpb.ChatDetails {

	if details == nil {
		return nil
	}

	return &chatpb.ChatDetails{
		Chat:            chatCardToProto(details.Chat),
		IsEmpty:         details.IsEmpty,
		CommonTags:      details.CommonTags,
		LastReadSeq:     details.LastReadSeq,
		PeerLastReadSeq: details.PeerLastReadSeq,
		Messaging: &chatpb.ChatMessaging{
			CanSend:     details.Messaging.CanSend,
			Restriction: details.Messaging.Restriction,
		},
	}
}

func chatMessagesQueryFromProto(request *chatpb.GetMessagesRequest) model.ChatMessagesQuery {

	var query model.ChatMessagesQuery

	if request.Limit != nil {
		limit := int(*request.Limit)
		query.Limit = &limit
	}

	switch cursor := request.GetCursor().(type) {

	case *chatpb.GetMessagesRequest_BeforeSeq:
		seq := cursor.BeforeSeq
		query.BeforeSeq = &seq

	case *chatpb.GetMessagesRequest_AfterSeq:
		seq := cursor.AfterSeq
		query.AfterSeq = &seq
	}

	return query
}

func chatMessagesPageToProto(page *model.ChatMessagesPage) *chatpb.ChatMessagesPage {

	if page == nil {
		return nil
	}

	result := &chatpb.ChatMessagesPage{
		Items:      make([]*chatpb.ChatMessage, 0, len(page.Items)),
		NextCursor: page.NextCursor,
		HasMore:    page.HasMore,
	}

	switch page.Direction {

	case model.ChatPageDirectionOlder:
		result.Direction = chatpb.ChatPageDirection_CHAT_PAGE_DIRECTION_OLDER

	case model.ChatPageDirectionNewer:
		result.Direction = chatpb.ChatPageDirection_CHAT_PAGE_DIRECTION_NEWER
	}

	for _, message := range page.Items {
		result.Items = append(result.Items, chatMessageToProto(&message))
	}

	return result
}

func chatEventToProto(event *model.ChatEvent) (*chatpb.ChatEvent, error) {

	if event == nil {
		return nil, fmt.Errorf("proto: missing chat event")
	}

	result := &chatpb.ChatEvent{EventId: event.EventID}

	switch payload := event.Payload.(type) {

	case model.MessageCreated:
		result.Payload = &chatpb.ChatEvent_MessageCreated{
			MessageCreated: &chatpb.MessageCreated{
				ChatId:          strconv.FormatInt(payload.ChatID, 10),
				ClientMessageId: payload.ClientMessageID,
				Message:         chatMessageToProto(&payload.Message),
			},
		}

	case model.MessageUpdated:
		result.Payload = &chatpb.ChatEvent_MessageUpdated{
			MessageUpdated: &chatpb.MessageUpdated{
				ChatId:  strconv.FormatInt(payload.ChatID, 10),
				Message: chatMessageToProto(&payload.Message),
			},
		}

	case model.MessageDeleted:
		result.Payload = &chatpb.ChatEvent_MessageDeleted{
			MessageDeleted: &chatpb.MessageDeleted{
				ChatId:  strconv.FormatInt(payload.ChatID, 10),
				Message: chatMessageToProto(&payload.Message),
			},
		}

	case model.ChatCreated:
		result.Payload = &chatpb.ChatEvent_ChatCreated{ChatCreated: chatCardToProto(payload.Chat)}

	case model.ChatUpdated:
		result.Payload = &chatpb.ChatEvent_ChatUpdated{ChatUpdated: chatCardToProto(payload.Chat)}

	case model.ChatsSummaryUpdated:
		result.Payload = &chatpb.ChatEvent_ChatSummaryUpdated{ChatSummaryUpdated: chatSummaryToProto(&payload.Summary)}

	case model.ChatReadUpdated:
		result.Payload = &chatpb.ChatEvent_ChatReadUpdated{
			ChatReadUpdated: &chatpb.ChatReadUpdated{
				ChatId:      strconv.FormatInt(payload.ChatID, 10),
				UserId:      payload.UserID,
				LastReadSeq: payload.LastReadSeq,
			},
		}

	default:
		return nil, fmt.Errorf("proto: unsupported chat event payload %T", event.Payload)
	}

	return result, nil
}

func optionalChatsLimit(request *chatpb.GetChatsRequest) *int {

	if request.Limit == nil {
		return nil
	}

	limit := int(request.GetLimit())
	return &limit
}
