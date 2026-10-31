package handler

import (
	"context"
	"dating-app/internal/handler/dto"
	"dating-app/internal/model"
	"net/http"
	"strconv"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
)

type EventService interface {
	SearchEventsByCity(ctx context.Context, userID int64, matchID int64, city string) (*model.SearchEventsState, bool, error)
	GetSearchEvents(ctx context.Context, userID, matchID int64, searchID uuid.UUID) (*model.SearchEventsState, error)
	SubmitResults(ctx context.Context, userID, matchID int64, searchID uuid.UUID, likedEventsIDs []int64) (*model.SearchEventsState, error)
}

type EventHandler struct {
	svc EventService
}

func NewEventHandler(svc EventService) *EventHandler {
	return &EventHandler{svc: svc}
}

func (h *EventHandler) CreateSearch(w http.ResponseWriter, r *http.Request, userID int64) {
	matchID, err := strconv.ParseInt(mux.Vars(r)["match_id"], 10, 64)

	if err != nil || matchID <= 0 {
		writeError(r.Context(), w, http.StatusBadRequest, codeValidationError, "Некорректный match_id", nil)
		return
	}

	var req dto.CreateSearchRequest

	if !decodeJSON(w, r, &req) {
		return
	}

	state, created, err := h.svc.SearchEventsByCity(r.Context(), userID, matchID, req.City)

	if err != nil {
		writeServiceError(r.Context(), w, "search events by city", err)
		return
	}

	resp := dto.NewSearchStateResponse(state)
	if created {
		writeJSON(r.Context(), w, http.StatusCreated, resp)
		return
	}

	writeJSON(r.Context(), w, http.StatusOK, resp)
}

func (h *EventHandler) GetSearch(w http.ResponseWriter, r *http.Request, userID int64) {

	matchID, err := strconv.ParseInt(mux.Vars(r)["match_id"], 10, 64)
	if err != nil || matchID <= 0 {
		writeError(r.Context(), w, http.StatusBadRequest, codeValidationError, "Некорректный match_id", nil)
		return
	}

	searchID, err := uuid.Parse(mux.Vars(r)["search_id"])
	if err != nil {
		writeError(r.Context(), w, http.StatusBadRequest, codeValidationError, "Некорректный search_id", nil)
		return
	}

	state, err := h.svc.GetSearchEvents(r.Context(), userID, matchID, searchID)
	if err != nil {
		writeServiceError(r.Context(), w, "get search events", err)
		return
	}

	resp := dto.NewSearchStateResponse(state)

	writeJSON(r.Context(), w, http.StatusOK, resp)
}

func (h *EventHandler) Submit(w http.ResponseWriter, r *http.Request, userID int64) {
	matchID, err := strconv.ParseInt(mux.Vars(r)["match_id"], 10, 64)
	if err != nil || matchID <= 0 {
		writeError(r.Context(), w, http.StatusBadRequest, codeValidationError, "Некорректный match_id", nil)
		return
	}

	searchID, err := uuid.Parse(mux.Vars(r)["search_id"])
	if err != nil {
		writeError(r.Context(), w, http.StatusBadRequest, codeValidationError, "Некорректный search_id", nil)
		return
	}

	var req dto.SearchLikedEventsRequest
	if !decodeJSON(w, r, &req) {
		return
	}

	if req.LikedEventsIDs == nil {
		writeError(r.Context(), w, http.StatusBadRequest, codeValidationError, "Поле liked_event_ids обязательно", nil)
		return
	}

	state, err := h.svc.SubmitResults(r.Context(), userID, matchID, searchID, req.LikedEventsIDs)
	if err != nil {
		writeServiceError(r.Context(), w, "submit search results", err)
		return
	}

	resp := dto.NewSearchStateResponse(state)

	writeJSON(r.Context(), w, http.StatusOK, resp)
}
