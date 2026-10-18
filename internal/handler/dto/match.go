package dto

import (
	"dating-app/internal/model"
	"net/url"
	"time"
)

// ParseMatchesQuery разбирает необязательные limit и cursor списка мэтчей
func ParseMatchesQuery(q url.Values) (PageQuery, map[string]string) {
	return parsePageQuery(q, model.MatchesDefaultLimit, model.MatchesMaxLimit)
}

type MatchesResponse struct {
	Items      []MatchItem `json:"items"`
	Total      int         `json:"total"`
	NextCursor *int64      `json:"next_cursor"`
}

type MatchItem struct {
	MatchID       int64     `json:"match_id"`
	UserID        int64     `json:"user_id"`
	Name          *string   `json:"name"`
	Age           *int      `json:"age"`
	PhotoURL      *string   `json:"photo_url"`
	Compatibility *float64  `json:"compatibility"`
	IsNew         bool      `json:"is_new"`
	CreatedAt     time.Time `json:"created_at"`
}

func NewMatchesResponse(page *model.MatchPage) MatchesResponse {
	resp := MatchesResponse{
		Items:      make([]MatchItem, 0, len(page.Items)),
		Total:      page.Total,
		NextCursor: page.NextCursor,
	}
	for _, m := range page.Items {
		resp.Items = append(resp.Items, MatchItem{
			MatchID:       m.ID,
			UserID:        m.UserID,
			Name:          m.Name,
			Age:           m.Age,
			PhotoURL:      m.MainPhotoURL,
			Compatibility: m.Compatibility,
			IsNew:         m.IsNew,
			CreatedAt:     m.CreatedAt,
		})
	}
	return resp
}
