package handler

import (
	"dating-app/internal/handler/dto"
	"dating-app/internal/model"
	"dating-app/internal/validate"
	"errors"
	"io"
	"log/slog"
	"maps"
	"mime/multipart"
	"net/http"
	"strconv"
	"strings"
)

const (
	registerMaxMemory   = 8 << 20
	registerMaxBodySize = 6*validate.PhotoMaxSize + 1<<20
)

func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, registerMaxBodySize)
	if err := r.ParseMultipartForm(registerMaxMemory); err != nil {
		if _, ok := errors.AsType[*http.MaxBytesError](err); ok {
			writeError(w, http.StatusRequestEntityTooLarge, codePayloadTooLarge, "Слишком большой запрос", nil)
			return
		}
		writeError(w, http.StatusBadRequest, codeValidationError, "Ожидается multipart/form-data", nil)
		return
	}
	defer r.MultipartForm.RemoveAll()

	req, parseErrs := registerRequestFromForm(r.MultipartForm)
	req.Normalize()
	errs := req.Validate()
	maps.Copy(errs, parseErrs)
	if len(errs) > 0 {
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
		Photos:        req.Photos,
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

func registerRequestFromForm(form *multipart.Form) (dto.RegisterRequest, map[string]string) {
	errs := make(map[string]string)
	value := func(key string) string {
		if v := form.Value[key]; len(v) > 0 {
			return v[0]
		}
		return ""
	}
	intValue := func(key string) int {
		v, ok := form.Value[key]
		if !ok || len(v) == 0 {
			errs[key] = validate.ErrRequired.Error()
			return 0
		}
		n, err := strconv.Atoi(strings.TrimSpace(v[0]))
		if err != nil {
			errs[key] = validate.ErrNotInteger.Error()
		}
		return n
	}

	req := dto.RegisterRequest{
		Name:          value("name"),
		Email:         value("email"),
		Password:      value("password"),
		BirthDate:     value("birth_date"),
		Sex:           value("sex"),
		SearchSex:     value("search_sex"),
		DatingIntent:  value("dating_intent"),
		SearchAgeFrom: intValue("search_age_from"),
		SearchAgeTo:   intValue("search_age_to"),
		Tags:          form.Value["tags"],
	}
	if v, ok := form.Value["about_me"]; ok && len(v) > 0 {
		req.AboutMe = &v[0]
	}

	for _, fh := range form.File["photos"] {
		data, err := readPhoto(fh)
		if err != nil {
			errs["photos"] = validate.ErrPhotoFormat.Error()
			continue
		}
		req.Photos = append(req.Photos, model.PhotoUpload{Data: data, Ext: validate.PhotoExt(data)})
	}

	return req, errs
}

func readPhoto(fh *multipart.FileHeader) ([]byte, error) {
	f, err := fh.Open()
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return io.ReadAll(io.LimitReader(f, validate.PhotoMaxSize+1))
}
