package config

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	LogDir          string
	HTTP            HTTPConfig
	GRPC            GRPCConfig
	Auth            AuthConfig
	TLS             TLSConfig
	WebSocket       WebSocketConfig
	Logging         LoggingConfig
	ShutdownTimeout time.Duration
}

type HTTPConfig struct {
	Port              string
	CORSOrigins       []string
	ReadHeaderTimeout time.Duration
	ReadTimeout       time.Duration
	WriteTimeout      time.Duration
	IdleTimeout       time.Duration
}

type GRPCConfig struct {
	AuthAddr       string
	ProfileAddr    string
	MatchingAddr   string
	ChatAddr       string
	RequestTimeout time.Duration
}

type AuthConfig struct {
	JWTSecret     string
	JWTAccessTTL  time.Duration
	JWTRefreshTTL time.Duration
	CookieSecure  bool
}

type TLSConfig struct {
	CertFile string
	KeyFile  string
}

type WebSocketConfig struct {
	AllowedOrigins    []string
	HandshakeTimeout  time.Duration
	WriteTimeout      time.Duration
	PongTimeout       time.Duration
	PingInterval      time.Duration
	MaxMessageBytes   int64
	SendQueueSize     int
	CommandsQueueSize int
}

type LoggingConfig struct {
	Level slog.Level
}

func Load() (*Config, error) {

	var r envReader

	cfg := &Config{
		LogDir: r.str("LOG_DIR", "./logs/gateway"),
		HTTP: HTTPConfig{
			Port:              r.str("PORT", "8080"),
			CORSOrigins:       r.list("CORS_ORIGINS", "http://localhost:5173,https://totally-swipes.ru"),
			ReadHeaderTimeout: r.duration("HTTP_READ_HEADER_TIMEOUT", 5*time.Second),
			ReadTimeout:       r.duration("HTTP_READ_TIMEOUT", 10*time.Second),
			WriteTimeout:      r.duration("HTTP_WRITE_TIMEOUT", 10*time.Second),
			IdleTimeout:       r.duration("HTTP_IDLE_TIMEOUT", 60*time.Second),
		},
		GRPC: GRPCConfig{
			AuthAddr:       r.str("AUTH_GRPC_ADDR", "auth:50051"),
			ProfileAddr:    r.str("PROFILE_GRPC_ADDR", "profile:50051"),
			MatchingAddr:   r.str("MATCHING_GRPC_ADDR", "matching:50051"),
			ChatAddr:       r.str("CHAT_GRPC_ADDR", "chat:50051"),
			RequestTimeout: r.positiveDuration("GRPC_REQUEST_TIMEOUT", 5*time.Second),
		},
		Auth: AuthConfig{
			JWTSecret:     r.required("JWT_SECRET"),
			JWTAccessTTL:  r.positiveDuration("JWT_ACCESS_TTL", 15*time.Minute),
			JWTRefreshTTL: r.positiveDuration("JWT_REFRESH_TTL", 30*24*time.Hour),
			CookieSecure:  r.boolean("COOKIE_SECURE", true),
		},
		TLS: TLSConfig{
			CertFile: r.str("TLS_CERT_FILE", ""),
			KeyFile:  r.str("TLS_KEY_FILE", ""),
		},
		WebSocket: WebSocketConfig{
			AllowedOrigins:    r.list("WS_ALLOWED_ORIGINS", "http://localhost:5173,https://totally-swipes.ru"),
			HandshakeTimeout:  r.positiveDuration("WS_HANDSHAKE_TIMEOUT", 5*time.Second),
			WriteTimeout:      r.positiveDuration("WS_WRITE_TIMEOUT", 10*time.Second),
			PongTimeout:       r.positiveDuration("WS_PONG_TIMEOUT", 60*time.Second),
			PingInterval:      r.positiveDuration("WS_PING_INTERVAL", 25*time.Second),
			MaxMessageBytes:   r.positiveInt64("WS_MAX_MESSAGE_BYTES", 65536),
			SendQueueSize:     r.positiveInt("WS_SEND_QUEUE_SIZE", 64),
			CommandsQueueSize: r.positiveInt("WS_COMMANDS_QUEUE_SIZE", 64),
		},
		Logging: LoggingConfig{
			Level: r.logLevel("LOG_LEVEL", slog.LevelInfo),
		},
		ShutdownTimeout: r.positiveDuration("SHUTDOWN_TIMEOUT", 10*time.Second),
	}

	port, err := strconv.Atoi(cfg.HTTP.Port)

	if (cfg.TLS.CertFile == "") != (cfg.TLS.KeyFile == "") {
		r.errs = append(r.errs, errors.New("TLS_CERT_FILE and TLS_KEY_FILE must be set together"))
	}

	if err != nil || port < 1 || port > 65535 {
		r.errs = append(r.errs, errors.New("PORT must be an integer between 1 and 65535"))
	}

	if cfg.WebSocket.PingInterval >= cfg.WebSocket.PongTimeout {
		r.errs = append(r.errs, errors.New("WS_PING_INTERVAL must be less than WS_PONG_TIMEOUT"))
	}

	if err := r.err(); err != nil {
		return nil, err
	}

	return cfg, nil
}

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

func (r *envReader) list(key, def string) []string {
	var items []string
	for _, item := range strings.Split(r.str(key, def), ",") {
		if item = strings.TrimSpace(item); item != "" {
			items = append(items, item)
		}
	}

	return items
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

	if len(r.errs) == before && d == 0 {
		r.fail(key, os.Getenv(key), errors.New("must be positive"))
	}

	return d
}

func (r *envReader) positiveInt64(key string, def int64) int64 {

	value := os.Getenv(key)

	if value == "" {
		return def
	}

	result, err := strconv.ParseInt(value, 10, 64)

	if err != nil {
		r.fail(key, value, err)
		return 0
	}

	if result <= 0 {
		r.fail(key, value, errors.New("must be positive"))
	}

	return result
}

func (r *envReader) positiveInt(key string, def int) int {

	value := os.Getenv(key)

	if value == "" {
		return def
	}

	result, err := strconv.Atoi(value)

	if err != nil {
		r.fail(key, value, err)
		return 0
	}

	if result <= 0 {
		r.fail(key, value, errors.New("must be positive"))
	}

	return result
}

func (r *envReader) logLevel(key string, def slog.Level) slog.Level {

	value := os.Getenv(key)

	if value == "" {
		return def
	}

	var level slog.Level

	if err := level.UnmarshalText([]byte(value)); err != nil {
		r.fail(key, value, err)
	}

	return level
}
