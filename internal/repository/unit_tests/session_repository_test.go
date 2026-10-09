package repository

import (
	"context"
	. "dating-app/internal/repository"
	"errors"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"

	"dating-app/internal/model"
)

func newTestSessionRepo(t *testing.T) (*SessionRepo, *miniredis.Miniredis) {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { rdb.Close() })
	return NewSessionRepository(rdb), mr
}

func TestSessionRepo_CreateGetRevoke(t *testing.T) {
	repo, mr := newTestSessionRepo(t)
	ctx := context.Background()
	expiresAt := time.Now().Add(time.Hour).Truncate(time.Second)

	err := repo.Create(ctx, &model.Session{ID: "sid", UserID: 12, TokenHash: "h", ExpiresAt: expiresAt})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	for _, key := range []string{"refresh:session:sid", "refresh:hash:h"} {
		if ttl := mr.TTL(key); ttl <= 0 || ttl > time.Hour {
			t.Errorf("%s ttl = %v, want (0, 1h]", key, ttl)
		}
	}

	got, err := repo.GetByTokenHash(ctx, "h")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	want := model.Session{ID: "sid", UserID: 12, TokenHash: "h", ExpiresAt: expiresAt}
	if *got != want {
		t.Errorf("got %+v, want %+v", *got, want)
	}

	if ok, err := repo.Revoke(ctx, "sid"); err != nil || !ok {
		t.Fatalf("revoke: ok = %v, err = %v", ok, err)
	}
	got, _ = repo.GetByTokenHash(ctx, "h")
	if !got.Revoked {
		t.Error("session must be revoked")
	}
}

func TestSessionRepo_NotFound(t *testing.T) {
	repo, mr := newTestSessionRepo(t)
	ctx := context.Background()

	if _, err := repo.GetByTokenHash(ctx, "nope"); !errors.Is(err, model.ErrNotFound) {
		t.Errorf("unknown hash: err = %v", err)
	}

	_ = repo.Create(ctx, &model.Session{ID: "sid", UserID: 1, TokenHash: "h", ExpiresAt: time.Now().Add(time.Minute)})
	mr.FastForward(2 * time.Minute)
	if _, err := repo.GetByTokenHash(ctx, "h"); !errors.Is(err, model.ErrNotFound) {
		t.Errorf("expired: err = %v", err)
	}

	if ok, err := repo.Revoke(ctx, "sid"); err != nil || ok {
		t.Fatalf("revoke missing: ok = %v, err = %v", ok, err)
	}
	if mr.Exists("refresh:session:sid") {
		t.Error("revoke must not recreate expired session")
	}
}

func TestSessionRepo_RevokeOnce(t *testing.T) {
	repo, mr := newTestSessionRepo(t)
	ctx := context.Background()
	_ = repo.Create(ctx, &model.Session{ID: "sid", UserID: 1, TokenHash: "h", ExpiresAt: time.Now().Add(time.Hour)})

	ok, err := repo.Revoke(ctx, "sid")
	if err != nil || !ok {
		t.Fatalf("first revoke: ok = %v, err = %v", ok, err)
	}
	if got, _ := repo.GetByTokenHash(ctx, "h"); !got.Revoked {
		t.Error("session must be revoked")
	}

	if ok, err := repo.Revoke(ctx, "sid"); err != nil || ok {
		t.Errorf("second revoke: ok = %v, err = %v", ok, err)
	}

	if ok, err := repo.Revoke(ctx, "missing"); err != nil || ok {
		t.Errorf("missing: ok = %v, err = %v", ok, err)
	}
	if mr.Exists("refresh:session:missing") {
		t.Error("revoke must not create session key")
	}
}
