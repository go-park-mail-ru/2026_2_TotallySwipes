package dto

type ProfileShortResponse struct {
	UserID   int64   `json:"user_id"`
	Name     string  `json:"name"`
	Age      int     `json:"age"`
	PhotoURL *string `json:"photo_url"`
}
