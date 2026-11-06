package handler

import (
	"context"
	"dating-app/internal/handler/dto"
	"dating-app/internal/model"
	"net/http"
)

type LocationService interface {
	GetLocationPreview(ctx context.Context, userID int64, coordinates *model.Coordinates) (*model.UserLocationPreview, error)

	SaveLocation(ctx context.Context, userID int64, previewID string) (*model.UserLocation, error)
}

type LocationHandler struct {
	svc LocationService
}

func NewLocationHandler(svc LocationService) *LocationHandler {
	return &LocationHandler{svc: svc}
}

func (h *LocationHandler) Preview(w http.ResponseWriter, r *http.Request, userID int64) {
	var req dto.LocationPreviewRequest

	if !decodeJSON(w, r, &req) {
		return
	}

	coordinates, errs := req.ToModel()
	if len(errs) > 0 {
		writeError(r.Context(), w, http.StatusBadRequest, codeValidationError, msgInvalidData, errs)
		return
	}

	preview, err := h.svc.GetLocationPreview(r.Context(), userID, coordinates)

	if err != nil {
		writeServiceError(r.Context(), w, "preview location", err)
		return
	}

	resp := dto.NewLocationPreviewResponse(preview)

	writeJSON(r.Context(), w, http.StatusOK, resp)
}

func (h *LocationHandler) SaveLocation(w http.ResponseWriter, r *http.Request, userID int64) {
	var req dto.LocationConfirmRequest

	if !decodeJSON(w, r, &req) {
		return
	}

	errs := req.Validate()

	if len(errs) > 0 {
		writeError(r.Context(), w, http.StatusBadRequest, codeValidationError, msgInvalidData, errs)
		return
	}

	result, err := h.svc.SaveLocation(r.Context(), userID, req.PreviewID)

	if err != nil {
		writeServiceError(r.Context(), w, "save location", err)
		return
	}

	resp := dto.NewLocationResponse(result)

	writeJSON(r.Context(), w, http.StatusOK, resp)
}
