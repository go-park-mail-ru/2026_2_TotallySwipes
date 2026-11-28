package handler

import (
	"net/url"
	"testing"

	"dating-app/gateway/internal/handler/dto"
)

func TestParseChatListQuery(t *testing.T) {

	for _, raw := range []string{"", "limit=&cursor="} {
		values, _ := url.ParseQuery(raw)
		query, errs := dto.ParseChatListQuery(values)
		if len(errs) != 0 || query.Limit != nil || query.Cursor != nil {
			t.Fatalf("query=%+v errors=%v", query, errs)
		}
	}

	query, errs := dto.ParseChatListQuery(url.Values{"limit": {"20"}, "cursor": {"100"}})
	if len(errs) != 0 || query.Limit == nil || *query.Limit != 20 || query.Cursor == nil || *query.Cursor != "100" {
		t.Fatalf("query=%+v errors=%v", query, errs)
	}

	for _, values := range []url.Values{
		{"limit": {"0"}}, {"limit": {"101"}}, {"limit": {"null"}}, {"limit": {"1", "2"}},
		{"cursor": {"-1"}}, {"cursor": {"bad"}}, {"cursor": {"0", "1"}},
	} {
		if _, errs := dto.ParseChatListQuery(values); len(errs) == 0 {
			t.Fatalf("invalid query accepted: %v", values)
		}
	}
}
