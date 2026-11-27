// Package config читает настройки сервиса авторизации из окружения (см. deploy/env/auth.env.example).
package config

import (
	"log/slog"
	"time"

	"dating-app/internal/envreader"
)

type Config struct {
	GRPCPort           string
	DatabaseURL        string
	RedisAddr          string
	ProfileGRPCAddr    string
	GRPCRequestTimeout time.Duration
	LogDir             string
	LogLevel           slog.Level
	ShutdownTimeout    time.Duration

	JWTSecret     string
	JWTAccessTTL  time.Duration
	JWTRefreshTTL time.Duration
}

func Load() (*Config, error) {
	var r envreader.Reader

	cfg := &Config{
		GRPCPort:           r.Port("GRPC_PORT", "50051"),
		DatabaseURL:        r.Required("DATABASE_URL"),
		RedisAddr:          r.Str("REDIS_ADDR", "localhost:6379"),
		ProfileGRPCAddr:    r.Str("PROFILE_GRPC_ADDR", "profile:50051"),
		GRPCRequestTimeout: r.PositiveDuration("GRPC_REQUEST_TIMEOUT", 5*time.Second),
		LogDir:             r.Str("LOG_DIR", "./logs/auth"),
		LogLevel:           r.LogLevel("LOG_LEVEL", slog.LevelInfo),
		ShutdownTimeout:    r.PositiveDuration("SHUTDOWN_TIMEOUT", 10*time.Second),

		JWTSecret:     r.Required("JWT_SECRET"),
		JWTAccessTTL:  r.PositiveDuration("JWT_ACCESS_TTL", 15*time.Minute),
		JWTRefreshTTL: r.PositiveDuration("JWT_REFRESH_TTL", 30*24*time.Hour),
	}

	if err := r.Err(); err != nil {
		return nil, err
	}
	return cfg, nil
}
