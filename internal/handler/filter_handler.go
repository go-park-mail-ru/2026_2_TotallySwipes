package handler

import (
	"context"
	"dating-app/internal/handler/dto"
	"dating-app/internal/model"
	"net/http"
)

type FilterService interface {
	Get(ctx context.Context, userID int64) (*model.SearchFilter, error)
	Set(ctx context.Context, userID int64, f model.SearchFilter) (*model.SearchFilter, error)
}

type FilterHandler struct {
	svc FilterService
}

func NewFilterHandler(svc FilterService) *FilterHandler {
	return &FilterHandler{svc: svc}
}

// Get - GET /filters/me: фильтр ленты; если не задан - значения по умолчанию
func (h *FilterHandler) Get(w http.ResponseWriter, r *http.Request, userID int64) {
	f, err := h.svc.Get(r.Context(), userID)
	if err != nil {
		writeServiceError(w, "get filter", err)
		return
	}
	writeJSON(w, http.StatusOK, dto.NewFilterResponse(f))
}

// Set - PUT /filters/me: заменяет фильтр ленты целиком
func (h *FilterHandler) Set(w http.ResponseWriter, r *http.Request, userID int64) {
	var req dto.SetFilterRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	filter, errs := req.ToModel()
	if len(errs) > 0 {
		writeError(w, http.StatusBadRequest, codeValidationError, msgInvalidData, errs)
		return
	}

	f, err := h.svc.Set(r.Context(), userID, filter)
	if err != nil {
		writeServiceError(w, "set filter", err)
		return
	}
	writeJSON(w, http.StatusOK, dto.NewFilterResponse(f))
}
