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
