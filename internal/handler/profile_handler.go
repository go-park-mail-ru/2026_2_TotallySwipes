package handler

import (
	"context"
	"dating-app/internal/handler/dto"
	"dating-app/internal/model"
	"dating-app/internal/validate"
	"errors"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/gorilla/mux"
)

const (
	photoUploadMaxMemory = 1 << 20
	photoUploadMaxBody   = validate.PhotoMaxSize + 1<<20
)

type ProfileService interface {
	GetShortProfile(ctx context.Context, userID int64) (*model.ProfileShort, error)
	GetMyProfile(ctx context.Context, userID int64) (*model.Profile, error)
	UpdateProfile(ctx context.Context, userID int64, patch *model.ProfilePatch) (*model.Profile, error)
	AddPhoto(ctx context.Context, userID int64, upload model.PhotoUpload) ([]model.Photo, error)
	DeletePhoto(ctx context.Context, userID, photoID int64) ([]model.Photo, error)
	ReorderPhotos(ctx context.Context, userID int64, photoIDs []int64) ([]model.Photo, error)
}

type ProfileHandler struct {
	svc ProfileService
}

func NewProfileHandler(svc ProfileService) *ProfileHandler {
	return &ProfileHandler{svc: svc}
}

// ShortProfile - GET /profile/me/short: имя, возраст, главное фото и missing
func (h *ProfileHandler) ShortProfile(w http.ResponseWriter, r *http.Request, userID int64) {
	short, err := h.svc.GetShortProfile(r.Context(), userID)
	if err != nil {
		writeServiceError(w, "get short profile", err)
		return
	}
	writeJSON(w, http.StatusOK, dto.NewProfileShortResponse(short))
}

// Get - GET /profile/me: анкета целиком и список незаполненных обязательных полей
func (h *ProfileHandler) Get(w http.ResponseWriter, r *http.Request, userID int64) {
	profile, err := h.svc.GetMyProfile(r.Context(), userID)
	if err != nil {
		writeServiceError(w, "get my profile", err)
		return
	}
	writeJSON(w, http.StatusOK, dto.NewProfileResponse(profile, time.Now()))
}

// Update - PATCH /profile/me: меняет присланные поля, каждый вызов - новая версия анкеты
func (h *ProfileHandler) Update(w http.ResponseWriter, r *http.Request, userID int64) {
	var req dto.UpdateProfileRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	patch, errs := req.ToPatch()
	if len(errs) > 0 {
		writeError(w, http.StatusBadRequest, codeValidationError, msgInvalidData, errs)
		return
	}

	profile, err := h.svc.UpdateProfile(r.Context(), userID, patch)
	if err != nil {
		writeServiceError(w, "update profile", err)
		return
	}
	writeJSON(w, http.StatusOK, dto.NewProfileResponse(profile, time.Now()))
}

// AddPhoto - POST /profile/me/photos: одно фото в части photo, добавляется в конец
func (h *ProfileHandler) AddPhoto(w http.ResponseWriter, r *http.Request, userID int64) {
	upload, ok := readPhotoUpload(w, r)
	if !ok {
		return
	}

	photos, err := h.svc.AddPhoto(r.Context(), userID, upload)
	if err != nil {
		writeServiceError(w, "add photo", err)
		return
	}
	writeJSON(w, http.StatusCreated, dto.NewPhotosResponse(photos))
}

// DeletePhoto - DELETE /profile/me/photos/{photo_id}: следующие фото сдвигаются на его место
func (h *ProfileHandler) DeletePhoto(w http.ResponseWriter, r *http.Request, userID int64) {
	photoID, err := strconv.ParseInt(mux.Vars(r)["photo_id"], 10, 64)
	if err != nil || photoID <= 0 {
		writeError(w, http.StatusBadRequest, codeValidationError, "Некорректный photo_id", nil)
		return
	}

	photos, err := h.svc.DeletePhoto(r.Context(), userID, photoID)
	if err != nil {
		writeServiceError(w, "delete photo", err)
		return
	}
	writeJSON(w, http.StatusOK, dto.NewPhotosResponse(photos))
}

// ReorderPhotos - PUT /profile/me/photos/order: задаёт порядок всех фото, первое становится главным
func (h *ProfileHandler) ReorderPhotos(w http.ResponseWriter, r *http.Request, userID int64) {
	var req dto.ReorderPhotosRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if errs := req.Validate(); len(errs) > 0 {
		writeError(w, http.StatusBadRequest, codeValidationError, msgInvalidData, errs)
		return
	}

	photos, err := h.svc.ReorderPhotos(r.Context(), userID, req.PhotoIDs)
	if err != nil {
		writeServiceError(w, "reorder photos", err)
		return
	}
	writeJSON(w, http.StatusOK, dto.NewPhotosResponse(photos))
}

// readPhotoUpload читает один файл из части photo; при ошибке сам отвечает клиенту
func readPhotoUpload(w http.ResponseWriter, r *http.Request) (model.PhotoUpload, bool) {
	r.Body = http.MaxBytesReader(w, r.Body, photoUploadMaxBody)
	if err := r.ParseMultipartForm(photoUploadMaxMemory); err != nil {
		if _, ok := errors.AsType[*http.MaxBytesError](err); ok {
			writeError(w, http.StatusRequestEntityTooLarge, codePayloadTooLarge, "Слишком большой запрос", nil)
			return model.PhotoUpload{}, false
		}
		writeError(w, http.StatusBadRequest, codeValidationError, "Ожидается multipart/form-data", nil)
		return model.PhotoUpload{}, false
	}
	defer r.MultipartForm.RemoveAll()

	files := r.MultipartForm.File["photo"]
	if len(files) != 1 {
		writeError(w, http.StatusBadRequest, codeValidationError, msgInvalidData,
			map[string]string{"photo": "нужно передать ровно один файл"})
		return model.PhotoUpload{}, false
	}

	f, err := files[0].Open()
	if err != nil {
		writeError(w, http.StatusBadRequest, codeValidationError, msgInvalidData,
			map[string]string{"photo": validate.ErrPhotoFormat.Error()})
		return model.PhotoUpload{}, false
	}
	defer f.Close()

	data, err := io.ReadAll(io.LimitReader(f, validate.PhotoMaxSize+1))
	if err != nil {
		writeError(w, http.StatusBadRequest, codeValidationError, msgInvalidData,
			map[string]string{"photo": validate.ErrPhotoFormat.Error()})
		return model.PhotoUpload{}, false
	}

	upload := model.PhotoUpload{Data: data, Ext: validate.PhotoExt(data)}
	if err := validate.ValidatePhoto(upload); err != nil {
		writeError(w, http.StatusBadRequest, codeValidationError, msgInvalidData, map[string]string{"photo": err.Error()})
		return model.PhotoUpload{}, false
	}
	return upload, true
}
