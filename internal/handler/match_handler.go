package handler

import (
	"context"
	"dating-app/internal/handler/dto"
	"dating-app/internal/model"
	"net/http"
)

type MatchService interface {
	GetMatches(ctx context.Context, userID int64, limit int, cursor *int64) (*model.MatchPage, error)
}

type MatchHandler struct {
	svc MatchService
}

func NewMatchHandler(svc MatchService) *MatchHandler {
	return &MatchHandler{svc: svc}
}

// List - GET /matches: страница мэтчей текущего пользователя до cursor
func (h *MatchHandler) List(w http.ResponseWriter, r *http.Request, userID int64) {
	query, errs := dto.ParseMatchesQuery(r.URL.Query())
	if len(errs) > 0 {
		writeError(w, http.StatusBadRequest, codeValidationError, msgInvalidMatchesQuery, errs)
		return
	}

	page, err := h.svc.GetMatches(r.Context(), userID, query.Limit, query.Cursor)
	if err != nil {
		writeServiceError(w, "get matches", err)
		return
	}
	writeJSON(w, http.StatusOK, dto.NewMatchesResponse(page))
}
