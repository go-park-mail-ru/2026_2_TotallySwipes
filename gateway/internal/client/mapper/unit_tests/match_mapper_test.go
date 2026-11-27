package mapper_test

import (
	"strings"
	"testing"
	"time"

	"dating-app/gateway/internal/client/mapper"
	matchpb "dating-app/services/matching/api/gen/go/match/v1"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestMatchesRejectInvalidChatID(t *testing.T) {

	for _, chatID := range []string{"", "invalid", "0", "-1", "9223372036854775808"} {
		t.Run(chatID, func(t *testing.T) {

			response := &matchpb.GetMatchesResponse{
				Items: []*matchpb.MatchItem{{
					MatchId:   20,
					ChatId:    &chatID,
					CreatedAt: timestamppb.New(time.Now()),
				}},
			}

			page, err := mapper.MatchPageFromProto(response)

			if page != nil || err == nil || !strings.HasPrefix(err.Error(), "proto:") || !strings.Contains(err.Error(), "match.chat_id") {
				t.Fatalf("page = %v, error = %v; want proto chat ID error", page, err)
			}
		})
	}
}
