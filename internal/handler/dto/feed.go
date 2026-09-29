package dto

type FeedResponse struct {
	Items      []FeedItem `json:"items"`
	NextCursor *int64     `json:"next_cursor"`
}

type FeedItem struct {
	UserID        int64       `json:"user_id"`
	Name          string      `json:"name"`
	Age           int         `json:"age"`
	DatingIntent  string      `json:"dating_intent"`
	Compatibility *float64    `json:"compatibility"`
	AboutMe       *string     `json:"about_me"`
	Tags          []string    `json:"tags"`
	Photos        []FeedPhoto `json:"photos"`
}

type FeedPhoto struct {
	ID  int64  `json:"id"`
	URL string `json:"url"`
}
