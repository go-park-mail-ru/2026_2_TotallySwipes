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
	GetURL(ctx context.Context, storageKey string) (string, error)
}

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

func NewLocalPhotoURLProvider(baseURL string) *LocalPhotoURLProvider {
	return &LocalPhotoURLProvider{
		mediaBaseURL: baseURL,
	}
}
