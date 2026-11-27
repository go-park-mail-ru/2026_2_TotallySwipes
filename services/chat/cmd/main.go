package main

import (
	"context"
	"database/sql"
	"net"
	"os/exec"

	profilepb "dating-app/gen/go/profile/v1"

	chatpb "dating-app/services/chat/api/gen/go/chat/v1"
	"dating-app/services/chat/internal/broker"
	"dating-app/services/chat/internal/client"
	"dating-app/services/chat/internal/config"
	delivery "dating-app/services/chat/internal/delivery/grpc"
	"dating-app/services/chat/internal/logging"
	"dating-app/services/chat/internal/provider"
	"dating-app/services/chat/internal/repository"
	"dating-app/services/chat/internal/service"
	"dating-app/services/chat/internal/storage/s3"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	_ "github.com/jackc/pgx/v5/stdlib"
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
		slog.Error("load chat config", "error", err)
		return err
	}

	logger, logFile, err := logging.New(cfg.LogDir, cfg.LogLevel)

	if err != nil {
		slog.Error("init chat logger", "error", err)
		return fmt.Errorf("init logger: %w", err)
	}
	defer logFile.Close()
	slog.SetDefault(logger)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := run(ctx, cfg); err != nil {
		logger.Error("chat stopped", "error", err)
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

	connectCtx, cancel = context.WithTimeout(ctx, cfg.ConnectTimeout)
	storage, err := s3.OpenMinIOStorage(connectCtx, cfg.MinIO)
	cancel()

	if err != nil {
		return fmt.Errorf("open media storage: %w", err)
	}

	profileConn, err := grpc.NewClient(cfg.ProfileGRPCAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return fmt.Errorf("create profile client: %w", err)
	}
	defer profileConn.Close()

	profiles := client.NewProfileAdapter(profilepb.NewProfileServiceClient(profileConn))
	eventBroker := broker.NewMemoryBroker(cfg.BrokerQueueSize)
	mediaLinks := provider.NewMediaLinkBuilder()

	ffprobePath, err := exec.LookPath(cfg.FFprobePath)

	if err != nil {
		return fmt.Errorf("find ffprobe: %w", err)
	}

	inspector := provider.NewFFprobeInspector(ffprobePath)

	repo := repository.NewChatRepository(db)
	chatService := service.NewChatService(repo, profiles, eventBroker, storage, mediaLinks, inspector)
	chatHandler := delivery.NewChatHandler(chatService)

	server := grpc.NewServer(grpc.MaxRecvMsgSize(maxGRPCMessageSize))
	chatpb.RegisterChatServiceServer(server, chatHandler)

	healthServer := health.NewServer()
	healthpb.RegisterHealthServer(server, healthServer)
	healthServer.SetServingStatus("", healthpb.HealthCheckResponse_SERVING)
	healthServer.SetServingStatus(chatpb.ChatService_ServiceDesc.ServiceName, healthpb.HealthCheckResponse_SERVING)
	defer healthServer.Shutdown()

	listener, err := net.Listen("tcp", ":"+cfg.GRPCPort)

	if err != nil {
		return fmt.Errorf("listen chat grpc: %w", err)
	}
	defer listener.Close()

	slog.Info("chat grpc started", "port", cfg.GRPCPort)

	return serveGRPC(ctx, server, listener, cfg.ShutdownTimeout, healthServer.Shutdown)
}
