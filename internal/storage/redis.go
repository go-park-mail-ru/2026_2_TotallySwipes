package storage

import (
	"context"
	"fmt"

	"github.com/redis/go-redis/v9"
)

// OpenRedis создаёт клиента и проверяет, что Redis доступен
func OpenRedis(ctx context.Context, addr string) (*redis.Client, error) {
	rdb := redis.NewClient(&redis.Options{Addr: addr})

	if err := rdb.Ping(ctx).Err(); err != nil {
		rdb.Close()
		return nil, fmt.Errorf("ping redis: %w", err)
	}
	return rdb, nil
}
