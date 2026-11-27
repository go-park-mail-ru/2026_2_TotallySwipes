package config_test

import (
	"dating-app/gateway/internal/config"
	"log/slog"
	"strings"
	"testing"
	"time"
)

func TestLoad(t *testing.T) {

	t.Setenv("JWT_SECRET", "test-secret")
	t.Setenv("PORT", "8081")
	t.Setenv("AUTH_GRPC_ADDR", "localhost:50051")
	t.Setenv("GRPC_REQUEST_TIMEOUT", "3s")
	t.Setenv("COOKIE_SECURE", "false")
	t.Setenv("LOG_LEVEL", "debug")
	t.Setenv("WS_ALLOWED_ORIGINS", " https://example.com, ,http://localhost:5173 ")
	t.Setenv("WS_PING_INTERVAL", "20s")
	t.Setenv("WS_PONG_TIMEOUT", "60s")
	t.Setenv("WS_MAX_MESSAGE_BYTES", "1024")
	t.Setenv("WS_SEND_QUEUE_SIZE", "8")
	cfg, err := config.Load()

	if err != nil {
		t.Fatal(err)
	}

	if cfg.HTTP.Port != "8081" || cfg.GRPC.AuthAddr != "localhost:50051" || cfg.GRPC.RequestTimeout != 3*time.Second || cfg.Auth.CookieSecure || cfg.Logging.Level != slog.LevelDebug {
		t.Fatal("environment overrides were not applied")
	}

	if len(cfg.WebSocket.AllowedOrigins) != 2 || cfg.WebSocket.AllowedOrigins[0] != "https://example.com" || cfg.WebSocket.MaxMessageBytes != 1024 || cfg.WebSocket.SendQueueSize != 8 {
		t.Fatalf("unexpected WebSocket configuration: %+v", cfg.WebSocket)
	}
}

func TestLoadCollectsErrors(t *testing.T) {

	t.Setenv("JWT_SECRET", "")
	t.Setenv("PORT", "70000")
	t.Setenv("GRPC_REQUEST_TIMEOUT", "0s")
	t.Setenv("WS_PING_INTERVAL", "60s")
	t.Setenv("WS_PONG_TIMEOUT", "30s")
	t.Setenv("WS_MAX_MESSAGE_BYTES", "-1")
	t.Setenv("WS_SEND_QUEUE_SIZE", "invalid")
	t.Setenv("LOG_LEVEL", "invalid")
	_, err := config.Load()

	if err == nil {
		t.Fatal("expected configuration errors")
	}

	for _, key := range []string{"JWT_SECRET", "PORT", "GRPC_REQUEST_TIMEOUT", "WS_PING_INTERVAL", "WS_MAX_MESSAGE_BYTES", "WS_SEND_QUEUE_SIZE", "LOG_LEVEL"} {

		if !strings.Contains(err.Error(), key) {
			t.Errorf("missing error for %s: %v", key, err)
		}
	}
}
