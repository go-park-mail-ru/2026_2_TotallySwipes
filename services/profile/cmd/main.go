// Сервис анкет: анкета, фото, психологический тест, фильтр поиска и геолокация по gRPC.
package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	profilepb "dating-app/gen/go/profile/v1"
	"dating-app/internal/geocoder"
	"dating-app/internal/grpcserver"
	"dating-app/internal/logging"
	"dating-app/internal/psychotest"
	"dating-app/internal/repository/postgres"
	redisrepo "dating-app/internal/repository/redis"
	"dating-app/internal/service"
	pgstorage "dating-app/internal/storage/postgres"
	redisstorage "dating-app/internal/storage/redis"
	"dating-app/internal/storage/s3"
	"dating-app/services/profile/internal/config"
	"dating-app/services/profile/internal/grpchandler"
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
		logger.Error("profile service stopped", "error", err)
		return err
	}
	return nil
}

func run(ctx context.Context, cfg *config.Config, logger *slog.Logger) error {
	test, err := psychotest.Load()
	if err != nil {
		return err
	}

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

	photoStorage, err := s3.OpenMinIOStorage(connectCtx, cfg.MinIO)
	if err != nil {
		return err
	}

	profileRepo := postgres.NewProfileRepository(db)
	compatibilitySvc := service.NewCompatibilityService()
	// Фото отдаёт gateway (/media/...), порт MinIO снаружи закрыт.
	photoURLs := s3.NewProxyURLProvider(cfg.PublicURL)
	reverseGeocoder := geocoder.NewDaData(&http.Client{Timeout: cfg.Geocoder.HTTPClientTimeout}, cfg.Geocoder.BaseURL, cfg.Geocoder.APIKey)

	server := grpchandler.NewServer(
		service.NewProfileService(profileRepo, compatibilitySvc, photoURLs, photoStorage),
		photoStorage,
		service.NewFilterService(postgres.NewFilterRepository(db)),
		service.NewPsychoTestService(postgres.NewPsychoTestRepository(db), profileRepo, compatibilitySvc, test),
		service.NewLocationService(
			postgres.NewLocationRepository(db),
			reverseGeocoder,
			redisrepo.NewLocationPreviewRepository(rdb, cfg.LocationPreviewTTL),
		),
	)

	srv := grpcserver.New()
	profilepb.RegisterProfileServiceServer(srv, server)

	return grpcserver.Run(ctx, srv, cfg.GRPCPort, cfg.ShutdownTimeout)
}
