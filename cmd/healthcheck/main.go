package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
)

func main() {

	grpcAddress := flag.String("grpc", "", "gRPC health address")
	httpAddress := flag.String("http", "", "HTTP health URL")
	revision := flag.String("revision", "", "expected HTTP release revision")
	flag.Parse()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	if err := check(ctx, *grpcAddress, *httpAddress, *revision); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func check(ctx context.Context, grpcAddress, httpAddress, revision string) error {

	if (grpcAddress == "") == (httpAddress == "") {
		return fmt.Errorf("specify exactly one of -grpc or -http")
	}

	if grpcAddress != "" {

		conn, err := grpc.NewClient(grpcAddress, grpc.WithTransportCredentials(insecure.NewCredentials()))

		if err != nil {
			return err
		}

		defer conn.Close()
		response, err := healthpb.NewHealthClient(conn).Check(ctx, &healthpb.HealthCheckRequest{})

		if err != nil {
			return err
		}

		if response.GetStatus() != healthpb.HealthCheckResponse_SERVING {
			return fmt.Errorf("gRPC service is not serving")
		}

		return nil
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, httpAddress, nil)

	if err != nil {
		return err
	}

	response, err := (&http.Client{CheckRedirect: func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}}).Do(request)

	if err != nil {
		return err
	}

	defer response.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 4096))

	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP health status: %d", response.StatusCode)
	}

	if revision != "" && response.Header.Get("X-Release-Revision") != revision {
		return fmt.Errorf("HTTP health returned a different release")
	}

	return nil
}
