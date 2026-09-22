package config

import (
	"fmt"
	"os"
	"strings"
	"time"
)

type Config struct {
	Port          string
	DatabaseURL   string
	JWTSecret     string
	JWTAccessTTL  time.Duration
	JWTRefreshTTL time.Duration

	HTTPReadHeaderTimeout time.Duration
	HTTPReadTimeout       time.Duration
	HTTPWriteTimeout      time.Duration
	HTTPIdleTimeout       time.Duration
}

const (
	defaultPort = "8080"

	defaultJWTAccessTTL  = 15 * time.Minute
	defaultJWTRefreshTTL = 30 * 24 * time.Hour

	defaultHTTPReadHeaderTimeout = 5 * time.Second
	defaultHTTPReadTimeout       = 10 * time.Second
	defaultHTTPWriteTimeout      = 10 * time.Second
	defaultHTTPIdleTimeout       = 60 * time.Second
)

// Load читает конфигурацию из переменных окружения. Обязательны только
// DATABASE_URL и JWT_SECRET - у них нет разумного дефолта. Остальное - с
// дефолтом, но если задано криво, тоже ошибка.
func Load() (*Config, error) {
	cfg := &Config{
		Port: envOrDefault("PORT", defaultPort),
	}

	var missing []string

	cfg.DatabaseURL = requireEnv("DATABASE_URL", &missing)
	cfg.JWTSecret = requireEnv("JWT_SECRET", &missing)

	if len(missing) > 0 {
		return nil, fmt.Errorf("missing required environment variables: %s", strings.Join(missing, ", "))
	}

	var err error

	if cfg.JWTAccessTTL, err = envDurationOrDefault("JWT_ACCESS_TTL", defaultJWTAccessTTL); err != nil {
		return nil, err
	}
	if cfg.JWTRefreshTTL, err = envDurationOrDefault("JWT_REFRESH_TTL", defaultJWTRefreshTTL); err != nil {
		return nil, err
	}
	if cfg.HTTPReadHeaderTimeout, err = envDurationOrDefault("HTTP_READ_HEADER_TIMEOUT", defaultHTTPReadHeaderTimeout); err != nil {
		return nil, err
	}
	if cfg.HTTPReadTimeout, err = envDurationOrDefault("HTTP_READ_TIMEOUT", defaultHTTPReadTimeout); err != nil {
		return nil, err
	}
	if cfg.HTTPWriteTimeout, err = envDurationOrDefault("HTTP_WRITE_TIMEOUT", defaultHTTPWriteTimeout); err != nil {
		return nil, err
	}
	if cfg.HTTPIdleTimeout, err = envDurationOrDefault("HTTP_IDLE_TIMEOUT", defaultHTTPIdleTimeout); err != nil {
		return nil, err
	}

	return cfg, nil
}

func envOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func requireEnv(key string, missing *[]string) string {
	v := os.Getenv(key)
	if v == "" {
		*missing = append(*missing, key)
	}
	return v
}

func envDurationOrDefault(key string, def time.Duration) (time.Duration, error) {
	v := os.Getenv(key)
	if v == "" {
		return def, nil
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return 0, fmt.Errorf("invalid %s %q: %w", key, v, err)
	}
	return d, nil
}
