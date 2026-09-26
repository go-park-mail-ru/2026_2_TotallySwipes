package model

type FeedPage struct {
	Items      []FeedItem
	NextCursor *int64
}

type FeedItem struct {
	UserID  int64
	Name    string
	Age     int
	AboutMe *string
	Tags    []string
	Photos  []FeedPhoto
}

type FeedPhoto struct {
	ID  int64
	URL string
}
