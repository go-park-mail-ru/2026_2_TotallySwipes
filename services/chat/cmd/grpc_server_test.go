package main

import (
	"context"
	"net"
	"sync/atomic"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/test/bufconn"
)

func TestGracefulShutdownIdleServer(t *testing.T) {

	listener := bufconn.Listen(1 << 20)
	defer listener.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var marked atomic.Bool
	result := make(chan error, 1)

	go func() {

		result <- serveGRPC(ctx, grpc.NewServer(), listener, time.Second, func() { marked.Store(true) })
	}()

	cancel()

	select {

	case err := <-result:

		if err != nil || !marked.Load() {
			t.Fatalf("error = %v, marked not serving = %v", err, marked.Load())
		}

	case <-time.After(3 * time.Second):
		t.Fatal("idle server did not stop")
	}
}

func TestShutdownStopsLongLivedStream(t *testing.T) {

	listener := bufconn.Listen(1 << 20)
	defer listener.Close()
	server := grpc.NewServer()
	healthServer := health.NewServer()
	healthpb.RegisterHealthServer(server, healthServer)
	healthServer.SetServingStatus("", healthpb.HealthCheckResponse_SERVING)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	const shutdownTimeout = 20 * time.Millisecond
	result := make(chan error, 1)

	go func() {

		result <- serveGRPC(ctx, server, listener, shutdownTimeout, healthServer.Shutdown)
	}()

	conn, err := grpc.NewClient("passthrough:///chat-test",
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) { return listener.DialContext(ctx) }),
	)

	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	requestCtx, requestCancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer requestCancel()
	stream, err := healthpb.NewHealthClient(conn).Watch(requestCtx, &healthpb.HealthCheckRequest{})

	if err != nil {
		t.Fatal(err)
	}

	if _, err := stream.Recv(); err != nil {
		t.Fatal(err)
	}

	started := time.Now()
	cancel()

	select {

	case err := <-result:

		if err != nil {
			t.Fatal(err)
		}
		if time.Since(started) < shutdownTimeout {
			t.Fatal("active stream did not hold graceful shutdown")
		}

	case <-time.After(3 * time.Second):
		server.Stop()
		t.Fatal("active stream prevented forced shutdown")
	}

	// Перед закрытием транспорта могло прийти последнее обновление health.
	for {

		if _, err := stream.Recv(); err != nil {
			break
		}
	}
}
