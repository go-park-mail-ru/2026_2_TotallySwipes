package model

type FeedPage struct {
	Items      []FeedItem
	NextCursor *int64
}

type FeedItem struct {
	UserID        int64
	Name          string
	Age           int
	DatingGoal    DatingGoal
	Compatibility *float64
	AboutMe       *string
	Education     *Education
	Work          *string
	Smoking       *Attitude
	Alcohol       *Attitude
	Height        *int
	Tags          []string
	Photos        []FeedPhoto
}

type FeedPhoto struct {
	ID  int64
	URL string
}
