// Package grpcserver собирает gRPC-сервер микросервиса: интерсепторы, health и graceful shutdown.
package grpcserver

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"runtime/debug"
	"time"

	"dating-app/internal/grpcerr"
	"dating-app/internal/requestctx"
	"dating-app/internal/validate"

	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/status"
)

// MaxMsgSize вмещает загрузку фото максимального размера вместе с остальными полями запроса.
const MaxMsgSize = validate.PhotoMaxSize + 1<<20

// New создаёт сервер с интерсепторами recovery → request ID → логирование → перевод ошибок,
// health-сервисом и reflection (для grpcurl во внутренней сети).
func New(opts ...grpc.ServerOption) *grpc.Server {
	base := []grpc.ServerOption{
		grpc.MaxRecvMsgSize(MaxMsgSize),
		grpc.MaxSendMsgSize(MaxMsgSize),
		grpc.ChainUnaryInterceptor(
			RecoveryInterceptor,
			RequestIDInterceptor,
			LoggingInterceptor,
			ErrorInterceptor,
		),
	}
	srv := grpc.NewServer(append(base, opts...)...)

	healthpb.RegisterHealthServer(srv, health.NewServer())
	reflection.Register(srv)
	return srv
}

// Run слушает порт и обслуживает запросы до отмены ctx, затем останавливается штатно,
// но не дольше shutdownTimeout.
func Run(ctx context.Context, srv *grpc.Server, port string, shutdownTimeout time.Duration) error {
	lis, err := net.Listen("tcp", ":"+port)
	if err != nil {
		return fmt.Errorf("listen :%s: %w", port, err)
	}

	serveErr := make(chan error, 1)
	go func() { serveErr <- srv.Serve(lis) }()
	slog.Info("grpc server started", "port", port)

	select {
	case err := <-serveErr:
		return err
	case <-ctx.Done():
	}

	slog.Info("grpc server stopping")
	stopped := make(chan struct{})
	go func() {
		srv.GracefulStop()
		close(stopped)
	}()

	select {
	case <-stopped:
	case <-time.After(shutdownTimeout):
		srv.Stop()
	}

	if err := <-serveErr; err != nil && !errors.Is(err, grpc.ErrServerStopped) {
		return err
	}
	return nil
}

// RecoveryInterceptor превращает панику обработчика в codes.Internal.
func RecoveryInterceptor(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp any, err error) {
	defer func() {
		if r := recover(); r != nil {
			slog.ErrorContext(ctx, "grpc panic", "method", info.FullMethod, "panic", r, "stack", string(debug.Stack()))
			err = status.Error(codes.Internal, "internal error")
		}
	}()
	return handler(ctx, req)
}

// RequestIDInterceptor берёт request ID из входящих метаданных или генерирует новый.
func RequestIDInterceptor(ctx context.Context, req any, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
	id := ""
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		if values := md.Get(requestctx.MetadataKey); len(values) > 0 {
			id = values[0]
		}
	}
	if id == "" {
		id = uuid.NewString()
	}
	return handler(context.WithValue(ctx, requestctx.RequestIDKey{}, id), req)
}

// LoggingInterceptor пишет метод, код ответа и длительность вызова.
func LoggingInterceptor(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
	start := time.Now()
	resp, err := handler(ctx, req)

	code := status.Code(err)
	level := slog.LevelInfo
	switch code {
	case codes.Internal, codes.Unknown, codes.DataLoss, codes.Unimplemented:
		level = slog.LevelError
	case codes.Unavailable, codes.DeadlineExceeded:
		level = slog.LevelWarn
	}
	slog.Log(ctx, level, "grpc request", "method", info.FullMethod, "code", code.String(), "duration", time.Since(start))
	return resp, err
}

// ErrorInterceptor переводит ошибки сервиса в статусы; непредусмотренные ошибки логируются целиком,
// клиенту уходит только codes.Internal.
func ErrorInterceptor(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
	resp, err := handler(ctx, req)
	if err == nil {
		return resp, nil
	}

	converted := grpcerr.ToStatus(err)
	if !grpcerr.IsKnown(err) && status.Code(converted) == codes.Internal {
		slog.ErrorContext(ctx, "grpc handler failed", "method", info.FullMethod, "error", err)
	}
	return nil, converted
}
