package handler

import (
	"dating-app/internal/service"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
)

func Write(w http.ResponseWriter, status int, data any) {
	body, err := json.Marshal(data)
	if err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if _, err := w.Write(body); err != nil {
		slog.Error("write response", "error", err)
	}
}

func WriteError(w http.ResponseWriter, status int, code, message string) {
	Write(w, status, map[string]any{
		"error": map[string]string{
			"code":    code,
			"message": message,
		},
	})
}

type FeedHandler struct {
	profileSvc service.ProfileService
}

func (h *FeedHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {

	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		WriteError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Метод не поддерживается")
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

	//Загулшка userID
	var userID int64 = 1

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

	Write(w, http.StatusOK, page)
}
