package handler

import (
	"dating-app/internal/handler/dto"
	"dating-app/internal/middleware"
	"dating-app/internal/model"
	"dating-app/internal/service"
	"errors"
	"net/http"
	"strconv"
)

type GetFeedHandler struct {
	profileSvc service.ProfileService
}

func NewGetFeedHandler(svc service.ProfileService) *GetFeedHandler {
	return &GetFeedHandler{profileSvc: svc}
}

func (h *GetFeedHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {

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
		case errors.Is(err, model.ErrInvalidFeedRequest):
			WriteError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Некорректные параметры ленты")
		case errors.Is(err, model.ErrProfileRequired):
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

	response := dto.FeedResponse{
		Items:      make([]dto.FeedItem, 0, len(page.Items)),
		NextCursor: page.NextCursor,
	}

	for _, item := range page.Items {
		photos := make([]dto.FeedPhoto, 0, len(item.Photos))
		for _, photo := range item.Photos {
			photos = append(photos, dto.FeedPhoto{ID: photo.ID, URL: photo.URL})
		}

		intent := string(item.DatingIntent)
		for label, goal := range model.DatingGoalByIntent {
			if goal == item.DatingIntent {
				intent = label
				break
			}
		}

		response.Items = append(response.Items, dto.FeedItem{
			UserID:        item.UserID,
			Name:          item.Name,
			Age:           item.Age,
			DatingIntent:  intent,
			Compatibility: item.Compatibility,
			AboutMe:       item.AboutMe,
			Tags:          item.Tags,
			Photos:        photos,
		})
	}

	Write(w, http.StatusOK, response)
}
