package config_test

import (
	"strings"
	"testing"
	"time"

	"dating-app/services/chat/internal/config"
)

func TestLoadConnections(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://chat_app@localhost/chat")
	t.Setenv("REDIS_ADDR", "redis:6379")

	cfg, err := config.Load()

	if err != nil || cfg.Database.URL != "postgres://chat_app@localhost/chat" || cfg.Redis.Addr != "redis:6379" {
		t.Fatalf("config = %#v, error = %v", cfg, err)
	}
}

func TestLoadDefaultRedis(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://localhost/chat")
	t.Setenv("REDIS_ADDR", "")

	cfg, err := config.Load()

	if err != nil || cfg.Redis.Addr != "localhost:6379" {
		t.Fatalf("config = %#v, error = %v", cfg, err)
	}
}

func TestLoadRequiresDatabase(t *testing.T) {
	t.Setenv("DATABASE_URL", "")

	cfg, err := config.Load()

	if cfg != nil || err == nil || !strings.Contains(err.Error(), "DATABASE_URL") {
		t.Fatalf("config = %#v, error = %v", cfg, err)
	}
}

func TestChatRuntimeConfig(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://localhost/chat")
	t.Setenv("GRPC_PORT", "50052")
	t.Setenv("PROFILE_GRPC_ADDR", "profile:50051")
	t.Setenv("CHAT_EVENT_QUEUE_SIZE", "64")
	t.Setenv("MINIO_BUCKET", "chat-media")
	t.Setenv("SHUTDOWN_TIMEOUT", "3s")

	cfg, err := config.Load()

	if err != nil || cfg.GRPCPort != "50052" || cfg.BrokerQueueSize != 64 || cfg.MinIO.Bucket != "chat-media" || cfg.ShutdownTimeout != 3*time.Second {
		t.Fatalf("config = %#v, error = %v", cfg, err)
	}
}

func TestInvalidChatRuntimeConfig(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://localhost/chat")

	for _, tc := range []struct{ key, value string }{
		{"GRPC_PORT", "70000"}, {"CHAT_EVENT_QUEUE_SIZE", "0"},
		{"MINIO_USE_TLS", "invalid"}, {"MINIO_DOWNLOAD_URL_TTL", "0s"},
		{"MINIO_DOWNLOAD_URL_TTL", "192h"}, {"CONNECT_TIMEOUT", "-1s"},
		{"SHUTDOWN_TIMEOUT", "0s"},
	} {
		t.Run(tc.key+tc.value, func(t *testing.T) {
			t.Setenv(tc.key, tc.value)

			if cfg, err := config.Load(); cfg != nil || err == nil {
				t.Fatalf("config = %#v, error = %v", cfg, err)
			}
		})
	}
}
