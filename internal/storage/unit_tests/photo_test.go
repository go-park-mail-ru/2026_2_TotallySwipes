package storage

import (
	"context"
	. "dating-app/internal/storage"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLocalPhotoStorage(t *testing.T) {
	dir := t.TempDir()
	s := NewLocalPhotoStorage(dir)
	ctx := context.Background()

	key, err := s.Save(ctx, []byte("data"), ".jpg")
	if err != nil {
		t.Fatalf("save: %v", err)
	}
	if !strings.HasPrefix(key, "uploads/") || !strings.HasSuffix(key, ".jpg") {
		t.Errorf("key = %q", key)
	}
	got, err := os.ReadFile(filepath.Join(dir, key))
	if err != nil || string(got) != "data" {
		t.Fatalf("file = %q, err = %v", got, err)
	}

	other, _ := s.Save(ctx, []byte("data"), ".jpg")
	if other == key {
		t.Error("keys must be unique")
	}

	if err := s.Delete(ctx, key); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, key)); !os.IsNotExist(err) {
		t.Errorf("file must be deleted, stat err = %v", err)
	}
	if err := s.Delete(ctx, key); err != nil {
		t.Errorf("delete missing: %v", err)
	}
	if err := s.Delete(ctx, "../secret"); err == nil {
		t.Error("key outside storage must be rejected")
	}
}
