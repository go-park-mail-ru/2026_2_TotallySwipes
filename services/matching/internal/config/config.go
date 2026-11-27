package config

import (
	"errors"
	"log/slog"
	"strconv"
	"time"
)

type Config struct {
	GRPCPort        string
	Database        DatabaseConfig
	Redis           RedisConfig
	ProfileGRPCAddr string
	ChatGRPCAddr    string
	TimepadBaseURL  string
	TimepadToken    string
	TimepadTimeout  time.Duration
	ConnectTimeout  time.Duration
	ShutdownTimeout time.Duration
	LogDir          string
	LogLevel        slog.Level
}

type DatabaseConfig struct {
	URL string
}

type RedisConfig struct {
	Addr               string
	EventRepositoryTTL time.Duration
}

// Load использует правила чтения env монолита и собирает ошибки вместе.
func Load() (*Config, error) {

	var r envReader

	cfg := &Config{
		GRPCPort: r.str("GRPC_PORT", "50051"),
		Database: DatabaseConfig{
			URL: r.required("DATABASE_URL"),
		},
		Redis: RedisConfig{
			Addr:               r.str("REDIS_ADDR", "localhost:6379"),
			EventRepositoryTTL: r.positiveDuration("EVENT_SEARCH_TTL", 48*time.Hour),
		},
		ProfileGRPCAddr: r.str("PROFILE_GRPC_ADDR", "profile:50051"),
		ChatGRPCAddr:    r.str("CHAT_GRPC_ADDR", "chat:50051"),
		TimepadBaseURL:  r.str("TIMEPAD_BASE_URL", "https://api.timepad.ru/v1/events.json"),
		TimepadToken:    r.required("TIMEPAD_API_TOKEN"),
		TimepadTimeout:  r.positiveDuration("TIMEPAD_HTTP_CLIENT_TIMEOUT", 5*time.Second),
		ConnectTimeout:  r.positiveDuration("CONNECT_TIMEOUT", 5*time.Second),
		ShutdownTimeout: r.positiveDuration("SHUTDOWN_TIMEOUT", 10*time.Second),
		LogDir:          r.str("LOG_DIR", "./logs/matching"),
	}

	port, err := strconv.Atoi(cfg.GRPCPort)

	if err != nil || port < 1 || port > 65535 {
		r.fail("GRPC_PORT", cfg.GRPCPort, errors.New("must be between 1 and 65535"))
	}

	level := r.str("LOG_LEVEL", "info")

	if err := cfg.LogLevel.UnmarshalText([]byte(level)); err != nil {
		r.fail("LOG_LEVEL", level, err)
	}

	if err := r.err(); err != nil {
		return nil, err
	}

	return cfg, nil
}
