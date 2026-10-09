package service

import (
	"context"
	"dating-app/internal/model"
	"fmt"
	"io/fs"
	"net/url"
	"strings"
)

type URLProvider interface {
	// GetURL возвращает публичный URL фото или model.ErrInvalidPhotoStorageKey.
	GetURL(ctx context.Context, storageKey string) (string, error)
}

// GetURL формирует публичный URL фотографии с экранированием пути.
// Принимает: контекст ctx (не используется) и относительный ключ storageKey.
// Возвращает: URL или model.ErrInvalidPhotoStorageKey, если ключ недопустим.
func (p *LocalPhotoURLProvider) GetURL(ctx context.Context, storageKey string) (string, error) {
	if storageKey == "." || !fs.ValidPath(storageKey) || strings.Contains(storageKey, "\\") {
		return "", fmt.Errorf("get photo URL key=%q: %w", storageKey, model.ErrInvalidPhotoStorageKey)
	}
	photoPath := (&url.URL{Path: "/cats/" + storageKey}).EscapedPath()

	return strings.TrimRight(p.mediaBaseURL, "/") + photoPath, nil
}

type LocalPhotoURLProvider struct {
	mediaBaseURL string
}

// NewLocalPhotoURLProvider создаёт поставщик URL локальных фотографий.
// Принимает: базовый публичный адрес baseURL.
// Возвращает: экземпляр LocalPhotoURLProvider.
func NewLocalPhotoURLProvider(baseURL string) *LocalPhotoURLProvider {
	return &LocalPhotoURLProvider{
		mediaBaseURL: baseURL,
	}
}
