// Package config читает настройки сервиса анкет из окружения (см. deploy/env/profile.env.example).
package config

import (
	"log/slog"
	"time"

	appconfig "dating-app/internal/config"
	"dating-app/internal/envreader"
)

type Config struct {
	GRPCPort        string
	DatabaseURL     string
	RedisAddr       string
	PublicURL       string
	LogDir          string
	LogLevel        slog.Level
	ShutdownTimeout time.Duration

	LocationPreviewTTL time.Duration
	MinIO              appconfig.MinIOConfig
	Geocoder           appconfig.GeocoderConfig
}

func Load() (*Config, error) {
	var r envreader.Reader

	cfg := &Config{
		GRPCPort:        r.Port("GRPC_PORT", "50051"),
		DatabaseURL:     r.Required("DATABASE_URL"),
		RedisAddr:       r.Str("REDIS_ADDR", "localhost:6379"),
		PublicURL:       r.Required("PUBLIC_URL"),
		LogDir:          r.Str("LOG_DIR", "./logs/profile"),
		LogLevel:        r.LogLevel("LOG_LEVEL", slog.LevelInfo),
		ShutdownTimeout: r.PositiveDuration("SHUTDOWN_TIMEOUT", 10*time.Second),

		LocationPreviewTTL: r.PositiveDuration("LOCATION_PREVIEW_TTL", 10*time.Minute),
		MinIO: appconfig.MinIOConfig{
			Endpoint:       r.Str("MINIO_ENDPOINT", "minio:9000"),
			PublicEndpoint: r.Str("MINIO_PUBLIC_ENDPOINT", "localhost:9000"),
			AccessKey:      r.Required("MINIO_ACCESS_KEY"),
			SecretKey:      r.Required("MINIO_SECRET_KEY"),
			Bucket:         r.Str("MINIO_BUCKET", "photos"),
			UseTLS:         r.Bool("MINIO_USE_TLS", false),
			PhotoURLTTL:    r.PositiveDuration("MINIO_PHOTO_URL_TTL", 15*time.Minute),
		},
		Geocoder: appconfig.GeocoderConfig{
			HTTPClientTimeout: r.PositiveDuration("GEOCODER_HTTP_CLIENT_TIMEOUT", 5*time.Second),
			BaseURL:           r.Str("GEOCODER_BASE_URL", "https://suggestions.dadata.ru/suggestions/api/4_1/rs/geolocate/address"),
			APIKey:            r.Required("GEOCODER_API_KEY"),
		},
	}

	if err := r.Err(); err != nil {
		return nil, err
	}
	return cfg, nil
}
