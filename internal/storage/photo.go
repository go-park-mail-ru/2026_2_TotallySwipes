package storage

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
)

const uploadsDir = "uploads"

type LocalPhotoStorage struct {
	dir string
}

func NewLocalPhotoStorage(dir string) *LocalPhotoStorage {
	return &LocalPhotoStorage{dir: dir}
}

func (s *LocalPhotoStorage) Save(_ context.Context, data []byte, ext string) (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("save photo: random name: %w", err)
	}
	key := path.Join(uploadsDir, hex.EncodeToString(b)+ext)

	if err := os.MkdirAll(filepath.Join(s.dir, uploadsDir), 0o755); err != nil {
		return "", fmt.Errorf("save photo: %w", err)
	}

	f, err := os.OpenFile(filepath.Join(s.dir, filepath.FromSlash(key)), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return "", fmt.Errorf("save photo %s: %w", key, err)
	}
	if _, err := f.Write(data); err != nil {
		f.Close()
		os.Remove(f.Name())
		return "", fmt.Errorf("save photo %s: %w", key, err)
	}
	if err := f.Close(); err != nil {
		os.Remove(f.Name())
		return "", fmt.Errorf("save photo %s: %w", key, err)
	}
	return key, nil
}

func (s *LocalPhotoStorage) Delete(_ context.Context, key string) error {
	if !fs.ValidPath(key) {
		return fmt.Errorf("delete photo %q: invalid key", key)
	}
	err := os.Remove(filepath.Join(s.dir, filepath.FromSlash(key)))
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("delete photo %s: %w", key, err)
	}
	return nil
}
