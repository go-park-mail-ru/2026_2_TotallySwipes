package mapper_test

import (
	"encoding/json"
	"math"
	"testing"
	"time"

	"dating-app/gateway/internal/client/mapper"
	"dating-app/gateway/internal/handler/dto"
	matchpb "dating-app/services/matching/api/gen/go/match/v1"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestMatchesChatIDMapping(t *testing.T) {

	chatID := "9223372036854775807"
	cursor := int64(20)
	now := timestamppb.New(time.Date(2026, 11, 27, 12, 0, 0, 0, time.UTC))
	response := &matchpb.GetMatchesResponse{
		Items: []*matchpb.MatchItem{
			{MatchId: 30, ChatId: &chatID, UserId: 2, CreatedAt: now},
			{MatchId: 20, UserId: 3, CreatedAt: now},
		},
		Total:      3,
		NextCursor: &cursor,
	}

	page, err := mapper.MatchPageFromProto(response)

	if err != nil {
		t.Fatal(err)
	}

	if len(page.Items) != 2 || page.Items[0].ChatID == nil || *page.Items[0].ChatID != math.MaxInt64 || page.Items[1].ChatID != nil {
		t.Fatalf("chat IDs were not preserved: %#v", page.Items)
	}

	payload, err := json.Marshal(dto.NewMatchesResponse(page))

	if err != nil {
		t.Fatal(err)
	}

	var result struct {
		Items      []map[string]json.RawMessage `json:"items"`
		Total      int                          `json:"total"`
		NextCursor *int64                       `json:"next_cursor"`
	}

	if err := json.Unmarshal(payload, &result); err != nil {
		t.Fatal(err)
	}

	// Строка сохраняет точный ID, превышающий безопасный диапазон чисел JavaScript.
	if string(result.Items[0]["chat_id"]) != `"9223372036854775807"` || string(result.Items[1]["chat_id"]) != "null" {
		t.Fatalf("unexpected HTTP chat IDs: %s", payload)
	}

	if result.Total != 3 || result.NextCursor == nil || *result.NextCursor != 20 {
		t.Fatalf("pagination was not preserved: %s", payload)
	}
}
