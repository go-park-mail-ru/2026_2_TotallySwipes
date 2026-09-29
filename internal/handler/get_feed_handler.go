package handler

import (
	"dating-app/internal/middleware"
	"dating-app/internal/model"
	"dating-app/internal/service"
	"errors"
	"net/http"
	"strconv"
)

type FeedPage struct {
	Items      []FeedItem `json:"items"`
	NextCursor *int64     `json:"next_cursor"`
}

type FeedItem struct {
	UserID        int64             `json:"user_id"`
	Name          string            `json:"name"`
	Age           int               `json:"age"`
	DatingIntent  model.DatingGoal  `json:"dating_intent"`
	Compatibility *float64          `json:"compatibility"`
	AboutMe       *string           `json:"about_me"`
	Tags          []string          `json:"tags"`
	Photos        []model.FeedPhoto `json:"photos"`
}

type FeedPhoto struct {
	ID  int64  `json:"id"`
	URL string `json:"url"`
}

type FeedHandler struct {
	profileSvc service.ProfileService
}

func NewFeedHandler(svc service.ProfileService) *FeedHandler {
	return &FeedHandler{profileSvc: svc}
}

func (h *FeedHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {

	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		WriteError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Метод не поддерживается")
		return
	}

	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		WriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Необходима авторизация")
		return
	}
	query := r.URL.Query()

	limit := 10

	if values, exists := query["limit"]; exists {
		if len(values) != 1 {
			WriteError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Параметр limit должен быть указан один раз")
			return
		}

		value, err := strconv.Atoi(values[0])

		if err != nil || value < 1 || value > 10 {
			WriteError(w, http.StatusBadRequest,
				"VALIDATION_ERROR", "limit должен быть целым числом от 1 до 10")
			return
		}

		limit = value
	}

	var cursor *int64

	if values, exists := query["cursor"]; exists {
		if len(values) != 1 {
			WriteError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Параметр cursor должен быть указан один раз")
			return
		}

		value, err := strconv.ParseInt(values[0], 10, 64)

		if err != nil || value < 1 {
			WriteError(w, http.StatusBadRequest,
				"VALIDATION_ERROR", "cursor должен быть положительным целым числом")
			return
		}
		cursor = &value
	}

	page, err := h.profileSvc.GetNextFeed(r.Context(), userID, limit, cursor)

	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidFeedRequest):
			WriteError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Некорректные параметры ленты")
		case errors.Is(err, service.ErrProfileRequired):
			WriteError(w, http.StatusConflict, "PROFILE_REQUIRED", "Для просмотра ленты необходимо создать профиль")
		default:
			WriteError(w, http.StatusInternalServerError, "FEED_LOAD_FAILED", "Не удалось загрузить ленту")
		}
		return
	}

	if page == nil {
		WriteError(w, http.StatusInternalServerError, "FEED_LOAD_FAILED", "Не удалось загрузить ленту")
		return
	}

	response := FeedPage{
		Items:      make([]FeedItem, 0, len(page.Items)),
		NextCursor: page.NextCursor,
	}

	for _, item := range page.Items {
		response.Items = append(response.Items, FeedItem{
			UserID:        item.UserID,
			Name:          item.Name,
			Age:           item.Age,
			DatingIntent:  item.DatingIntent,
			Compatibility: item.Compatibility,
			AboutMe:       item.AboutMe,
			Tags:          item.Tags,
			Photos:        item.Photos,
		})
	}

	Write(w, http.StatusOK, response)
}
