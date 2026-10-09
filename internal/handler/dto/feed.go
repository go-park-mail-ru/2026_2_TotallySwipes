package dto

import (
	"dating-app/internal/model"
	"net/url"
	"strconv"
)

const (
	feedDefaultLimit = 10
	feedMaxLimit     = 10
)

type FeedQuery struct {
	Limit  int
	Cursor *int64
}

// ParseFeedQuery разбирает необязательные limit и cursor; пустая map - запрос корректен
func ParseFeedQuery(q url.Values) (FeedQuery, map[string]string) {
	errs := make(map[string]string)
	query := FeedQuery{Limit: feedDefaultLimit}

	if v, ok := singleParam(q, "limit", errs); ok {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 || n > feedMaxLimit {
			errs["limit"] = "limit должен быть целым числом от 1 до 10"
		} else {
			query.Limit = n
		}
	}

	if v, ok := singleParam(q, "cursor", errs); ok {
		n, err := strconv.ParseInt(v, 10, 64)
		if err != nil || n < 1 {
			errs["cursor"] = "cursor должен быть положительным целым числом"
		} else {
			query.Cursor = &n
		}
	}

	return query, errs
}

func singleParam(q url.Values, key string, errs map[string]string) (string, bool) {
	values, exists := q[key]
	if !exists {
		return "", false
	}
	if len(values) != 1 {
		errs[key] = "параметр " + key + " должен быть указан один раз"
		return "", false
	}
	return values[0], true
}

type FeedResponse struct {
	Items      []FeedItem `json:"items"`
	NextCursor *int64     `json:"next_cursor"`
}

type FeedItem struct {
	UserID        int64       `json:"user_id"`
	Name          string      `json:"name"`
	Age           int         `json:"age"`
	DatingGoal    string      `json:"dating_goal"`
	Compatibility *float64    `json:"compatibility"`
	AboutMe       *string     `json:"about_me"`
	Tags          []string    `json:"tags"`
	Photos        []FeedPhoto `json:"photos"`
}

type FeedPhoto struct {
	ID  int64  `json:"id"`
	URL string `json:"url"`
}

func NewFeedResponse(page *model.FeedPage) FeedResponse {
	resp := FeedResponse{
		Items:      make([]FeedItem, 0, len(page.Items)),
		NextCursor: page.NextCursor,
	}
	for _, item := range page.Items {
		photos := make([]FeedPhoto, 0, len(item.Photos))
		for _, photo := range item.Photos {
			photos = append(photos, FeedPhoto{ID: photo.ID, URL: photo.URL})
		}
		resp.Items = append(resp.Items, FeedItem{
			UserID:        item.UserID,
			Name:          item.Name,
			Age:           item.Age,
			DatingGoal:    string(item.DatingGoal),
			Compatibility: item.Compatibility,
			AboutMe:       item.AboutMe,
			Tags:          item.Tags,
			Photos:        photos,
		})
	}
	return resp
}
