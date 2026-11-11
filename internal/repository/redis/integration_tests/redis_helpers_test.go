//go:build integration

package redis_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

// realRedis использует отдельную тестовую БД, которую очищает перед каждым тестом.
func realRedis(t *testing.T) *redis.Client {
	t.Helper()
	addr := os.Getenv("TEST_REDIS_ADDR")
	if addr == "" {
		t.Fatal("TEST_REDIS_ADDR is not set; run make test-integration")
	}
	client := redis.NewClient(&redis.Options{Addr: addr, DB: 15})
	t.Cleanup(func() { client.Close() })
	if err := client.FlushDB(context.Background()).Err(); err != nil {
		t.Fatal(err)
	}
	return client
}

// expireKeys сокращает TTL в настоящем Redis и ждёт удаления ключей сервером.
func expireKeys(t *testing.T, client *redis.Client, keys ...string) {
	t.Helper()
	ctx := context.Background()
	for _, key := range keys {
		if err := client.PExpire(ctx, key, time.Millisecond).Err(); err != nil {
			t.Fatal(err)
		}
	}
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		n, err := client.Exists(ctx, keys...).Result()
		if err != nil {
			t.Fatal(err)
		}
		if n == 0 {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("Redis keys did not expire")
}
