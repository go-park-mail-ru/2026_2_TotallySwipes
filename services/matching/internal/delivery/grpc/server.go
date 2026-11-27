package grpc

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"time"

	"google.golang.org/grpc"
)

// ServeGRPC запускает сервер и завершает активные RPC при остановке приложения.
func ServeGRPC(ctx context.Context, server *grpc.Server, listener net.Listener, shutdownTimeout time.Duration, markNotServing func()) error {
	defer server.Stop()

	serveErr := make(chan error, 1)
	go func() {

		serveErr <- server.Serve(listener)
	}()

	select {
	case err := <-serveErr:
		if errors.Is(err, grpc.ErrServerStopped) {
			return nil
		}
		return err
	case <-ctx.Done():
	}

	markNotServing()
	slog.Info("stopping matching grpc server")

	done := make(chan struct{})

	go func() {
		server.GracefulStop()
		close(done)
	}()

	timer := time.NewTimer(shutdownTimeout)
	defer timer.Stop()

	select {
	case <-done:
		slog.Info("matching grpc server stopped gracefully")

	case <-timer.C:
		slog.Warn("matching shutdown timeout exceeded; stopping active RPCs")
		server.Stop()
		<-done
	}

	return nil
}
