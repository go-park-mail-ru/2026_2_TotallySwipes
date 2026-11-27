package main

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"time"

	"google.golang.org/grpc"
)

const maxGRPCMessageSize = 12 << 20

func serveGRPC(ctx context.Context, server *grpc.Server, listener net.Listener, shutdownTimeout time.Duration, markNotServing func()) error {
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
	slog.Info("stopping chat grpc server")

	done := make(chan struct{})

	go func() {
		server.GracefulStop()
		close(done)
	}()

	timer := time.NewTimer(shutdownTimeout)
	defer timer.Stop()

	select {
	case <-done:
		slog.Info("chat grpc server stopped gracefully")

	case <-timer.C:
		slog.Warn("chat shutdown timeout exceeded; stopping active RPCs")
		server.Stop()
		<-done
	}

	return nil
}
