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
	name, ok := strings.CutPrefix(key, "profiles/")
	if !ok || !strings.HasSuffix(name, ".jpg") || strings.Contains(name, "/") {
		t.Errorf("key = %q", key)
	}
	got, err := os.ReadFile(filepath.Join(dir, "profiles", name))
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
	if _, err := os.Stat(filepath.Join(dir, "profiles", name)); !os.IsNotExist(err) {
		t.Errorf("file must be deleted, stat err = %v", err)
	}
	if err := s.Delete(ctx, key); err != nil {
		t.Errorf("delete missing: %v", err)
	}
	for _, bad := range []string{"../secret", "profiles/../secret", "profiles/a/b.jpg", "demo/profile_01.jpg"} {
		if err := s.Delete(ctx, bad); err == nil {
			t.Errorf("key %q must be rejected", bad)
		}
	}
}
