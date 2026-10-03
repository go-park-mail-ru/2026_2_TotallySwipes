package handler

import (
	"dating-app/internal/handler/dto"
	"dating-app/internal/middleware"
	"dating-app/internal/model"
	"dating-app/internal/service"
	"errors"
	"net/http"
)

type GetProfileShortHandler struct {
	profileSvc service.ProfileService
}

func NewGetProfileShortHandler(svc service.ProfileService) *GetProfileShortHandler {
	return &GetProfileShortHandler{profileSvc: svc}
}

func (h *GetProfileShortHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {

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

	short, err := h.profileSvc.GetShortProfile(r.Context(), userID)
	if errors.Is(err, model.ErrProfileRequired) {
		WriteError(w, http.StatusNotFound, "PROFILE_NOT_FOUND", "Анкета не найдена")
		return
	}
	if err != nil || short == nil {
		WriteError(w, http.StatusInternalServerError, codeInternalError, "Не удалось загрузить профиль")
		return
	}

	Write(w, http.StatusOK, dto.ProfileShortResponse{
		UserID:   short.UserID,
		Name:     short.Name,
		Age:      short.Age,
		PhotoURL: short.MainPhotoURL,
	})
}
