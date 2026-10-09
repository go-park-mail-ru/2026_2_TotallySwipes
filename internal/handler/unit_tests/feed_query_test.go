package handler

import (
	"net/url"
	"reflect"
	"testing"

	"dating-app/internal/handler/dto"
)

func TestParseFeedQuery(t *testing.T) {
	five := int64(5)
	for _, tc := range []struct {
		name       string
		raw        string
		want       dto.FeedQuery
		wantFields []string
	}{
		{"по умолчанию", "", dto.FeedQuery{Limit: 10}, nil},
		{"limit и cursor", "limit=3&cursor=5", dto.FeedQuery{Limit: 3, Cursor: &five}, nil},
		{"limit вне диапазона", "limit=11", dto.FeedQuery{Limit: 10}, []string{"limit"}},
		{"повтор limit", "limit=1&limit=2", dto.FeedQuery{Limit: 10}, []string{"limit"}},
		{"кривой cursor", "cursor=0", dto.FeedQuery{Limit: 10}, []string{"cursor"}},
		{"обе ошибки", "limit=x&cursor=-1", dto.FeedQuery{Limit: 10}, []string{"cursor", "limit"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			q, _ := url.ParseQuery(tc.raw)
			got, errs := dto.ParseFeedQuery(q)
			if fields := keys(errs); !reflect.DeepEqual(fields, tc.wantFields) {
				t.Fatalf("fields = %v, want %v", fields, tc.wantFields)
			}
			if len(errs) == 0 && !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("query = %+v, want %+v", got, tc.want)
			}
		})
	}
}
