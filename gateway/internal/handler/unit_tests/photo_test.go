package handler

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	. "dating-app/internal/handler"
	"dating-app/internal/model"

	"github.com/gorilla/mux"
)

type nopSeekCloser struct{ *bytes.Reader }

func (nopSeekCloser) Close() error { return nil }

type fakePhotoReader struct {
	data []byte
	err  error
	key  string
}

func (f *fakePhotoReader) OpenPhoto(_ context.Context, key string) (io.ReadSeekCloser, time.Time, error) {
	f.key = key
	if f.err != nil {
		return nil, time.Time{}, f.err
	}
	return nopSeekCloser{bytes.NewReader(f.data)}, time.Date(2026, 11, 7, 0, 0, 0, 0, time.UTC), nil
}

func servePhoto(reader *fakePhotoReader, target string, hdr http.Header) *httptest.ResponseRecorder {
	r := mux.NewRouter()
	r.HandleFunc("/media/{key:.+}", NewPhotoHandler(reader).Get)
	req := httptest.NewRequest(http.MethodGet, target, nil)
	for k, v := range hdr {
		req.Header[k] = v
	}
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec
}

func TestPhotoHandlerGet(t *testing.T) {
	reader := &fakePhotoReader{data: []byte("\x89PNG-data")}
	rec := servePhoto(reader, "/media/photos/abc.png", nil)
	if rec.Code != http.StatusOK || rec.Body.String() != "\x89PNG-data" || reader.key != "photos/abc.png" {
		t.Fatalf("code=%d body=%q key=%q", rec.Code, rec.Body.String(), reader.key)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "image/png" {
		t.Fatalf("content-type=%q", ct)
	}

	rec = servePhoto(reader, "/media/photos/abc.png", http.Header{"Range": {"bytes=0-3"}})
	if rec.Code != http.StatusPartialContent || rec.Body.String() != "\x89PNG" {
		t.Fatalf("range: code=%d body=%q", rec.Code, rec.Body.String())
	}
}

func TestPhotoHandlerNotFound(t *testing.T) {
	for _, err := range []error{model.ErrPhotoNotFound, model.ErrInvalidPhotoStorageKey} {
		rec := servePhoto(&fakePhotoReader{err: err}, "/media/photos/x.png", nil)
		if rec.Code != http.StatusNotFound {
			t.Errorf("err=%v code=%d", err, rec.Code)
		}
	}
}
