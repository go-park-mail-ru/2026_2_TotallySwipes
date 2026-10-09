package handler

import (
	"context"
	"dating-app/internal/handler/dto"
	"dating-app/internal/model"
	"net/http"
)

type FeedService interface {
	GetNextFeed(ctx context.Context, userID int64, limit int, cursor *int64) (*model.FeedPage, error)
}

type FeedHandler struct {
	svc FeedService
}

func NewFeedHandler(svc FeedService) *FeedHandler {
	return &FeedHandler{svc: svc}
}

// Get - GET /feed: страница анкет после cursor
func (h *FeedHandler) Get(w http.ResponseWriter, r *http.Request, userID int64) {
	query, errs := dto.ParseFeedQuery(r.URL.Query())
	if len(errs) > 0 {
		writeError(w, http.StatusBadRequest, codeValidationError, "Некорректные параметры ленты", errs)
		return
	}

	page, err := h.svc.GetNextFeed(r.Context(), userID, query.Limit, query.Cursor)
	if err != nil {
		writeServiceError(w, "get feed", err)
		return
	}
	writeJSON(w, http.StatusOK, dto.NewFeedResponse(page))
}
