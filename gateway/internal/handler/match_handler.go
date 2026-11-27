package handler

import (
	"context"
	"dating-app/gateway/internal/handler/dto"
	"dating-app/gateway/internal/model"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
)

type MatchService interface {
	GetMatchedProfile(ctx context.Context, viewerID, targetID int64) (*model.MatchedProfile, error)
	GetMatches(ctx context.Context, userID int64, limit int, cursor *int64) (*model.MatchPage, error)
	MarkSeen(ctx context.Context, userID, matchID int64) error
	GetSummary(ctx context.Context, userID int64) (*model.MatchSummary, error)
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
		writeError(r.Context(), w, http.StatusBadRequest, codeValidationError, msgInvalidMatchesQuery, errs)
		return
	}

	page, err := h.svc.GetMatches(r.Context(), userID, query.Limit, query.Cursor)
	if err != nil {
		writeServiceError(r.Context(), w, "get matches", err)
		return
	}
	writeJSON(r.Context(), w, http.StatusOK, dto.NewMatchesResponse(page))
}

// MarkSeen - POST /matches/{match_id}/seen: мэтч перестаёт быть новым для текущего пользователя
func (h *MatchHandler) MarkSeen(w http.ResponseWriter, r *http.Request, userID int64) {
	matchID, err := strconv.ParseInt(mux.Vars(r)["match_id"], 10, 64)
	if err != nil || matchID <= 0 {
		writeError(r.Context(), w, http.StatusBadRequest, codeValidationError, "Некорректный match_id", nil)
		return
	}

	if err := h.svc.MarkSeen(r.Context(), userID, matchID); err != nil {
		writeServiceError(r.Context(), w, "mark match seen", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// Summary - GET /matches/summary: число мэтчей и новых для счётчика в навигации
func (h *MatchHandler) Summary(w http.ResponseWriter, r *http.Request, userID int64) {
	summary, err := h.svc.GetSummary(r.Context(), userID)
	if err != nil {
		writeServiceError(r.Context(), w, "get matches summary", err)
		return
	}
	writeJSON(r.Context(), w, http.StatusOK, dto.NewMatchesSummaryResponse(summary))
}

// GetMatchedProfile - GET /profiles/{user_id}: анкета собеседника, доступна только при мэтче
func (h *MatchHandler) GetMatchedProfile(w http.ResponseWriter, r *http.Request, userID int64) {
	targetID, err := strconv.ParseInt(mux.Vars(r)["user_id"], 10, 64)
	if err != nil || targetID <= 0 {
		writeError(r.Context(), w, http.StatusBadRequest, codeValidationError, "Некорректный user_id", nil)
		return
	}

	profile, err := h.svc.GetMatchedProfile(r.Context(), userID, targetID)
	if err != nil {
		writeServiceError(r.Context(), w, "get matched profile", err)
		return
	}
	writeJSON(r.Context(), w, http.StatusOK, dto.NewMatchedProfileResponse(profile))
}
