package handler

import (
	"dating-app/internal/middleware"
	"dating-app/internal/model"
	"dating-app/internal/service"
	"errors"
	"net/http"
)

type GetMyTestResultHandler struct {
	testSvc service.TestService
}

func NewGetMyTestResultHandler(svc service.TestService) *GetMyTestResultHandler {
	return &GetMyTestResultHandler{testSvc: svc}
}

func (h *GetMyTestResultHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {

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

	result, err := h.testSvc.GetMyTestResult(r.Context(), userID)

	if err != nil {
		switch {
		case errors.Is(err, model.ErrTestResultNotFound):
			WriteError(w, http.StatusNotFound, "TEST_RESULT_NOT_FOUND", "Тест ещё не пройден")
		case errors.Is(err, model.ErrProfileRequired):
			WriteError(w, http.StatusConflict, "PROFILE_REQUIRED", "Для прохождения теста необходимо создать профиль")
		default:
			WriteError(w, http.StatusInternalServerError, "INTERNAL_SERVER_ERROR", "Не удалось получить результаты теста")
		}
		return
	}

	if result == nil {
		WriteError(w, http.StatusInternalServerError, "INTERNAL_SERVER_ERROR", "Не удалось получить результаты теста")
		return
	}

	Write(w, http.StatusOK, newTestResultResponse(result))
}
