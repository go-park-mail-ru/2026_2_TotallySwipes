package dto

import (
	"net/url"
	"strconv"

	"dating-app/gateway/internal/model"
)

type ChatMessagingResponse struct {
	CanSend     bool    `json:"can_send"`
	Restriction *string `json:"restriction"`
}

type ChatDetailsResponse struct {
	Chat            ChatCardResponse      `json:"chat"`
	IsEmpty         bool                  `json:"is_empty"`
	CommonTags      []string              `json:"common_tags"`
	LastReadSeq     int64                 `json:"last_read_seq"`
	PeerLastReadSeq int64                 `json:"peer_last_read_seq"`
	Messaging       ChatMessagingResponse `json:"messaging"`
}

type ChatPageResponse struct {
	Items      []ChatCardResponse `json:"items"`
	NextCursor *string            `json:"next_cursor"`
}

func NewChatPageResponse(page model.ChatPage) ChatPageResponse {

	result := ChatPageResponse{
		Items:      make([]ChatCardResponse, 0, len(page.Items)),
		NextCursor: page.NextCursor,
	}

	for _, card := range page.Items {
		result.Items = append(result.Items, NewChatCardResponse(card))
	}

	return result
}

type ChatMessagesPageResponse struct {
	Items      []ChatMessageResponse   `json:"items"`
	Direction  model.ChatPageDirection `json:"direction"`
	NextCursor *int64                  `json:"next_cursor"`
	HasMore    bool                    `json:"has_more"`
}

func NewChatSummaryResponse(summary model.ChatSummary) ChatSummaryResponse {

	return ChatSummaryResponse{NewChatsCount: summary.NewChatsCount, UnreadChatsCount: summary.UnreadChatsCount, UnreadMessagesCount: summary.UnreadMessagesCount, Version: summary.Version}
}

func NewChatDetailsResponse(details model.ChatDetails) ChatDetailsResponse {

	return ChatDetailsResponse{
		Chat: NewChatCardResponse(details.Chat), IsEmpty: details.IsEmpty,
		CommonTags: append([]string{}, details.CommonTags...), LastReadSeq: details.LastReadSeq,
		PeerLastReadSeq: details.PeerLastReadSeq,
		Messaging:       ChatMessagingResponse{CanSend: details.Messaging.CanSend, Restriction: details.Messaging.Restriction},
	}
}

func NewChatMessagesPageResponse(page model.ChatMessagesPage) ChatMessagesPageResponse {

	result := ChatMessagesPageResponse{Items: make([]ChatMessageResponse, 0, len(page.Items)), Direction: page.Direction, NextCursor: page.NextCursor, HasMore: page.HasMore}

	for _, message := range page.Items {
		result.Items = append(result.Items, NewChatMessageResponse(message))
	}

	return result
}

func ParseChatMessagesQuery(values url.Values) (model.ChatMessagesQuery, map[string]string) {

	errs := make(map[string]string)
	limit := 50
	result := model.ChatMessagesQuery{Limit: &limit}

	if entries, exists := values["limit"]; exists {

		parsed, err := strconv.Atoi(values.Get("limit"))

		if len(entries) != 1 || err != nil || parsed < 1 || parsed > 100 {
			errs["limit"] = "должен быть целым числом от 1 до 100"
		} else {
			limit = parsed
		}
	}

	for _, field := range []string{"before_seq", "after_seq"} {

		if entries, exists := values[field]; exists {

			seq, err := strconv.ParseInt(values.Get(field), 10, 64)

			if len(entries) != 1 || err != nil || seq < 0 || field == "before_seq" && seq == 0 {
				errs[field] = "некорректная позиция сообщения"
				continue
			}

			if field == "before_seq" {
				result.BeforeSeq = &seq
			} else {
				result.AfterSeq = &seq
			}
		}
	}

	if values.Has("before_seq") && values.Has("after_seq") {
		errs["cursor"] = "before_seq и after_seq нельзя передавать одновременно"
	}

	return result, errs
}

type ChatListQueryRequest struct {
	Limit  *int    `json:"limit"`
	Cursor *string `json:"cursor"`
}

func ParseChatListQuery(values url.Values) (model.ChatListQuery, map[string]string) {

	var request ChatListQueryRequest
	errs := make(map[string]string)

	if entries, exists := values["limit"]; exists {

		if len(entries) != 1 {
			errs["limit"] = "параметр нельзя передавать несколько раз"
		} else if entries[0] != "" {
			limit, err := strconv.Atoi(entries[0])

			if err != nil || limit < 1 || limit > 100 {
				errs["limit"] = "должен быть целым числом от 1 до 100"
			} else {
				request.Limit = &limit
			}
		}
	}

	if entries, exists := values["cursor"]; exists {

		if len(entries) != 1 {
			errs["cursor"] = "параметр нельзя передавать несколько раз"
		} else if entries[0] != "" {
			offset, err := strconv.ParseInt(entries[0], 10, 64)

			if err != nil || offset < 0 {
				errs["cursor"] = "должен быть неотрицательным целым числом в виде строки"
			} else {
				cursor := entries[0]
				request.Cursor = &cursor
			}
		}
	}

	return model.ChatListQuery{Limit: request.Limit, Cursor: request.Cursor}, errs
}
