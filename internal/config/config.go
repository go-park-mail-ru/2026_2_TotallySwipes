package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	Database DatabaseConfig
	Redis    RedisConfig
	Auth     AuthConfig
	HTTP     HTTPConfig
}

type DatabaseConfig struct {
	URL string
}

type RedisConfig struct {
	Addr string
}

type AuthConfig struct {
	JWTSecret     string
	JWTAccessTTL  time.Duration
	JWTRefreshTTL time.Duration
	CookieSecure  bool
}

type HTTPConfig struct {
	Port              string
	ReadHeaderTimeout time.Duration
	ReadTimeout       time.Duration
	WriteTimeout      time.Duration
	IdleTimeout       time.Duration
}

// Load читает конфигурацию из переменных окружения. Обязательны только
// DATABASE_URL и JWT_SECRET - у них нет разумного дефолта. Остальное - с
// дефолтом, но если задано криво, тоже ошибка. Все ошибки отдаются разом
func Load() (*Config, error) {
	var r envReader

	cfg := &Config{
		Database: DatabaseConfig{
			URL: r.required("DATABASE_URL"),
		},
		Redis: RedisConfig{
			Addr: r.str("REDIS_ADDR", "localhost:6379"),
		},
		Auth: AuthConfig{
			JWTSecret:     r.required("JWT_SECRET"),
			JWTAccessTTL:  r.positiveDuration("JWT_ACCESS_TTL", 15*time.Minute),
			JWTRefreshTTL: r.positiveDuration("JWT_REFRESH_TTL", 30*24*time.Hour),
			CookieSecure:  r.boolean("COOKIE_SECURE", true),
		},
		HTTP: HTTPConfig{
			Port:              r.str("PORT", "8080"),
			ReadHeaderTimeout: r.duration("HTTP_READ_HEADER_TIMEOUT", 5*time.Second),
			ReadTimeout:       r.duration("HTTP_READ_TIMEOUT", 10*time.Second),
			WriteTimeout:      r.duration("HTTP_WRITE_TIMEOUT", 10*time.Second),
			IdleTimeout:       r.duration("HTTP_IDLE_TIMEOUT", 60*time.Second),
		},
	}

	if err := r.err(); err != nil {
		return nil, err
	}
	return cfg, nil
}

// envReader читает переменные и копит ошибки
type envReader struct {
	errs []error
}

func (r *envReader) err() error {
	return errors.Join(r.errs...)
}

func (r *envReader) fail(key, v string, err error) {
	r.errs = append(r.errs, fmt.Errorf("invalid %s %q: %w", key, v, err))
}

func (r *envReader) required(key string) string {
	v := os.Getenv(key)
	if v == "" {
		r.errs = append(r.errs, fmt.Errorf("missing required environment variable %s", key))
	}
	return v
}

func (r *envReader) str(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func (r *envReader) boolean(key string, def bool) bool {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		r.fail(key, v, err)
	}
	return b
}

// duration допускает 0
func (r *envReader) duration(key string, def time.Duration) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		r.fail(key, v, err)
		return 0
	}
	if d < 0 {
		r.fail(key, v, errors.New("must not be negative"))
	}
	return d
}

func (r *envReader) positiveDuration(key string, def time.Duration) time.Duration {
	before := len(r.errs)
	d := r.duration(key, def)
	// Если duration уже ругнулся, второй ошибки про ту же переменную не нужно
	if len(r.errs) == before && d == 0 {
		r.fail(key, os.Getenv(key), errors.New("must be positive"))
	}
	return d
}
