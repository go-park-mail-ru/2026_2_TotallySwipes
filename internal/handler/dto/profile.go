package dto

type ProfileShortResponse struct {
	UserID   int64   `json:"user_id"`
	Name     string  `json:"name"`
	PhotoURL *string `json:"photo_url"`
}
