package model

type SwipeAction string

const (
	SwipeLike      SwipeAction = "like"
	SwipeDislike   SwipeAction = "dislike"
	SwipeSuperLike SwipeAction = "super_like"
)

type Swipe struct {
	TargetID int64
	AuthorID int64
	Action   SwipeAction
}
