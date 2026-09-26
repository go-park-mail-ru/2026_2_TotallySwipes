package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
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

const (
	defaultPort = "8080"

	defaultRedisAddr = "localhost:6379"

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
	var missing []string

	databaseURL := requireEnv("DATABASE_URL", &missing)
	jwtSecret := requireEnv("JWT_SECRET", &missing)

	if len(missing) > 0 {
		return nil, fmt.Errorf("missing required environment variables: %s", strings.Join(missing, ", "))
	}

	auth, err := loadAuthConfig(jwtSecret)
	if err != nil {
		return nil, err
	}

	httpCfg, err := loadHTTPConfig()
	if err != nil {
		return nil, err
	}

	return &Config{
		Database: DatabaseConfig{URL: databaseURL},
		Redis:    loadRedisConfig(),
		Auth:     auth,
		HTTP:     httpCfg,
	}, nil
}

func loadAuthConfig(jwtSecret string) (AuthConfig, error) {
	accessTTL, err := envDurationOrDefault("JWT_ACCESS_TTL", defaultJWTAccessTTL, false)
	if err != nil {
		return AuthConfig{}, err
	}
	refreshTTL, err := envDurationOrDefault("JWT_REFRESH_TTL", defaultJWTRefreshTTL, false)
	if err != nil {
		return AuthConfig{}, err
	}

	cookieSecure, err := envBoolOrDefault("COOKIE_SECURE", true)
	if err != nil {
		return AuthConfig{}, err
	}

	return AuthConfig{
		JWTSecret:     jwtSecret,
		JWTAccessTTL:  accessTTL,
		JWTRefreshTTL: refreshTTL,
		CookieSecure:  cookieSecure,
	}, nil
}

func loadRedisConfig() RedisConfig {
	return RedisConfig{
		Addr: envOrDefault("REDIS_ADDR", defaultRedisAddr),
	}
}

func loadHTTPConfig() (HTTPConfig, error) {
	readHeaderTimeout, err := envDurationOrDefault("HTTP_READ_HEADER_TIMEOUT", defaultHTTPReadHeaderTimeout, true)
	if err != nil {
		return HTTPConfig{}, err
	}
	readTimeout, err := envDurationOrDefault("HTTP_READ_TIMEOUT", defaultHTTPReadTimeout, true)
	if err != nil {
		return HTTPConfig{}, err
	}
	writeTimeout, err := envDurationOrDefault("HTTP_WRITE_TIMEOUT", defaultHTTPWriteTimeout, true)
	if err != nil {
		return HTTPConfig{}, err
	}
	idleTimeout, err := envDurationOrDefault("HTTP_IDLE_TIMEOUT", defaultHTTPIdleTimeout, true)
	if err != nil {
		return HTTPConfig{}, err
	}

	return HTTPConfig{
		Port:              envOrDefault("PORT", defaultPort),
		ReadHeaderTimeout: readHeaderTimeout,
		ReadTimeout:       readTimeout,
		WriteTimeout:      writeTimeout,
		IdleTimeout:       idleTimeout,
	}, nil
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

func envBoolOrDefault(key string, def bool) (bool, error) {
	v := os.Getenv(key)
	if v == "" {
		return def, nil
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return false, fmt.Errorf("invalid %s %q: %w", key, v, err)
	}
	return b, nil
}

func envDurationOrDefault(key string, def time.Duration, allowZero bool) (time.Duration, error) {
	v := os.Getenv(key)
	if v == "" {
		return def, nil
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return 0, fmt.Errorf("invalid %s %q: %w", key, v, err)
	}
	if d < 0 || (d == 0 && !allowZero) {
		return 0, fmt.Errorf("invalid %s %q: must be positive", key, v)
	}
	return d, nil
}
