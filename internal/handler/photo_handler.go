package handler

import (
	"context"
	"dating-app/internal/model"
	"errors"
	"io"
	"net/http"
	"path"
	"time"

	"github.com/gorilla/mux"
)

// PhotoReader открывает файл фото по ключу хранилища.
type PhotoReader interface {
	OpenPhoto(ctx context.Context, key string) (io.ReadSeekCloser, time.Time, error)
}

type PhotoHandler struct {
	photos PhotoReader
}

func NewPhotoHandler(photos PhotoReader) *PhotoHandler {
	return &PhotoHandler{photos: photos}
}

// Get - GET /media/{key}: отдаёт фото из хранилища (ключи — случайные UUID).
func (h *PhotoHandler) Get(w http.ResponseWriter, r *http.Request) {
	key := mux.Vars(r)["key"]

	body, modTime, err := h.photos.OpenPhoto(r.Context(), key)
	if err != nil {
		if errors.Is(err, model.ErrInvalidPhotoStorageKey) {
			err = model.ErrPhotoNotFound
		}
		writeServiceError(r.Context(), w, "get photo", err)
		return
	}
	defer body.Close()

	w.Header().Set("Cache-Control", "public, max-age=86400")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	// ServeContent берёт Content-Type по расширению и поддерживает Range/If-Modified-Since.
	http.ServeContent(w, r, path.Base(key), modTime, body)
}
