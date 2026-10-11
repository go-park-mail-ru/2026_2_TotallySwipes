package dto

type SwipeActionRequest struct {
	TargetID int64  `json:"target_user_id"`
	Action   string `json:"action"`
}

func (r SwipeActionRequest) Validate() map[string]string {

	errs := make(map[string]string)

	if r.TargetID <= 0 {
		errs["target_user_id"] = "Идентификатор пользователя должен быть положительным"
	}

	switch r.Action {
	case "like", "dislike", "super_like":
	default:
		errs["action"] = "Допустимые действия: like, dislike, super_like"
	}

	return errs
}
