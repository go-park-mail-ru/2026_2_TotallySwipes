package dto

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"dating-app/gateway/internal/model"
)

type ChatEvent struct {
	Type    string `json:"type"`
	EventID string `json:"event_id"`
	Data    any    `json:"data"`
}

type ChatReadUpdated struct {
	ChatID      string `json:"chat_id"`
	UserID      int64  `json:"user_id"`
	LastReadSeq int64  `json:"last_read_seq"`
}

type ChatMessageResponse struct {
	MessageID  string                  `json:"message_id"`
	Seq        int64                   `json:"seq"`
	SenderID   *int64                  `json:"sender_id"`
	Type       model.ChatMessageType   `json:"type"`
	Text       *string                 `json:"text"`
	Attachment *ChatAttachmentResponse `json:"attachment"`
	CreatedAt  time.Time               `json:"created_at"`
	EditedAt   *time.Time              `json:"edited_at"`
	DeletedAt  *time.Time              `json:"deleted_at"`
	Version    int64                   `json:"version"`
}

type ChatAttachmentResponse struct {
	URL             string  `json:"url"`
	DurationSeconds float64 `json:"duration_seconds"`
}

type ChatPeerResponse struct {
	UserID    int64   `json:"user_id"`
	Name      *string `json:"name"`
	Age       *int    `json:"age"`
	AvatarURL *string `json:"avatar_url"`
}

type ChatLastMessageResponse struct {
	MessageID       string                `json:"message_id"`
	Seq             int64                 `json:"seq"`
	SenderID        *int64                `json:"sender_id"`
	Type            model.ChatMessageType `json:"type"`
	TextPreview     *string               `json:"text_preview"`
	DurationSeconds *float64              `json:"duration_seconds"`
	CreatedAt       time.Time             `json:"created_at"`
	IsReadByPeer    *bool                 `json:"is_read_by_peer"`
}

type ChatCardResponse struct {
	ChatID         string                   `json:"chat_id"`
	MatchID        int64                    `json:"match_id"`
	Peer           ChatPeerResponse         `json:"peer"`
	IsNew          bool                     `json:"is_new"`
	UnreadCount    int64                    `json:"unread_count"`
	LastMessage    *ChatLastMessageResponse `json:"last_message"`
	CreatedAt      time.Time                `json:"created_at"`
	LastActivityAt time.Time                `json:"last_activity_at"`
	Version        int64                    `json:"version"`
}

type ChatSummaryResponse struct {
	NewChatsCount       int64 `json:"new_chats_count"`
	UnreadChatsCount    int64 `json:"unread_chats_count"`
	UnreadMessagesCount int64 `json:"unread_messages_count"`
	Version             int64 `json:"version"`
}

type MessageCreatedData struct {
	ChatID          string              `json:"chat_id"`
	ClientMessageID *string             `json:"client_message_id"`
	Message         ChatMessageResponse `json:"message"`
}

type MessageChangedData struct {
	ChatID  string              `json:"chat_id"`
	Message ChatMessageResponse `json:"message"`
}

func NewChatEvent(event *model.ChatEvent) (ChatEvent, error) {

	if event == nil {
		return ChatEvent{}, fmt.Errorf("missing chat event")
	}

	result := ChatEvent{EventID: event.EventID}

	switch payload := event.Payload.(type) {

	case model.ChatReadUpdated:
		result.Type = "chat.read.updated"
		result.Data = ChatReadUpdated{ChatID: strconv.FormatInt(payload.ChatID, 10), UserID: payload.UserID, LastReadSeq: payload.LastReadSeq}

	case model.ChatCreated:
		result.Type = "chat.created"
		result.Data = NewChatCardResponse(payload.Chat)

	case model.ChatUpdated:
		result.Type = "chat.updated"
		result.Data = NewChatCardResponse(payload.Chat)

	case model.ChatsSummaryUpdated:
		result.Type = "chats.summary.updated"
		result.Data = ChatSummaryResponse{
			NewChatsCount:       payload.Summary.NewChatsCount,
			UnreadChatsCount:    payload.Summary.UnreadChatsCount,
			UnreadMessagesCount: payload.Summary.UnreadMessagesCount,
			Version:             payload.Summary.Version,
		}

	case model.MessageCreated:
		result.Type = "message.created"
		result.Data = MessageCreatedData{ChatID: strconv.FormatInt(payload.ChatID, 10), ClientMessageID: payload.ClientMessageID, Message: NewChatMessageResponse(payload.Message)}

	case model.MessageUpdated:
		result.Type = "message.updated"
		result.Data = MessageChangedData{ChatID: strconv.FormatInt(payload.ChatID, 10), Message: NewChatMessageResponse(payload.Message)}

	case model.MessageDeleted:
		result.Type = "message.deleted"
		result.Data = MessageChangedData{ChatID: strconv.FormatInt(payload.ChatID, 10), Message: NewChatMessageResponse(payload.Message)}

	default:
		return ChatEvent{}, fmt.Errorf("unsupported chat event payload: %T", event.Payload)
	}

	return result, nil
}

func NewChatMessageResponse(message model.ChatMessage) ChatMessageResponse {

	result := ChatMessageResponse{
		MessageID: strconv.FormatInt(message.MessageID, 10),
		Seq:       message.Seq,
		SenderID:  message.SenderID,
		Type:      message.Type,
		Text:      message.Text,
		CreatedAt: message.CreatedAt,
		EditedAt:  message.EditedAt,
		DeletedAt: message.DeletedAt,
		Version:   message.Version,
	}

	if message.Attachment != nil {
		result.Attachment = &ChatAttachmentResponse{URL: message.Attachment.URL, DurationSeconds: message.Attachment.DurationSeconds}
	}

	return result
}

func NewChatCardResponse(card model.ChatCard) ChatCardResponse {

	result := ChatCardResponse{
		ChatID:  strconv.FormatInt(card.ChatID, 10),
		MatchID: card.MatchID,
		Peer: ChatPeerResponse{
			UserID:    card.Peer.UserID,
			Name:      card.Peer.Name,
			Age:       card.Peer.Age,
			AvatarURL: card.Peer.AvatarURL},

		IsNew:          card.IsNew,
		UnreadCount:    card.UnreadCount,
		CreatedAt:      card.CreatedAt,
		LastActivityAt: card.LastActivityAt,
		Version:        card.Version,
	}

	if message := card.LastMessage; message != nil {
		result.LastMessage = &ChatLastMessageResponse{
			MessageID:       strconv.FormatInt(message.MessageID, 10),
			Seq:             message.Seq,
			SenderID:        message.SenderID,
			Type:            message.Type,
			TextPreview:     message.TextPreview,
			DurationSeconds: message.DurationSeconds,
			CreatedAt:       message.CreatedAt,
			IsReadByPeer:    message.IsReadByPeer,
		}
	}

	return result
}

type ChatCommand struct {
	Type      string          `json:"type"`
	RequestID string          `json:"request_id"`
	Data      json.RawMessage `json:"data"`
}

type SendMessageCommand struct {
	ChatID          string `json:"chat_id"`
	ClientMessageID string `json:"client_message_id"`
	Text            string `json:"text"`
}

type EditMessageCommand struct {
	ChatID    string `json:"chat_id"`
	MessageID string `json:"message_id"`
	Text      string `json:"text"`
}

type DeleteMessageCommand struct {
	ChatID    string `json:"chat_id"`
	MessageID string `json:"message_id"`
}

type ReadChatCommand struct {
	ChatID      string `json:"chat_id"`
	LastReadSeq *int64 `json:"last_read_seq"`
}

func ParseChatCommandData(command ChatCommand) (any, error) {

	raw := bytes.TrimSpace(command.Data)

	if len(raw) == 0 || raw[0] != '{' {
		return nil, fmt.Errorf("command data must be an object")
	}

	switch command.Type {

	case "message.send":
		var data SendMessageCommand

		if err := json.Unmarshal(raw, &data); err != nil {
			return nil, fmt.Errorf("invalid message.send data: %w", err)
		}

		chatID, err := parseChatCommandID(data.ChatID, "chat_id")

		if err != nil {
			return nil, err
		}

		if strings.TrimSpace(data.Text) == "" {
			return nil, fmt.Errorf("text must not be empty")
		}

		return model.ChatSendMessage{ChatID: chatID, ClientMessageID: data.ClientMessageID, Text: data.Text}, nil

	case "message.edit":
		var data EditMessageCommand

		if err := json.Unmarshal(raw, &data); err != nil {
			return nil, fmt.Errorf("invalid message.edit data: %w", err)
		}

		chatID, err := parseChatCommandID(data.ChatID, "chat_id")
		if err != nil {
			return nil, err
		}

		messageID, err := parseChatCommandID(data.MessageID, "message_id")
		if err != nil {
			return nil, err
		}

		if strings.TrimSpace(data.Text) == "" {
			return nil, fmt.Errorf("text must not be empty")
		}

		return model.ChatEditMessage{ChatID: chatID, MessageID: messageID, Text: data.Text}, nil

	case "message.delete":
		var data DeleteMessageCommand

		if err := json.Unmarshal(raw, &data); err != nil {
			return nil, fmt.Errorf("invalid message.delete data: %w", err)
		}

		chatID, err := parseChatCommandID(data.ChatID, "chat_id")

		if err != nil {
			return nil, err
		}

		messageID, err := parseChatCommandID(data.MessageID, "message_id")

		if err != nil {
			return nil, err
		}

		return model.ChatDeleteMessage{ChatID: chatID, MessageID: messageID}, nil

	case "chat.read":
		var data ReadChatCommand

		if err := json.Unmarshal(raw, &data); err != nil {
			return nil, fmt.Errorf("invalid chat.read data: %w", err)
		}

		chatID, err := parseChatCommandID(data.ChatID, "chat_id")

		if err != nil {
			return nil, err
		}

		if data.LastReadSeq == nil || *data.LastReadSeq < 0 {
			return nil, fmt.Errorf("last_read_seq is required and must not be negative")
		}

		return model.ChatRead{ChatID: chatID, LastReadSeq: *data.LastReadSeq}, nil

	default:
		return nil, fmt.Errorf("unsupported chat command: %q", command.Type)
	}
}

func parseChatCommandID(value, field string) (int64, error) {

	id, err := strconv.ParseInt(value, 10, 64)

	if err != nil {
		return 0, fmt.Errorf("invalid %s: %w", field, err)
	}

	if id <= 0 {
		return 0, fmt.Errorf("%s must be positive", field)
	}

	return id, nil
}

type ChatCommandError struct {
	Type      string    `json:"type"`
	RequestID *string   `json:"request_id"`
	Error     ErrorBody `json:"error"`
}

func NewChatCommandError(requestID *string, code, message string) ChatCommandError {

	return ChatCommandError{
		Type:      "command.error",
		RequestID: requestID,
		Error: ErrorBody{
			Code:    code,
			Message: message,
		},
	}
}

type ChatCommandResult struct {
	Type      string `json:"type"`
	RequestID string `json:"request_id"`
	Data      any    `json:"data"`
}

func NewChatCommandResult(requestID string, data any) ChatCommandResult {

	return ChatCommandResult{
		Type:      "command.result",
		RequestID: requestID,
		Data:      data,
	}
}
