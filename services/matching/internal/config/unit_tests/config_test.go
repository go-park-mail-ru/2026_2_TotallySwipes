package config_test

import (
	"dating-app/services/matching/internal/config"
	"strings"
	"testing"
	"time"
)

func TestConfig(t *testing.T) {

	t.Setenv("DATABASE_URL", "postgres://localhost/test")
	t.Setenv("TIMEPAD_API_TOKEN", "test-token")
	t.Setenv("GRPC_PORT", "50051")
	t.Setenv("EVENT_SEARCH_TTL", "48h")
	t.Setenv("CHAT_GRPC_ADDR", "")
	cfg, err := config.Load()

	if err != nil || cfg.GRPCPort != "50051" || cfg.Redis.EventRepositoryTTL != 48*time.Hour || cfg.ChatGRPCAddr != "chat:50051" {
		t.Fatalf("config = %v, error = %v", cfg, err)
	}

	for _, key := range []string{"GRPC_PORT", "EVENT_SEARCH_TTL", "LOG_LEVEL"} {
		t.Run(key, func(t *testing.T) {

			t.Setenv(key, "invalid")

			if _, err := config.Load(); err == nil {
				t.Fatal("expected config error")
			}
		})
	}
}

func TestConnectionFields(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://matching_app@localhost/matching")
	t.Setenv("TIMEPAD_API_TOKEN", "token")
	t.Setenv("REDIS_ADDR", "redis:6379")
	t.Setenv("EVENT_SEARCH_TTL", "24h")
	t.Setenv("CHAT_GRPC_ADDR", "chat-instance:50052")

	cfg, err := config.Load()

	if err != nil || cfg.Database.URL != "postgres://matching_app@localhost/matching" || cfg.Redis.Addr != "redis:6379" || cfg.Redis.EventRepositoryTTL != 24*time.Hour || cfg.ChatGRPCAddr != "chat-instance:50052" {
		t.Fatalf("config = %#v, error = %v", cfg, err)
	}
}

func TestPositiveDurations(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://localhost/matching")
	t.Setenv("TIMEPAD_API_TOKEN", "token")

	for _, value := range []string{"0s", "-1s", "invalid"} {
		t.Run(value, func(t *testing.T) {
			t.Setenv("EVENT_SEARCH_TTL", value)

			if cfg, err := config.Load(); err == nil || cfg != nil {
				t.Fatalf("config = %#v, error = %v", cfg, err)
			}
		})
	}
}

func TestMissingEnvironmentAggregatesErrors(t *testing.T) {
	t.Setenv("DATABASE_URL", "")
	t.Setenv("TIMEPAD_API_TOKEN", "")

	cfg, err := config.Load()

	if cfg != nil || err == nil || !strings.Contains(err.Error(), "DATABASE_URL") || !strings.Contains(err.Error(), "TIMEPAD_API_TOKEN") {
		t.Fatalf("config = %#v, error = %v", cfg, err)
	}
}
