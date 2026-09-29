package handler

import (
	"dating-app/internal/handler/dto"
	"dating-app/internal/model"
	"dating-app/internal/validate"
	"errors"
	"log/slog"
	"net/http"
)

func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	var req dto.RegisterRequest
	if !decodeJSON(w, r, &req) {
		return
	}

	req.Normalize()
	if errs := req.Validate(); len(errs) > 0 {
		writeError(w, http.StatusBadRequest, codeValidationError, "Некорректные данные", errs)
		return
	}

	// Ошибки тут быть не может - дату уже проверил Validate
	birthDate, _ := validate.ParseBirthDate(req.BirthDate)

	var aboutMe string
	if req.AboutMe != nil {
		aboutMe = *req.AboutMe
	}

	res, err := h.svc.Register(r.Context(), model.RegisterInput{
		Name:          req.Name,
		Email:         req.Email,
		Password:      req.Password,
		BirthDate:     birthDate,
		Sex:           model.Sex(req.Sex),
		SearchSex:     model.SearchSex(req.SearchSex),
		DatingGoal:    model.DatingGoalByIntent[req.DatingIntent],
		AboutMe:       aboutMe,
		SearchAgeFrom: req.SearchAgeFrom,
		SearchAgeTo:   req.SearchAgeTo,
		Tags:          req.Tags,
	})
	switch {
	case errors.Is(err, model.ErrEmailAlreadyExists):
		writeError(w, http.StatusConflict, codeEmailAlreadyExists, "Почта уже занята", nil)
		return
	case errors.Is(err, model.ErrPasswordTooLong):
		writeError(w, http.StatusBadRequest, codeValidationError, "Некорректные данные",
			map[string]string{"password": validate.ErrPasswordTooLong.Error()})
		return
	case errors.Is(err, model.ErrSessionNotOpened):
		// Аккаунт уже есть, но выдать токены не получилось
		slog.Error("register user: session not opened", "user_id", res.UserID, "error", err)
		writeJSON(w, http.StatusCreated, dto.AuthResponse{
			UserID:           res.UserID,
			ProfileCompleted: res.ProfileCompleted,
		})
		return
	case err != nil:
		slog.Error("register user", "error", err)
		writeError(w, http.StatusInternalServerError, codeInternalError, msgInternalError, nil)
		return
	}

	h.setSessionCookies(w, res.Tokens)
	writeJSON(w, http.StatusCreated, dto.AuthResponse{
		UserID:           res.UserID,
		ProfileCompleted: res.ProfileCompleted,
	})
}
