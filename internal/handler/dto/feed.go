package dto

import (
	"dating-app/internal/model"
	"fmt"
	"net/url"
	"strconv"
)

const (
	feedDefaultLimit = 10
	feedMaxLimit     = 10
)

type PageQuery struct {
	Limit  int
	Cursor *int64
}

type FeedQuery = PageQuery

// ParseFeedQuery разбирает необязательные limit и cursor; пустая map - запрос корректен
func ParseFeedQuery(q url.Values) (FeedQuery, map[string]string) {
	return parsePageQuery(q, feedDefaultLimit, feedMaxLimit)
}

func parsePageQuery(q url.Values, defaultLimit, maxLimit int) (PageQuery, map[string]string) {
	errs := make(map[string]string)
	query := PageQuery{Limit: defaultLimit}

	if v, ok := singleParam(q, "limit", errs); ok {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 || n > maxLimit {
			errs["limit"] = fmt.Sprintf("limit должен быть целым числом от 1 до %d", maxLimit)
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
	City          *string          `json:"city"`
	UserID        int64            `json:"user_id"`
	Name          string           `json:"name"`
	Age           int              `json:"age"`
	DatingGoal    string           `json:"dating_goal"`
	Compatibility *float64         `json:"compatibility"`
	AboutMe       *string          `json:"about_me"`
	Education     *model.Education `json:"education"`
	Work          *string          `json:"work"`
	Smoking       *model.Attitude  `json:"smoking"`
	Alcohol       *model.Attitude  `json:"alcohol"`
	Height        *int             `json:"height"`
	Tags          []string         `json:"tags"`
	Photos        []FeedPhoto      `json:"photos"`
	LikedMe       bool             `json:"liked_me"`
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
		resp.Items = append(resp.Items, newFeedItem(item))
	}
	return resp
}

func newFeedItem(item model.FeedItem) FeedItem {
	photos := make([]FeedPhoto, 0, len(item.Photos))
	for _, photo := range item.Photos {
		photos = append(photos, FeedPhoto{ID: photo.ID, URL: photo.URL})
	}
	return FeedItem{
		City:          item.City,
		UserID:        item.UserID,
		Name:          item.Name,
		Age:           item.Age,
		DatingGoal:    string(item.DatingGoal),
		Compatibility: item.Compatibility,
		AboutMe:       item.AboutMe,
		Education:     item.Education,
		Work:          item.Work,
		Smoking:       item.Smoking,
		Alcohol:       item.Alcohol,
		Height:        item.Height,
		Tags:          item.Tags,
		Photos:        photos,
		LikedMe:       item.LikedMe,
	}
}

// MatchedProfileResponse - карточка анкеты в формате ленты и id мэтча для перехода к переписке
type MatchedProfileResponse struct {
	MatchID int64 `json:"match_id"`
	FeedItem
}

func NewMatchedProfileResponse(p *model.MatchedProfile) MatchedProfileResponse {
	return MatchedProfileResponse{MatchID: p.MatchID, FeedItem: newFeedItem(p.Item)}
}
