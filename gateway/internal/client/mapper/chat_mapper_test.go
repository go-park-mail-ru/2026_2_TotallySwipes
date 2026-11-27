package mapper

import (
	"testing"

	"dating-app/gateway/internal/model"
	chatpb "dating-app/services/chat/api/gen/go/chat/v1"
)

func TestChatMessagesQueryToProto(t *testing.T) {

	zero, before, limit := int64(0), int64(10), 25
	request, err := ChatMessagesQueryToProto(7, 42, model.ChatMessagesQuery{Limit: &limit, AfterSeq: &zero})

	if err != nil {
		t.Fatal(err)
	}

	cursor, ok := request.Cursor.(*chatpb.GetMessagesRequest_AfterSeq)

	if !ok || cursor.AfterSeq != 0 || request.GetLimit() != 25 || request.GetChatId() != "42" {
		t.Fatalf("unexpected request: %v", request)
	}

	request, err = ChatMessagesQueryToProto(7, 42, model.ChatMessagesQuery{})

	if err != nil || request.Cursor != nil || request.Limit != nil {
		t.Fatalf("optional fields were not preserved: %v, %v", request, err)
	}

	for _, query := range []model.ChatMessagesQuery{
		{BeforeSeq: &before, AfterSeq: &zero},
		{BeforeSeq: &zero},
	} {

		if _, err := ChatMessagesQueryToProto(7, 42, query); err == nil {
			t.Fatal("expected invalid cursor error")
		}
	}
}

func TestChatMessagesPageFromProto(t *testing.T) {

	page, err := ChatMessagesPageFromProto(&chatpb.ChatMessagesPage{Direction: chatpb.ChatPageDirection_CHAT_PAGE_DIRECTION_NEWER})

	if err != nil || page.Items == nil || page.NextCursor != nil || page.Direction != model.ChatPageDirectionNewer {
		t.Fatalf("unexpected empty page: %+v, %v", page, err)
	}

	if _, err := ChatMessagesPageFromProto(&chatpb.ChatMessagesPage{}); err == nil {
		t.Fatal("expected unknown direction error")
	}

	if _, err := ChatDetailsFromProto(nil); err == nil {
		t.Fatal("expected missing details error")
	}
}
