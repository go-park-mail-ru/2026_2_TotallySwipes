package handler

import (
	"context"
	"dating-app/internal/handler/dto"
	"dating-app/internal/model"
	"net/http"
)

type SwipeService interface {
	Swipe(ctx context.Context, swipe *model.Swipe) error
}

type SwipeHandler struct {
	svc SwipeService
}

func NewSwipeHandler(svc SwipeService) *SwipeHandler {
	return &SwipeHandler{svc: svc}
}

func (h *SwipeHandler) Swipe(w http.ResponseWriter, r *http.Request, userID int64) {

	var req dto.SwipeActionRequest
	if !decodeJSON(w, r, &req) {
		return
	}

	if errs := req.Validate(); len(errs) > 0 {
		writeError(w, http.StatusBadRequest, codeValidationError, msgInvalidData, errs)
		return
	}

	swipe := &model.Swipe{TargetID: req.TargetID, AuthorID: userID, Action: model.SwipeAction(req.Action)}

	err := h.svc.Swipe(r.Context(), swipe)

	if err != nil {
		writeServiceError(w, "swipe", err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
