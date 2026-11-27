// Сервис авторизации: регистрация, вход, обновление и отзыв сессий по gRPC.
package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	authpb "dating-app/gen/go/auth/v1"
	profilepb "dating-app/gen/go/profile/v1"
	"dating-app/internal/auth"
	"dating-app/internal/grpcclient"
	"dating-app/internal/grpcserver"
	"dating-app/internal/logging"
	"dating-app/internal/repository/postgres"
	redisrepo "dating-app/internal/repository/redis"
	"dating-app/internal/service"
	pgstorage "dating-app/internal/storage/postgres"
	redisstorage "dating-app/internal/storage/redis"
	"dating-app/services/auth/internal/client"
	"dating-app/services/auth/internal/config"
	"dating-app/services/auth/internal/grpchandler"
)

const connectTimeout = 5 * time.Second

func main() {
	if err := start(); err != nil {
		os.Exit(1)
	}
}

func start() error {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("load config", "error", err)
		return err
	}

	logger, logFile, err := logging.NewWithLevel(cfg.LogDir, cfg.LogLevel)
	if err != nil {
		slog.Error("init logger", "error", err)
		return err
	}
	defer logFile.Close()
	slog.SetDefault(logger)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := run(ctx, cfg, logger); err != nil {
		logger.Error("auth service stopped", "error", err)
		return err
	}
	return nil
}

func run(ctx context.Context, cfg *config.Config, logger *slog.Logger) error {
	connectCtx, cancel := context.WithTimeout(ctx, connectTimeout)
	defer cancel()

	db, err := pgstorage.OpenPostgres(connectCtx, cfg.DatabaseURL, pgstorage.NewTracer(logger))
	if err != nil {
		return err
	}
	defer db.Close()

	rdb, err := redisstorage.OpenRedis(connectCtx, cfg.RedisAddr)
	if err != nil {
		return err
	}
	defer rdb.Close()

	profileConn, err := grpcclient.Dial(cfg.ProfileGRPCAddr, cfg.GRPCRequestTimeout)
	if err != nil {
		return err
	}
	defer profileConn.Close()

	authSvc := service.NewAuthService(
		postgres.NewUserRepository(db),
		client.NewProfileMissing(profilepb.NewProfileServiceClient(profileConn)),
		redisrepo.NewSessionRepository(rdb),
		auth.BcryptHasher{},
		auth.NewJWTIssuer(cfg.JWTSecret, cfg.JWTAccessTTL),
		cfg.JWTRefreshTTL,
	)

	srv := grpcserver.New()
	authpb.RegisterAuthServiceServer(srv, grpchandler.NewServer(authSvc))

	return grpcserver.Run(ctx, srv, cfg.GRPCPort, cfg.ShutdownTimeout)
}
