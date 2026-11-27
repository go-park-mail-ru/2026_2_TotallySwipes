package main

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	profilepb "dating-app/gen/go/profile/v1"
	chatpb "dating-app/services/chat/api/gen/go/chat/v1"
	matchpb "dating-app/services/matching/api/gen/go/match/v1"
	"dating-app/services/matching/internal/client"
	"dating-app/services/matching/internal/config"
	delivery "dating-app/services/matching/internal/delivery/grpc"
	"dating-app/services/matching/internal/logging"
	"dating-app/services/matching/internal/provider/timepad"
	"dating-app/services/matching/internal/repository/postgres"
	redisrepo "dating-app/services/matching/internal/repository/redis"
	"dating-app/services/matching/internal/service"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/redis/go-redis/v9"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
)

func main() {

	if err := start(); err != nil {
		os.Exit(1)
	}
}

func start() error {

	cfg, err := config.Load()

	if err != nil {
		slog.Error("load matching config", "error", err)
		return err
	}

	logger, logFile, err := logging.New(cfg.LogDir, cfg.LogLevel)

	if err != nil {
		slog.Error("init matching logger", "error", err)
		return fmt.Errorf("init logger: %w", err)
	}

	defer logFile.Close()
	slog.SetDefault(logger)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := run(ctx, cfg); err != nil {
		logger.Error("matching stopped", "error", err)
		return err
	}

	return nil
}

func run(ctx context.Context, cfg *config.Config) error {

	db, err := sql.Open("pgx", cfg.Database.URL)
	if err != nil {
		return fmt.Errorf("open postgres: %w", err)
	}
	defer db.Close()

	connectCtx, cancel := context.WithTimeout(ctx, cfg.ConnectTimeout)
	err = db.PingContext(connectCtx)
	cancel()

	if err != nil {
		return fmt.Errorf("connect postgres: %w", err)
	}

	redisClient := redis.NewClient(&redis.Options{Addr: cfg.Redis.Addr})
	defer redisClient.Close()
	connectCtx, cancel = context.WithTimeout(ctx, cfg.ConnectTimeout)
	err = redisClient.Ping(connectCtx).Err()
	cancel()

	if err != nil {
		return fmt.Errorf("connect redis: %w", err)
	}

	profileConn, err := grpc.NewClient(cfg.ProfileGRPCAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))

	if err != nil {
		return fmt.Errorf("create profile client: %w", err)
	}
	defer profileConn.Close()

	chatConn, err := grpc.NewClient(cfg.ChatGRPCAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))

	if err != nil {
		return fmt.Errorf("create chat client: %w", err)
	}
	defer chatConn.Close()

	profiles := client.NewProfileAdapter(profilepb.NewProfileServiceClient(profileConn))
	chat := client.NewChatAdapter(chatpb.NewChatServiceClient(chatConn))
	timepadProvider := timepad.NewTimepadProvider(
		&http.Client{Timeout: cfg.TimepadTimeout},
		cfg.TimepadBaseURL,
		cfg.TimepadToken,
	)

	matchRepo := postgres.NewMatchRepository(db)
	swipeRepo := postgres.NewSwipeRepository(db)
	feedRepo := postgres.NewFeedRepository(db)
	eventRepo := redisrepo.NewEventRepository(redisClient, cfg.Redis.EventRepositoryTTL)

	compatibility := service.NewCompatibilityService()
	matches := service.NewMatchService(matchRepo, profiles, compatibility, eventRepo)
	feed := service.NewFeedService(profiles, feedRepo, compatibility)
	swipes := service.NewSwipeService(swipeRepo, profiles, chat)
	events := service.NewEventService(timepadProvider, matchRepo, eventRepo)
	matchHandler := delivery.NewMatchHandler(matches, feed, swipes, events)

	server := grpc.NewServer()
	matchpb.RegisterMatchServiceServer(server, matchHandler)
	healthServer := health.NewServer()
	healthpb.RegisterHealthServer(server, healthServer)
	healthServer.SetServingStatus("", healthpb.HealthCheckResponse_SERVING)
	healthServer.SetServingStatus(matchpb.MatchService_ServiceDesc.ServiceName, healthpb.HealthCheckResponse_SERVING)
	defer healthServer.Shutdown()
	listener, err := net.Listen("tcp", ":"+cfg.GRPCPort)

	if err != nil {
		return fmt.Errorf("listen grpc: %w", err)
	}

	defer listener.Close()

	slog.Info("matching grpc started", "port", cfg.GRPCPort)

	return delivery.ServeGRPC(ctx, server, listener, cfg.ShutdownTimeout, healthServer.Shutdown)
}
