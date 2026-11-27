package config

import (
	"errors"
	"log/slog"
	"strconv"
	"time"
)

type Config struct {
	GRPCPort        string
	ProfileGRPCAddr string
	MinIO           MinIOConfig
	BrokerQueueSize int
	FFprobePath     string
	ConnectTimeout  time.Duration
	ShutdownTimeout time.Duration
	LogDir          string
	LogLevel        slog.Level
	Database        DatabaseConfig
	Redis           RedisConfig
}

type DatabaseConfig struct {
	URL string
}

type RedisConfig struct {
	Addr string
}

// Load читает подключения по тем же правилам, что и конфиг монолита.
func Load() (*Config, error) {

	var r envReader

	cfg := &Config{
		GRPCPort:        r.str("GRPC_PORT", "50051"),
		ProfileGRPCAddr: r.str("PROFILE_GRPC_ADDR", "profile:50051"),
		MinIO: MinIOConfig{
			Endpoint:       r.str("MINIO_ENDPOINT", "minio:9000"),
			AccessKey:      r.str("MINIO_ACCESS_KEY", "minioadmin"),
			SecretKey:      r.str("MINIO_SECRET_KEY", "minioadmin"),
			Bucket:         r.str("MINIO_BUCKET", "chat-media"),
			UseTLS:         r.boolean("MINIO_USE_TLS", false),
			DownloadURLTTL: r.positiveDuration("MINIO_DOWNLOAD_URL_TTL", 15*time.Minute),
		},
		FFprobePath:     r.str("FFPROBE_PATH", "ffprobe"),
		ConnectTimeout:  r.positiveDuration("CONNECT_TIMEOUT", 5*time.Second),
		ShutdownTimeout: r.positiveDuration("SHUTDOWN_TIMEOUT", 10*time.Second),
		LogDir:          r.str("LOG_DIR", "./logs/chat"),
		Database: DatabaseConfig{
			URL: r.required("DATABASE_URL"),
		},
		Redis: RedisConfig{
			Addr: r.str("REDIS_ADDR", "localhost:6379"),
		},
	}

	port, err := strconv.Atoi(cfg.GRPCPort)

	if err != nil || port < 1 || port > 65535 {
		r.fail("GRPC_PORT", cfg.GRPCPort, errors.New("must be between 1 and 65535"))
	}

	queueSize := r.str("CHAT_EVENT_QUEUE_SIZE", "100")
	cfg.BrokerQueueSize, err = strconv.Atoi(queueSize)

	if err != nil || cfg.BrokerQueueSize <= 0 {
		r.fail("CHAT_EVENT_QUEUE_SIZE", queueSize, errors.New("must be a positive integer"))
	}

	if cfg.MinIO.DownloadURLTTL > 7*24*time.Hour {
		r.fail("MINIO_DOWNLOAD_URL_TTL", cfg.MinIO.DownloadURLTTL.String(), errors.New("must not exceed seven days"))
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
